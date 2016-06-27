package useranalytics

import (
	"testing"

	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"
)

func TestHash(t *testing.T) {

	hashedValues := make(map[string]bool)

	for _, test := range []struct {
		expectedMatch bool
		obj           interface{}
	}{
		{
			expectedMatch: false,
			obj: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
		},
		{
			expectedMatch: false,
			obj: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo2",
					Namespace: "bar",
				},
			},
		},
		{
			expectedMatch: true,
			obj: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
		},
		{
			expectedMatch: false,
			obj: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo3",
					Namespace: "bar",
				},
			},
		},
	} {
		event, err := newEvent(test.obj, watch.Added)
		if err != nil {
			t.Errorf("Unexpected error %s", err)
		}
		if _, exists := hashedValues[event.Hash()]; exists && !test.expectedMatch {
			t.Errorf("Did not expect to find hashed value for analytic %#v", event)
		}
	}
}

func TestAnalyticEventFactoryFuncs(t *testing.T) {
	tests := map[string]struct {
		input    interface{}
		expected *analyticsEvent
	}{
		"Pod": {
			input: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "pod",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"ReplicationController": {
			input: &api.ReplicationController{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "replicationcontroller",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"PersistentVolumeClaim": {
			input: &api.PersistentVolumeClaim{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "persistentvolumeclaim",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Secret": {
			input: &api.Secret{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "secret",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Service": {
			input: &api.Service{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "service",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Namespace": {
			input: &api.Namespace{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "namespace",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Deployment": {
			input: &deployapi.DeploymentConfig{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "deployment",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Route": {
			input: &routeapi.Route{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "route",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Build": {
			input: &buildapi.Build{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "build",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		//		"RoleBinding": {
		//			input: &api.RoleBinding{
		//				ObjectMeta: .ObjectMeta{
		//					Name: "foo",
		//					Namespace:"bar",
		//				},
		//			},
		//			expected: &analyticsEvent{
		//				objectKind: "rolebinding",
		//				event: "add",
		//				objectName: "foo",
		//				objectNamespace: "bar",
		//			},
		//		},
		"Template": {
			input: &templateapi.Template{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "template",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"ImageStream": {
			input: &imageapi.ImageStream{
				ObjectMeta: api.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "imagestream",
				event:           "add",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
	}

	for name, test := range tests {
		event, _ := newEvent(test.input, watch.Added)
		if event.objectName != test.expected.objectName {
			t.Errorf("Expected %s but got %s", test.expected.objectName, event.objectName)
		}
		if event.objectNamespace != test.expected.objectNamespace {
			t.Errorf("Test %s expected %s but got %s", name, test.expected.objectName, event.objectName)
		}
	}
}
