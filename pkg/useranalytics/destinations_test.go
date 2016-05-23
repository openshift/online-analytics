package useranalytics

import (
	"k8s.io/kubernetes/pkg/api"
	"strings"
	"testing"
)

func TestIntercomDestination(t *testing.T) {
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
	}

	// TODO:  this needs some kind of factory per object
	// that creates analyticsEvent objects
	event, _ := newEvent(pod, "added")

	dest := &IntercomDestination{
		Client: &mockIntercomEventClient{},
	}

	err := dest.Send(event)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	mockClient := dest.Client.(*mockIntercomEventClient)

	if mockClient.event == nil {
		t.Error("Expected non-nil event")
	}

	if mockClient.event.Email != pod.Namespace {
		t.Errorf("Expected %s, but got %s", pod.Namespace, mockClient.event.Email)
	}
}

func TestWoopraDestination(t *testing.T) {
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
	}

	// TODO:  this needs some kind of factory per object
	// that creates analyticsEvent objects
	event, _ := newEvent(pod, "created")

	dest := &WoopraDestination{
		Method:   "GET",
		Endpoint: "http://www.woopra.com/track/ce",
		Domain:   "dev.openshift.redhat.com",
		Client:   &mockHttpClient{},
	}

	err := dest.Send(event)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	mockClient := dest.Client.(*mockHttpClient)

	if !strings.Contains(mockClient.url, dest.Endpoint) {
		t.Errorf("Expected the destination to include %s , but got: %s", dest.Endpoint, mockClient.url)
	}

}


func TestWoopraLive(t *testing.T) {
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:      "foo",
			Namespace: "test-foo-bar",
		},
	}

	// TODO:  this needs some kind of factory per object
	// that creates analyticsEvent objects
	event, _ := newEvent(pod, "added")

	dest := &WoopraDestination{
		Method:   "GET",
		Endpoint: "http://www.woopra.com/track/ce",
		Domain:   "dev.openshift.redhat.com",
		Client:   NewSimpleHttpClient("REPLACE ME w/ username", "REPLACE ME w/ password"),
	}

	err := dest.Send(event)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	dest.Send(event)


}

func TestPrepEndpoint(t *testing.T) {
	before := "http://www.woopra.com/track/ce"
	expected := "http://www.woopra.com/track/ce?%s"
	after := prepEndpoint(before)
	if after != expected {
		t.Errorf("Expected %s, but got %s", expected, after)
	}
}
