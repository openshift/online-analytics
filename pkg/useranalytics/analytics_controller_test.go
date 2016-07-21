package useranalytics

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/client/testclient"
	projectapi "github.com/openshift/origin/pkg/project/api"
	userapi "github.com/openshift/origin/pkg/user/api"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
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

func TestGetUserId(t *testing.T) {
	tests := map[string]struct {
		defaultId       string
		onlineManagedId string
		expects         string
	}{
		"has-managed-online-id": {
			defaultId:       "",
			onlineManagedId: "foobar",
			expects:         "foobar",
		},
		"uses-default-id": {
			defaultId:       "fizzbuzz",
			onlineManagedId: "",
			expects:         "fizzbuzz",
		},
		"fallback-to-user-uid": {
			defaultId:       "",
			onlineManagedId: "",
			expects:         "abc123",
		},
	}

	for name, test := range tests {
		oc := &testclient.Fake{}
		kc := &ktestclient.Fake{}

		config := &AnalyticsControllerConfig{
			Destinations:            make(map[string]Destination),
			DefaultUserIds:          make(map[string]string),
			KubeClient:              kc,
			OSClient:                oc,
			MaximumQueueLength:      10000,
			MetricsServerPort:       9999,
			MetricsPollingFrequency: 5,
			ProjectWatchFunc:        RealProjectWatchFunc(oc),
			UserWatchFunc:           RealUserWatchFunc(oc),
		}
		config.Destinations["mock"] = &WoopraDestination{
			Method:   "GET",
			Domain:   "test",
			Endpoint: "http://127.0.0.1:8888/dest",
			Client:   NewSimpleHttpClient("", ""),
		}

		if test.defaultId != "" {
			config.DefaultUserIds["mock"] = test.defaultId
		}

		analyticsController, err := NewAnalyticsController(config)
		if err != nil {
			t.Fatalf("Error creating controller %v", err)
		}

		user := &userapi.User{
			ObjectMeta: api.ObjectMeta{
				Name: "foo-user",
				UID:  "abc123",
			},
		}
		if test.onlineManagedId != "" {
			user.Annotations = map[string]string{
				OnlineManagedID: test.onlineManagedId,
			}
		}

		namespace := &api.Namespace{
			ObjectMeta: api.ObjectMeta{
				Name: "foo",
				Annotations: map[string]string{
					projectapi.ProjectRequester: user.Name,
				},
			},
		}

		analyticsController.userStore.Add(user)
		analyticsController.namespaceStore.Add(namespace)

		pod := &api.Pod{}
		pod.Name = "foo"
		pod.Namespace = "foo"

		event, _ := newEvent(pod, watch.Added)
		// this is added by AddEvent, which puts many analyticEvents in the queue, one for each destination
		event.destination = "mock"

		userId, err := analyticsController.getUserId(event)
		if err != nil {
			t.Errorf("Error gettign UserID %v", err)
		}

		if userId != test.expects {
			t.Errorf("Test %s expects %s but got %s", name, test.expects, userId)
		}
	}
}
