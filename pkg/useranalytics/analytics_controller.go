package useranalytics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	// this import statement registers all Origin types w/ the client
	_ "github.com/openshift/origin/pkg/api/install"

	osclient "github.com/openshift/origin/pkg/client"
	projectapi "github.com/openshift/origin/pkg/project/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

// AnalyticsController is a controller that Watches & Forwards analytics data to various endpoints.
// Only new analytics are forwarded. There is no replay.
type AnalyticsController struct {
	watchResourceVersions   map[string]string
	destinations            map[string]Destination
	queue                   *cache.FIFO
	maximumQueueLength      int
	// required to lookup Projects and Users, as needed
	client                  osclient.Interface
	kclient                 kclient.Interface
	namespaceStore          cache.Store
	userStore               cache.Store
	defaultUserIds          map[string]string
	startTime               int64
	metricsServerPort       int
	metricsPollingFrequency int
	eventsHandled           int
	metrics                 *Stack
	mutex                   *sync.Mutex
	stopChannel             <-chan struct {}
	controllerID            string
	clusterName             string
}

type metricsSnapshot struct {
	Timestamp          int64
	CurrentQueueLength int
	EventsHandled      int
}

type AnalyticsControllerConfig struct {
	Destinations            map[string]Destination
	DefaultUserIds          map[string]string
	KubeClient              kclient.Interface
	OSClient                osclient.Interface
	MaximumQueueLength      int
	MetricsServerPort       int
	MetricsPollingFrequency int
	ClusterName             string
}

// NewAnalyticsController creates a new ThirdPartyAnalyticsController
func NewAnalyticsController(config *AnalyticsControllerConfig) (*AnalyticsController, error) {
	glog.V(1).Infof("Creating user-analytics controller")
	ctrl := &AnalyticsController{
		watchResourceVersions: make(map[string]string),
		queue:                   cache.NewFIFO(analyticKeyFunc),
		destinations:            config.Destinations,
		client:                  config.OSClient,
		kclient:                 config.KubeClient,
		defaultUserIds:          make(map[string]string),
		maximumQueueLength:      config.MaximumQueueLength,
		metricsServerPort:       config.MetricsServerPort,
		metricsPollingFrequency: config.MetricsPollingFrequency,
		metrics:                 NewStack(10),
		mutex:                   &sync.Mutex{},
		namespaceStore:          cache.NewStore(cache.MetaNamespaceKeyFunc),
		userStore:               cache.NewStore(cache.MetaNamespaceKeyFunc),
		controllerID:            string(util.NewUUID()),
		clusterName:             config.ClusterName,
	}
	for name, value := range config.DefaultUserIds {
		ctrl.defaultUserIds[name] = value
		glog.V(1).Infof("Setting default UserID %s for destination %s", value, name)
	}
	return ctrl, nil
}

func analyticKeyFunc(obj interface{}) (string, error) {
	e, ok := obj.(*analyticsEvent)
	if !ok {
		return "", fmt.Errorf("Expected type *analyticEvent but got %#v", obj)
	}
	return e.Hash(), nil
}

// Run starts all the watches within this controller and starts workers to process events
func (c *AnalyticsController) Run(stopCh <-chan struct {}, workers int) {
	glog.V(1).Infof("Starting ThirdPartyAnalyticsController\n")

	c.stopChannel = stopCh
	c.startTime = time.Now().UnixNano()

	// the workers that forward analytic events to destinations
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, c.stopChannel)
	}

	// projects are handled separately from other object watches because we actually want to maintain
	// a local cache of Projects for easy lookup.
	// this only needs to be called once because each Reflector created re-establishes its own watch on failure
	c.runProjectWatch()

	// watches are run within their own goroutines. Each goroutine has the watch func and can re-start
	// if its internal forever loop returns/exits for any reason.
	// this function only needs to be called once.
	c.runWatches()

	// this go routine collects metrics in the background
	go wait.Until(c.gatherMetrics, time.Duration(c.metricsPollingFrequency) * time.Second, c.stopChannel)
	// this go routine serves metrics for consumption
	go wait.Until(c.serveMetrics, 1 * time.Second, c.stopChannel)
}

// runWatches will attempt to run all watches in separate goroutines w/ the same stop channel.  Each has its own
// ability to restart the watch if the inner func doing the work fails for any reason.
func (c *AnalyticsController) runWatches() {
	watchListItems := WatchFuncList(c.kclient, c.client)
	for name, _ := range watchListItems {

		// assign local variable (not in range operator above) so that each
		// goroutine gets the correct watch function required
		wfnc := watchListItems[name]
		n := name
		kind := wfnc.objType.GetObjectKind().GroupVersionKind().Kind
		backoff := 1 * time.Second

		go wait.Until(func() {
			// any return from this func only exits that invocation of the func.
			// wait.Until will call it again after its sync period.
			glog.V(3).Infof("Starting watch for %s with resourceVersion %s", n, c.watchResourceVersions[kind])
			w, err := wfnc.watchFunc(api.ListOptions{ResourceVersion: c.watchResourceVersions[kind]})
			if err != nil {
				glog.Errorf("error creating watch %s: %v", n, err)
			}

			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > 60 * time.Second {
				backoff = 60 * time.Second
			}

			if w == nil {
				glog.Errorf("watch not created for %s, returning", n)
				return
			}

			for {
				select {
				case event, ok := <-w.ResultChan():
					if !ok {
						glog.Errorf("Error received from %s watch channel", n)
						return
					}

					if event.Type == watch.Error {
						glog.Errorf("Watch channel returned error: %s", spew.Sdump(event))
						return
					}

				// success means the watch is working.
				// reset the backoff back to 1s for this watch
					backoff = 1 * time.Second

					if event.Type == watch.Added || event.Type == watch.Deleted {
						m, err := meta.Accessor(event.Object)
						if err != nil {
							glog.Errorf("Unable to create object meta for %v", event.Object)
							return
						}
						// each watch is a separate go routine
						c.mutex.Lock()
						c.watchResourceVersions[kind] = m.GetResourceVersion()
						c.mutex.Unlock()

						analytic, err := newEvent(event.Object, event.Type)
						if err != nil {
							glog.Errorf("Unexpected error creation analytic from watch event %#v", event.Object)
						} else {
							// additional info will be set to the analytic and
							// an instance queued for all destinations
							err := c.AddEvent(analytic)
							if err != nil {
								glog.Errorf("Error adding event: %v - %v", err, analytic)
							}
						}
					}
				}
			}
		}, 1 * time.Millisecond, c.stopChannel)
	}
}

func (c *AnalyticsController) runProjectWatch() {
	namespaceLW := &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			return c.kclient.Namespaces().List(options)
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			return c.kclient.Namespaces().Watch(options)
		},
	}
	cache.NewReflector(namespaceLW, &api.Namespace{}, c.namespaceStore, 10 * time.Minute).Run()

	userLW := &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			return c.client.Users().List(options)
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			return c.client.Users().Watch(options)
		},
	}
	cache.NewReflector(userLW, &userapi.User{}, c.userStore, 10 * time.Minute).Run()
}

func (c *AnalyticsController) gatherMetrics() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	metric := &metricsSnapshot{
		Timestamp:          time.Now().Unix(),
		CurrentQueueLength: len(c.queue.ListKeys()),
		EventsHandled:      c.eventsHandled,
	}
	glog.Infof("Pushing metric: %#v", metric)
	c.metrics.Push(metric)
}

func (c *AnalyticsController) serveMetrics() {
	http.HandleFunc("/metrics", c.metricsHandler)
	strPort := fmt.Sprintf(":%d", c.metricsServerPort)
	glog.Infof("Starting metrics server on port %s", strPort)
	if err := http.ListenAndServe(strPort, nil); err != nil {
		glog.Fatal("Could not start server")
	}
}

func (c *AnalyticsController) metricsHandler(w http.ResponseWriter, r *http.Request) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	b, err := json.Marshal(c.metrics.AsList())
	if err != nil {
		fmt.Printf("Error: %s", err)
		return
	}
	fmt.Fprint(w, string(b))
}

func (c *AnalyticsController) processAnalyticFromQueue(obj interface{}) error {
	e, ok := obj.(*analyticsEvent)
	if !ok {
		return fmt.Errorf("Expected analyticEvent object but got %v", obj)
	}

	if len(e.destination) == 0 {
		return fmt.Errorf("No destination specified. Ignoring analytic: %v", e)
	}

	dest, ok := c.destinations[e.destination]
	if !ok {
		return fmt.Errorf("Destination %s not found", e.destination)
	}
	err := dest.Send(e)
	if err != nil {
		return fmt.Errorf("send error: %v ", err)
	}
	c.eventsHandled++
	return nil
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
func (c *AnalyticsController) worker() {
	for {
		func() {
			_, err := c.queue.Pop(c.processAnalyticFromQueue)
			if err != nil {
				glog.Errorf("error processing analytic: %v", err)
			}
		}()
	}
}

// AddEvent is the primary way of adding analytic events to the processing queue.
// Re-adding the same event will cause duplicates.
// Sending events is not retried.
// All destinations are queued as separate work items.
// The namespace owner is automatically assigned as the event owner.
// Events w/ timestamps earlier than the start of this controller are not processed.
func (c *AnalyticsController) AddEvent(ev *analyticsEvent) error {
	if len(c.queue.ListKeys()) > c.maximumQueueLength {
		return fmt.Errorf("analyticEvent reject, exceeds maximum queue length: %d - %#v", c.maximumQueueLength, ev)
	}
	if ev.timestamp.UnixNano() < c.startTime {
		glog.V(5).Infof("Warning: analyticEvent is too old: %v", ev)
		return nil
	}

	for destName, _ := range c.destinations {
		ev.destination = destName // needed here to find default ID by destination
		userId, err := c.getUserId(ev)
		if err != nil {
			switch err.reason {
			case missingProjectError:
			case requesterAnnotationNotFoundError:
				glog.V(3).Infoln(err.message)
			case userNotFoundError:
			case noIDFoundError:
				glog.V(5).Infoln(err.message)
			}
			return nil
		}

		e := &analyticsEvent{
			userID:          userId,
			event:           ev.event,
			objectKind:      ev.objectKind,
			objectName:      ev.objectName,
			objectNamespace: ev.objectNamespace,
			objectUID:       ev.objectUID,
			properties:      make(map[string]string),
			timestamp:       ev.timestamp,
			destination:     destName,
			clusterName:     c.clusterName,
			controllerID:    c.controllerID,
		}
		for key, value := range ev.properties {
			e.properties[key] = value
		}

		c.queue.Add(e)
	}

	return nil
}

type userIDError struct {
	message string
	reason  string
}

func (u *userIDError) Error() string {
	return u.message
}

const (
	missingProjectError = "ProjectNotFoundError"
	requesterAnnotationNotFoundError = "RequesterAnnotationNotFoundError"
	userNotFoundError = "UserNotFoundError"
	noIDFoundError = "NoIDFoundError"
)

// getUserId returns a unique identifier to associate analytics with. It wants to return, in order:
// 1. user.Annotations[OnlineManagedID], which is the Intercom ID in the Online environment
// 2. a default ID associated with the specific destination on the analytic event. Used for testing external endpoints.
// 3. user.UID for non-Online environment that want analytics (e.g, Dedicated).
// If an ID cannot be found for any reason, an empty string and error is returned
func (c *AnalyticsController) getUserId(ev *analyticsEvent) (string, *userIDError) {
	namespaceName := ev.objectNamespace
	if namespaceName == "" {
		// namespace has no namespace, but its name *is* the namespace
		namespaceName = ev.objectName
	}

	obj, exists, err := c.namespaceStore.GetByKey(namespaceName)
	if !exists || err != nil {
		return "", &userIDError{
			fmt.Sprintf("Project %s does not exist in local cache or error: %v", namespaceName, err),
			missingProjectError,
		}
	}

	namespace := obj.(*api.Namespace)

	username, exists := namespace.Annotations[projectapi.ProjectRequester]
	if !exists {
		return "", &userIDError{
			fmt.Sprintf("ProjectRequest annotation does not exist on project %s", namespaceName),
			requesterAnnotationNotFoundError,
		}
	}

	userObj, exists, err := c.userStore.GetByKey(username)
	if err != nil || !exists {
		return "", &userIDError{
			fmt.Sprintf("Failed to find user %s: %v", username, err),
			userNotFoundError,
		}
	}

	if userObj != nil {
		user := userObj.(*userapi.User)
		externalId, exists := user.Annotations[OnlineManagedID]
		if exists {
			return externalId, nil
		}

		// a defaultId is used for local testing against an external provider.
		if id, ok := c.defaultUserIds[ev.destination]; ok {
			return id, nil
		}

		// any non-Online environment (e.g, Dedicated) will never have an externalId
		// the fallback is UserID
		return string(user.UID), nil
	}

	return "", &userIDError{
		fmt.Sprintf("No suitable ID could be found for analytic. A user must be logged in for analytics to be counted.  %#v", ev),
		noIDFoundError,
	}
}
