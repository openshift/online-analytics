package useranalytics

import (
	"fmt"
	"math/big"
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
	"k8s.io/kubernetes/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

const KeyStrategyAnnotation = "annotation"
const KeyStrategyName = "name"
const KeyStrategyUID = "uid"
const OnlineManagedID = "openshift.io/online-managed-id"

// AnalyticsController is a controller that Watches & Forwards analytics data to various endpoints.
// Only new analytics are forwarded. There is no replay.
type AnalyticsController struct {
	watchResourceVersions map[string]string
	destinations          map[string]Destination
	queue                 *cache.FIFO
	maximumQueueLength    int
	// required to lookup Projects and Users, as needed
	client                  osclient.Interface
	kclient                 kclient.Interface
	namespaceStore          cache.Store
	userStore               cache.Store
	startTime               int64
	metricsPollingFrequency int
	eventsHandled           int
	metrics                 *Stack
	mutex                   *sync.Mutex
	stopChannel             <-chan struct{}
	controllerID            string
	clusterName             string
	userKeyStrategy         string
	userKeyAnnotation       string
}

type AnalyticsControllerConfig struct {
	Destinations            map[string]Destination
	KubeClient              kclient.Interface
	OSClient                osclient.Interface
	MaximumQueueLength      int
	MetricsPollingFrequency int
	ClusterName             string
	UserKeyStrategy         string
	UserKeyAnnotation       string
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
		maximumQueueLength:      config.MaximumQueueLength,
		metricsPollingFrequency: config.MetricsPollingFrequency,
		metrics:                 NewStack(10),
		mutex:                   &sync.Mutex{},
		namespaceStore:          cache.NewStore(cache.MetaNamespaceKeyFunc),
		userStore:               cache.NewStore(cache.MetaNamespaceKeyFunc),
		controllerID:            string(uuid.NewUUID()),
		clusterName:             config.ClusterName,
		userKeyStrategy:         config.UserKeyStrategy,
		userKeyAnnotation:       config.UserKeyAnnotation,
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
func (c *AnalyticsController) Run(stopCh <-chan struct{}, workers int) {
	glog.V(1).Infof("Starting ThirdPartyAnalyticsController\n")

	c.stopChannel = stopCh
	c.startTime = time.Now().UnixNano()

	glog.V(6).Infof("Controller start time: %v", c.startTime)
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

	http.HandleFunc("/healthz", HealthHandler)
	http.HandleFunc("/healthz/ready", HealthHandler)
}

// runWatches will attempt to run all watches in separate goroutines w/ the same stop channel.  Each has its own
// ability to restart the watch if the inner func doing the work fails for any reason.
func (c *AnalyticsController) runWatches() {
	lastResourceVersion := big.NewInt(0)
	currentResourceVersion := big.NewInt(0)
	watchListItems := WatchFuncList(c.kclient, c.client)
	for name := range watchListItems {

		// assign local variable (not in range operator above) so that each
		// goroutine gets the correct watch function required
		wfnc := watchListItems[name]
		n := name
		backoff := 1 * time.Second

		go wait.Until(func() {
			// any return from this func only exits that invocation of the func.
			// wait.Until will call it again after its sync period.
			glog.V(3).Infof("Starting watch for %s", n)
			w, err := wfnc.watchFunc(api.ListOptions{})
			if err != nil {
				glog.Errorf("error creating watch %s: %v", n, err)
			}

			glog.V(6).Infof("Backing off %s watch for %v seconds", n, backoff)
			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}

			if w == nil {
				glog.Errorf("watch function nil, watch not created for %s, returning", n)
				return
			}

			for {
				select {
				case event, ok := <-w.ResultChan():
					if !ok {
						glog.V(3).Infof("%s watch channel closed unexpectedly, attempting to re-establish watch", n)
						return
					}

					if event.Type == watch.Error {
						glog.Errorf("Watch channel for %s returned error: %s", n, spew.Sdump(event))
						return
					}

					// success means the watch is working.
					// reset the backoff back to 1s for this watch
					backoff = 1 * time.Second

					if event.Type == watch.Added || event.Type == watch.Deleted {
						if err != nil {
							glog.Errorf("Unable to create object meta for %v in %s watch", event.Object, n)
							return
						}

						m, err := meta.Accessor(event.Object)
						// if both resource versions can be converted to numbers
						// and if the current resource version is lower than the
						// last recorded resource version for this resource type
						// then skip the event
						if _, ok := lastResourceVersion.SetString(c.watchResourceVersions[n], 10); ok {
							if _, ok = currentResourceVersion.SetString(m.GetResourceVersion(), 10); ok {
								if lastResourceVersion.Cmp(currentResourceVersion) == 1 {
									glog.V(5).Infof("ResourceVersion %v is to old for %v (%v)", currentResourceVersion, n, c.watchResourceVersions[n])
									break
								}
							}
						}

						// each watch is a separate go routine
						c.mutex.Lock()
						c.watchResourceVersions[n] = m.GetResourceVersion()
						c.mutex.Unlock()

						analytic, err := newEvent(event.Object, event.Type)
						if err != nil {
							glog.Errorf("Unexpected error creation analytic in %s watch from watch event %#v", n, event.Object)
						} else {
							// additional info will be set to the analytic and
							// an instance queued for all destinations
							err := c.AddEvent(analytic)
							if err != nil {
								glog.Errorf("Error in %s watch adding event: %v - %v", n, err, analytic)
							}
						}
					}
				}
			}
		}, 1*time.Millisecond, c.stopChannel)
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
	cache.NewReflector(namespaceLW, &api.Namespace{}, c.namespaceStore, 10*time.Minute).Run()

	userLW := &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			return c.client.Users().List(options)
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			return c.client.Users().Watch(options)
		},
	}
	cache.NewReflector(userLW, &userapi.User{}, c.userStore, 10*time.Minute).Run()
}

func (c *AnalyticsController) processAnalyticFromQueue(obj interface{}) error {
	glog.V(6).Infof("Processing analytics event from queue: %v", obj)
	e, ok := obj.(*analyticsEvent)
	if !ok {
		return fmt.Errorf("Expected analyticEvent object but got %v", obj)
	}

	if len(e.destination) == 0 {
		return fmt.Errorf("No destination specified. Ignoring analytic: %v", e)
	}

	glog.V(6).Infof("Attempting to send analytic event to destination %s", e.destination)
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
	glog.V(6).Infof("Adding analyticEvent to queue: %v", ev)
	if len(c.queue.ListKeys()) > c.maximumQueueLength {
		return fmt.Errorf("analyticEvent reject, exceeds maximum queue length: %d - %#v", c.maximumQueueLength, ev)
	}
	if ev.timestamp.UnixNano() < c.startTime {
		glog.V(5).Infof("Warning: analyticEvent is too old: %v", ev)
		return nil
	}

	for destName := range c.destinations {
		ev.destination = destName // needed here to find default ID by destination
		userId, err := c.getUserId(ev)
		if err != nil {
			switch err.reason {
			case missingProjectError, requesterAnnotationNotFoundError:
				glog.V(3).Infoln(err.message)
			case userNotFoundError, noIDFoundError:
				glog.V(5).Infoln(err.message)
			default:
				glog.V(5).Infof("Unexpected error reason '%v' getting user id: %v", err.reason, err.message)
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
			annotations:     make(map[string]string),
			timestamp:       ev.timestamp,
			destination:     destName,
			clusterName:     c.clusterName,
			controllerID:    c.controllerID,
		}
		for key, value := range ev.properties {
			e.properties[key] = value
		}
		for key, value := range ev.annotations {
			e.annotations[key] = value
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
	missingProjectError              = "ProjectNotFoundError"
	requesterAnnotationNotFoundError = "RequesterAnnotationNotFoundError"
	userNotFoundError                = "UserNotFoundError"
	noIDFoundError                   = "NoIDFoundError"
)

// getUserId returns a unique identifier to associate analytics with.
// It will return the identifier based on the UserKeyStrategy and (optionally) UserKeyAnnotation flags:
//   1. UserKeyStrategy="name" will return user.Name
//   2. UserKeyStrategy="uid" will return user.UID
//   3. UserKeyStrategy="annotation" will return a user.Annotations[] value, with the key specified by UserKeyAnnotation
// If an ID cannot be found for any reason, an empty string and error is returned
func (c *AnalyticsController) getUserId(ev *analyticsEvent) (string, *userIDError) {
	username, e := c.getUsernameFromNamespace(ev)
	if e != nil {
		return "", e
	}

	userObj, exists, err := c.userStore.GetByKey(username)
	if err != nil || !exists {
		return "", &userIDError{
			fmt.Sprintf("Failed to find user %s: %v", username, err),
			userNotFoundError,
		}
	}

	if userObj == nil {
		return "", &userIDError{
			fmt.Sprintf("User object nil when trying to get user id"),
			noIDFoundError,
		}
	}

	glog.V(6).Infof("Getting userId with strategy %s", c.userKeyStrategy)
	user := userObj.(*userapi.User)
	switch c.userKeyStrategy {
	case KeyStrategyAnnotation:
		externalId, exists := user.Annotations[c.userKeyAnnotation]
		if exists {
			if len(externalId) > 0 {
				return externalId, nil
			}
		}
		return "", &userIDError{
			fmt.Sprintf("Annotation %s does not exist", c.userKeyAnnotation),
			userNotFoundError,
		}

	case KeyStrategyName:
		if len(user.Name) > 0 {
			return user.Name, nil
		}
		return "", &userIDError{
			fmt.Sprintf("Username does not exist"),
			userNotFoundError,
		}

	case KeyStrategyUID:
		// any non-Online environment (e.g, Dedicated) will never have an externalId
		// the fallback is UserID
		uid := string(user.UID)
		if len(uid) > 0 {
			return uid, nil
		}
		return "", &userIDError{
			fmt.Sprintf("User UID does not exist"),
			userNotFoundError,
		}

	default:
		panic("Invalid user key strategy set")
	}

	return "", &userIDError{
		fmt.Sprintf("No suitable ID could be found for analytic. A user must be logged in for analytics to be counted.  %#v", ev),
		noIDFoundError,
	}
}

func (c *AnalyticsController) getUsernameFromNamespace(ev *analyticsEvent) (string, *userIDError) {
	namespaceName := ev.objectNamespace
	if namespaceName == "" {
		// namespace has no namespace, but its name *is* the namespace
		namespaceName = ev.objectName
	}

	if ev.objectKind == "namespace" {
		username, exists := ev.annotations[projectapi.ProjectRequester]
		if !exists {
			return "", &userIDError{
				fmt.Sprintf("ProjectRequest annotation does not exist on project %s", namespaceName),
				requesterAnnotationNotFoundError,
			}
		}
		return username, nil
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

	return username, nil
}
