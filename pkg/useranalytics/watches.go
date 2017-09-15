package useranalytics

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	buildv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	osclient "github.com/openshift/origin/pkg/client"
	deployv1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
	imagev1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	routev1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	templatev1 "github.com/openshift/origin/pkg/template/apis/template/v1"
)

type watchListItem struct {
	objType   runtime.Object
	watchFunc func(options metav1.ListOptions) (watch.Interface, error)
	isOS      bool
}

// watchFuncList returns all the objects and watch functions we're using for analytics.
func WatchFuncList(kubeClient kclientset.Interface, osClient osclient.Interface) map[string]*watchListItem {
	return map[string]*watchListItem{
		// Kubernetes objects
		"pods": {
			objType: &api.Pod{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.Core().Pods(api.NamespaceAll).Watch(options)
			},
		},
		"replicationcontrollers": {
			objType: &api.ReplicationController{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.Core().ReplicationControllers(api.NamespaceAll).Watch(options)
			},
		},
		"persistentvolumeclaims": {
			objType: &api.PersistentVolumeClaim{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.Core().PersistentVolumeClaims(api.NamespaceAll).Watch(options)
			},
		},
		"secrets": {
			objType: &api.Secret{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.Core().Secrets(api.NamespaceAll).Watch(options)
			},
		},
		"services": {
			objType: &api.Service{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.Core().Services(api.NamespaceAll).Watch(options)
			},
		},
		"namespaces": {
			objType: &api.Namespace{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.Core().Namespaces().Watch(options)
			},
		},
		// Openshift objects
		"deploymentconfigs": {
			objType: &deployv1.DeploymentConfig{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return osClient.DeploymentConfigs(api.NamespaceAll).Watch(options)
			},
			isOS: true,
		},
		"routes": {
			objType: &routev1.Route{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return osClient.Routes(api.NamespaceAll).Watch(options)
			},
			isOS: true,
		},
		"builds": {
			objType: &buildv1.Build{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return osClient.Builds(api.NamespaceAll).Watch(options)
			},
			isOS: true,
		},
		"templates": {
			objType: &templatev1.Template{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return osClient.Templates(api.NamespaceAll).Watch(options)
			},
			isOS: true,
		},
		"imagestreams": {
			objType: &imagev1.ImageStream{},
			watchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return osClient.ImageStreams(api.NamespaceAll).Watch(options)
			},
			isOS: true,
		},
	}
}
