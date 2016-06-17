package useranalytics

import (
	"strings"
	"testing"
	"os"

	"k8s.io/kubernetes/pkg/api"
	meta "k8s.io/kubernetes/pkg/api/meta"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"github.com/openshift/origin/pkg/client/testclient"
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

	username := os.Getenv("WOOPRA_USERNAME")
	password := os.Getenv("WOOPRA_PASSWORD")

	// only run this live test when the variables are provided externally
	if username == "" || password == "" {
		return
	}

	oc := &testclient.Fake{}
	kc := &ktestclient.Fake{}

	items := watchFuncList(kc, oc, nil)

	for _, w := range items {
		m, err := meta.Accessor(w.objType)
		if err != nil {
			t.Errorf("Unable to create object meta for %v", w.objType)
		}
		m.SetName("foo")
		m.SetNamespace("foobar")

		event, _ := newEvent(w.objType, "added")

		dest := &WoopraDestination{
			Method:   "GET",
			Endpoint: "http://www.woopra.com/track/ce",
			Domain:   "dev.openshift.redhat.com",
			Client:   NewSimpleHttpClient(username, password),
		}

		err = dest.Send(event)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestPrepEndpoint(t *testing.T) {
	before := "http://www.woopra.com/track/ce"
	expected := "http://www.woopra.com/track/ce?%s"
	after := prepEndpoint(before)
	if after != expected {
		t.Errorf("Expected %s, but got %s", expected, after)
	}
}
