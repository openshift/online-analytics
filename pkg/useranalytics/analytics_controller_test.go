package useranalytics

import (
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/watch"
)

func TestAnalyticObjectCreation(t *testing.T) {
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
			CreationTimestamp: unversioned.Time{
				time.Now().Add(10 * time.Second),
			},
		},
	}

	ev, err := newEvent(pod, watch.Added)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	if ev.event != "pod_added" {
		t.Errorf("Expected %v but got %v", "pod_added", ev.event)
	}
	if ev.timestamp.UnixNano() != pod.CreationTimestamp.UnixNano() {
		t.Errorf("Expected %v but got %v", pod.CreationTimestamp.UnixNano(), ev.timestamp.UnixNano())
	}

	pod.DeletionTimestamp = &unversioned.Time{time.Now().Add(10 * time.Second)}
	ev, err = newEvent(pod, watch.Deleted)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	if ev.event != "pod_deleted" {
		t.Errorf("Expected %v but got %v", "pod_deleted", ev.event)
	}
	if ev.timestamp.UnixNano() != pod.DeletionTimestamp.UnixNano() {
		t.Errorf("Expected %v but got %v", pod.DeletionTimestamp.UnixNano(), ev.timestamp.UnixNano())
	}

	ev, err = newEvent(pod, watch.Modified)
	if err == nil {
		t.Errorf("Expected error but got nil")
	}
}

//
//func TestController(t *testing.T) {
//	mockClient := mockClient()
//	oc := &testclient.Fake{}
//	kc := &ktestclient.Fake{}
//
//	config := &AnalyticsControllerConfig{
//		Destinations: map[string]Destination{
//			"anywhere": &mockDestination{},
//		},
//		DefaultUserIds: map[string]string{
//			"anywhere": "default-id",
//		},
//		QueueClient:        mockClient,
//		MaximumQueueLength: 1000000,
//		OSClient: oc,
//		KubeClient: kc,
//	}
//
//	ctrl, err := NewAnalyticsController(config)
//	if err != nil {
//		t.Fatalf("Unexpected error: %v", err)
//	}
//
//	ctrl.startTime = time.Now().UnixNano()
//
//	pod := &api.Pod{
//		ObjectMeta: api.ObjectMeta{
//			Name:      "foo",
//			Namespace: "bar",
//			CreationTimestamp: unversioned.Time{
//				time.Now().Add(10 * time.Second),
//			},
//		},
//	}
//
//	ev, err := newEvent(pod, "add")
//	ev.destination = "anywhere"
//	if err != nil {
//		t.Errorf("Unexpected error: %v", err)
//	}
//
//	err = ctrl.AddEvent(ev)
//	if err != nil {
//		t.Errorf("Unexpected error: %v", err)
//	}
//
//	// past timestamp should be rejected
//	ev.timestamp = time.Now().AddDate(-1, 1, 1)
//	err = ctrl.AddEvent(ev)
//	if err == nil {
//		t.Error("Expected timestamp error")
//	}
//
//	// current analytic event but missing UserID, expected default userId
//	ev, _ = newEvent(pod, "add")
//	ev.destination = "foo"
//	ev.timestamp = time.Now()
//	delete(mockClient.user.Annotations, onlineManagedID)
//
//	err = ctrl.AddEvent(ev)
//	if err != nil {
//		t.Errorf("Unexpected error: %v", err)
//	}
//	if ev.userID != config.DefaultUserIds["anywhere"] {
//		t.Errorf("Unexpected error: %v", err)
//	}
//
//	// two events accepted above (one rejected for too old)
//	if len(ctrl.queue.ListKeys()) != 2 {
//		t.Errorf("Expected queue.length = %d but got %s", 2, len(ctrl.queue.ListKeys()))
//	}
//
//	// bounded queue returns an error when max queue length is exceeded
//	ctrl.maximumQueueLength = 0
//	err = ctrl.AddEvent(ev)
//	if err == nil {
//		t.Error("Expected maximum queue length error but got nil")
//	}
//
//	if len(ctrl.queue.ListKeys()) != 2 {
//		t.Errorf("Expected cache size %d but got %d", 2, len(ctrl.queue.ListKeys()))
//	}
//}
//
//func mockProjectWatchFunc(oc osclient.Interface) func(options api.ListOptions) (watch.Interface, error) {
//	fakeClient := oc.(*testclient.Fake)
//	return func(options api.ListOptions) (watch.Interface, error) {
//		return fakeClient.InvokesWatch(ktestclient.NewRootWatchAction("projects", api.ListOptions{}))
//	}
//}
//
//func mockUserWatchFunc(oc osclient.Interface) func(options api.ListOptions) (watch.Interface, error) {
//	fakeClient := oc.(*testclient.Fake)
//	return func(options api.ListOptions) (watch.Interface, error) {
//		return fakeClient.InvokesWatch(ktestclient.NewRootWatchAction("users", api.ListOptions{}))
//	}
//}
//
////func TestFramework(t *testing.T) {
////	oc := &testclient.Fake{}
////	kc := &ktestclient.Fake{}
////
////	projectWatchFunc := mockProjectWatchFunc(oc)
////	userWatchFunc := mockUserWatchFunc(oc)
////
////	fakeWatches := make(map[string]*watch.FakeWatcher)
////	for objKind, item := range watchFuncList(kc, oc, projectWatchFunc) {
////		kfakeWatch := watch.NewFake()
////		if item.isOS {
////			oc.AddWatchReactor(objKind, func(action ktestclient.Action) (bool, watch.Interface, error) {
////				return true, kfakeWatch, nil
////			})
////		} else {
////			kc.AddWatchReactor(objKind, func(action ktestclient.Action) (bool, watch.Interface, error) {
////				return true, kfakeWatch, nil
////			})
////		}
////		t.Logf("Adding %s fake watch %v\n", objKind, kfakeWatch)
////		fakeWatches[objKind] = kfakeWatch
////	}
////
////	fakeUserWatch := watch.NewFake()
////	oc.AddWatchReactor("users", func(action ktestclient.Action) (bool, watch.Interface, error) {
////		return true, fakeUserWatch, nil
////	})
////
////	stop := make(chan struct {})
////	defer close(stop)
////
////	endpoint := &mockHttpEndpoint{
////		port:       9006,
////		minLatency: 10,
////		maxLatency: 20,
////		flakeRate:  0,
////		urlPrefix:  "mock",
////	}
////	endpoint.start(stop)
////
////	config := &AnalyticsControllerConfig{
////		Destinations: map[string]Destination{
////			"anywhere": &WoopraDestination{
////				Method:   "GET",
////				Endpoint: "http://127.0.0.1:9006/mock",
////				Domain:   "domain",
////				Client:   NewSimpleHttpClient("username", "password"),
////			},
////		},
////		DefaultUserIds: map[string]string{
////			"anywhere": "default-id",
////		},
////		KubeClient:              kc,
////		OSClient:                oc,
////		MaximumQueueLength:      1000000,
////		MetricsServerPort:       9999,
////		MetricsPollingFrequency: 1,
////		ProjectWatchFunc:        projectWatchFunc,
////		UserWatchFunc:           userWatchFunc,
////	}
////
////	ctrl, err := NewAnalyticsController(config)
////	if err != nil {
////		t.Fatalf("Unexpected error: %v", err)
////	}
////
////	go ctrl.Run(stop, 3)
////
////	// give the watches a moment to start
////	time.Sleep(100 * time.Millisecond)
////
////	fakeUserWatch.Add(&userapi.User{
////		ObjectMeta: api.ObjectMeta{
////			Name:      "user-foo",
////			Annotations: map[string]string{
////				onlineManagedID: "distinct-id-from-some-system",
////			},
////		},
////	})
////
////	projectWatch := fakeWatches["projects"]
////	projectWatch.Add(&projectapi.Project{
////		ObjectMeta: api.ObjectMeta{
////			Name:      "project-foo",
////			Annotations: map[string]string{
////				projectapi.ProjectRequester: "user-foo",
////			},
////		},
////	})
////
////	// let the watches catch up and add the user & project
////	time.Sleep(100 * time.Millisecond)
////
////	if len(ctrl.userStore.ListKeys()) != 1 {
////		t.Fatal("Expected to have 1 user in the local cache")
////	}
////
////	if len(ctrl.projectStore.ListKeys()) != 1 {
////		t.Fatalf("Expected to have 1 project in the local cache, project watch = %v", projectWatch)
////	}
////
////	// let the endpoint startup...
////	time.Sleep(100 * time.Millisecond)
////
////	// this go routine gets metrics from the metricsPort in the controller
////	go wait.Until(func() {
////		client := &http.Client{}
////		endpoint := fmt.Sprintf("http://localhost:%d/metrics", ctrl.metricsServerPort)
////		req, _ := http.NewRequest("GET", endpoint, nil)
////		resp, err := client.Do(req)
////		if err != nil {
////			t.Error("Error pinging metrics endpoint %v", err)
////		}
////		if resp != nil {
////			body, _ := ioutil.ReadAll(resp.Body)
////			t.Logf("Unit test reporting - metrics are: %s", body)
////		}
////	}, 5 * time.Second, stop)
////
////	timeout := 3 * time.Second
////	started := time.Now()
////	numberOfObjects := 10
////
////	//	for i := 0; i <= numberOfObjects; i++ {
////	if w, ok := fakeWatches["pods"]; ok {
////		pod := &api.Pod{
////			ObjectMeta: api.ObjectMeta{
////				Name:      string(util.NewUUID()),
////				Namespace: "project-foo",
////				CreationTimestamp: unversioned.Time{
////					time.Now().Add(10 * time.Second),
////				},
////			},
////		}
////		t.Logf("Pod watch obj = %v", w)
////		w.Add(pod)
////	} else {
////		t.Fatalf("No watch found for %s", "pods")
////	}
////	//	}
////
////	//	t.Fatal("never gets here... ")
////
////	for endpoint.handled < numberOfObjects {
////		time.Sleep(100 * time.Millisecond)
////		t.Logf("Waiting.  Last = %s", endpoint.last)
////
////		if time.Now().After(started.Add(timeout)) {
////			t.Fatal("Timeout")
////		}
////	}
////
////	t.Logf("Last = %s\n", endpoint.last)
////	t.Logf("Handled = %d\n", endpoint.handled)
////}
//
////
////func TestWatches(t *testing.T) {
////	kc := &ktestclient.Fake{}
////	kfakeWatch := watch.NewFake()
////	kc.AddWatchReactor("pods", func(action ktestclient.Action) (bool, watch.Interface, error) {
////		return true, kfakeWatch, nil
////	})
////
////	stop := make(chan struct{})
////	defer close(stop)
////
////	// this go routine gets metrics from the metricsPort in the controller
////	go wait.Until(func() {
////		pod := &api.Pod{
////			ObjectMeta: api.ObjectMeta{
////				Name:      string(util.NewUUID()),
////				Namespace: "bar",
////				CreationTimestamp: unversioned.Time{
////					time.Now().Add(10 * time.Second),
////				},
////			},
////		}
////		kfakeWatch.Add(pod)
////	}, 1*time.Second, stop)
////
////	w, err := kc.Pods(api.NamespaceAll).Watch(api.ListOptions{})
////	if err != nil {
////		t.Errorf("Unexpected error %v", err)
////	}
////
////	eventsHandled := 0
////	maxEvents := 5
////	timeout := 5 * time.Second
////	started := time.Now()
////
////	for {
////		select {
////		case event := <-w.ResultChan():
////			if event.Type != watch.Added {
////				t.Errorf("Expected: %s, got: %s", watch.Added, event.Type)
////			} else {
////				eventsHandled++
////			}
////
////			fmt.Printf("and the event is %#v\n", event)
////
////		case <-time.After(wait.ForeverTestTimeout):
////			t.Errorf("Timed out waiting for an event")
////		}
////
////		if eventsHandled >= maxEvents {
////			t.Logf("handled the events we were waiting for")
////			break
////		}
////
////		if time.Now().After(started.Add(timeout)) {
////			t.Fatal("\n\nTimeout\n\n")
////		}
////	}
////
////	//	t.Error("FAIL!")
////}
