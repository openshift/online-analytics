package useranalytics

import (
	"testing"

	buildv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	deployv1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
	imagev1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	routev1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	templatev1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubernetes/pkg/api"
)

func TestHash(t *testing.T) {
	typer := api.Scheme

	hashedValues := make(map[string]bool)

	for _, test := range []struct {
		expectedMatch bool
		obj           interface{}
	}{
		{
			expectedMatch: false,
			obj: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
		},
		{
			expectedMatch: false,
			obj: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo2",
					Namespace: "bar",
				},
			},
		},
		{
			expectedMatch: true,
			obj: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
		},
		{
			expectedMatch: false,
			obj: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo3",
					Namespace: "bar",
				},
			},
		},
	} {
		event, err := newEvent(typer, test.obj, watch.Added)
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
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "pod",
				event:           "pod_added",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"ReplicationController": {
			input: &api.ReplicationController{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "replicationcontroller",
				event:           "replicationcontroller_added",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"PersistentVolumeClaim": {
			input: &api.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "persistentvolumeclaim",
				event:           "persistentvolumeclaim_added",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Secret": {
			input: &api.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "secret",
				event:           "secret_added",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Service": {
			input: &api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "service",
				event:           "service_added",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Namespace": {
			input: &api.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "namespace",
				event:           "namespace_added",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Deployment": {
			input: &deployv1.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "deployment",
				event:           "deploymentconfig_added",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Route": {
			input: &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "route",
				event:           "route_added",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"Build": {
			input: &buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "build",
				event:           "build_added",
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
			input: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "template",
				event:           "template_added",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
		"ImageStream": {
			input: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: &analyticsEvent{
				objectKind:      "imagestream",
				event:           "imagestream_added",
				objectName:      "foo",
				objectNamespace: "bar",
			},
		},
	}

	for name, test := range tests {
		event, err := newEvent(api.Scheme, test.input, watch.Added)
		if err != nil {
			t.Errorf("Test %s got unexpected error: %v", name, err)
		}
		if event.event != test.expected.event {
			t.Errorf("Test %s expected event '%s' but got '%s'", name, test.expected.event, event.event)
		}
		if event.objectName != test.expected.objectName {
			t.Errorf("Expected %s but got %s", test.expected.objectName, event.objectName)
		}
		if event.objectNamespace != test.expected.objectNamespace {
			t.Errorf("Test %s expected %s but got %s", name, test.expected.objectName, event.objectName)
		}
	}
}
