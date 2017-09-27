package useranalytics

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/client/testclient"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	userapi "github.com/openshift/origin/pkg/user/apis/user"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
)

func TestAnalyticObjectCreation(t *testing.T) {
	pod := &api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
			CreationTimestamp: metav1.Time{
				time.Now().Add(10 * time.Second),
			},
		},
	}

	ev, err := newEvent(api.Scheme, pod, watch.Added)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	if ev.event != "pod_added" {
		t.Errorf("Expected %v but got %v", "pod_added", ev.event)
	}
	if ev.timestamp.UnixNano() != pod.CreationTimestamp.UnixNano() {
		t.Errorf("Expected %v but got %v", pod.CreationTimestamp.UnixNano(), ev.timestamp.UnixNano())
	}

	pod.DeletionTimestamp = &metav1.Time{time.Now().Add(10 * time.Second)}
	ev, err = newEvent(api.Scheme, pod, watch.Deleted)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	if ev.event != "pod_deleted" {
		t.Errorf("Expected %v but got %v", "pod_deleted", ev.event)
	}
	if ev.timestamp.UnixNano() != pod.DeletionTimestamp.UnixNano() {
		t.Errorf("Expected %v but got %v", pod.DeletionTimestamp.UnixNano(), ev.timestamp.UnixNano())
	}

	ev, err = newEvent(api.Scheme, pod, watch.Modified)
	if err == nil {
		t.Errorf("Expected error but got nil")
	}
}

func TestGetUserId(t *testing.T) {
	tests := map[string]struct {
		defaultId       string
		onlineManagedId string
		expects         string
		config          *AnalyticsControllerConfig
	}{
		"has-managed-online-id": {
			onlineManagedId: "foobar",
			expects:         "foobar",
			config: &AnalyticsControllerConfig{
				Destinations:            make(map[string]Destination),
				KubeClient:              &ktestclient.Clientset{},
				OSClient:                &testclient.Fake{},
				MaximumQueueLength:      10000,
				MetricsPollingFrequency: 5,
				UserKeyAnnotation:       "openshift.io/online-managed-id",
				UserKeyStrategy:         "annotation",
			},
		},
		"fallback-to-user-uid": {
			onlineManagedId: "",
			expects:         "abc123",
			config: &AnalyticsControllerConfig{
				Destinations:            make(map[string]Destination),
				KubeClient:              &ktestclient.Clientset{},
				OSClient:                &testclient.Fake{},
				MaximumQueueLength:      10000,
				MetricsPollingFrequency: 5,
				UserKeyStrategy:         "uid",
			},
		},
	}

	for name, test := range tests {
		test.config.Destinations["mock"] = &WoopraDestination{
			Method:   "GET",
			Domain:   "test",
			Endpoint: "http://127.0.0.1:8888/dest",
			Client:   NewSimpleHttpClient(),
		}

		analyticsController, err := NewAnalyticsController(test.config)
		if err != nil {
			t.Fatalf("Error creating controller %v", err)
		}

		user := &userapi.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo-user",
				UID:  "abc123",
			},
		}
		if test.onlineManagedId != "" {
			user.Annotations = map[string]string{
				analyticsController.userKeyAnnotation: test.onlineManagedId,
			}
		}

		namespace := &api.Namespace{
			ObjectMeta: metav1.ObjectMeta{
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

		event, _ := newEvent(api.Scheme, pod, watch.Added)
		// this is added by AddEvent, which puts many analyticEvents in the queue, one for each destination
		event.destination = "mock"

		userId, err := analyticsController.getUserId(event)
		userIDError := err.(*userIDError)
		if userIDError != nil {
			t.Errorf("Error getting UserID %#v  %#v", err, userId)
		}
		if userId != test.expects {
			t.Errorf("Test %s expects %s but got %s", name, test.expects, userId)
		}
	}
}
