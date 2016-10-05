package useranalytics

import (
	"k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

const OnlineManagedID = "openshift.io/online-managed-id"

type watchListItem struct {
	objType   runtime.Object
	watchFunc func(options api.ListOptions) (watch.Interface, error)
	isOS      bool
}

// watchFuncList returns all the objects and watch functions we're using for analytics.
func WatchFuncList(kubeClient kclient.Interface, osClient osclient.Interface) map[string]*watchListItem {
	return map[string]*watchListItem{
		// Kubernetes objects
		"pods": {
			objType: &api.Pod{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return kubeClient.Pods(api.NamespaceAll).Watch(options)
			},
		},
		"replicationcontrollers": {
			objType: &api.ReplicationController{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return kubeClient.ReplicationControllers(api.NamespaceAll).Watch(options)
			},
		},
		"persistentvolumeclaims": {
			objType: &api.PersistentVolumeClaim{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return kubeClient.PersistentVolumeClaims(api.NamespaceAll).Watch(options)
			},
		},
		"secrets": {
			objType: &api.Secret{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return kubeClient.Secrets(api.NamespaceAll).Watch(options)
			},
		},
		"services": {
			objType: &api.Service{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return kubeClient.Services(api.NamespaceAll).Watch(options)
			},
		},
		"namespaces": {
			objType: &api.Namespace{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return kubeClient.Namespaces().Watch(options)
			},
		},
		// Openshift objects
		"deploymentconfigs": {
			objType: &deployapi.DeploymentConfig{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return osClient.DeploymentConfigs(api.NamespaceAll).Watch(options)
			},
			isOS: true,
		},
		"routes": {
			objType: &routeapi.Route{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return osClient.Routes(api.NamespaceAll).Watch(options)
			},
			isOS: true,
		},
		"builds": {
			objType: &buildapi.Build{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return osClient.Builds(api.NamespaceAll).Watch(options)
			},
			isOS: true,
		},
		"templates": {
			objType: &templateapi.Template{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return osClient.Templates(api.NamespaceAll).Watch(options)
			},
			isOS: true,
		},
		"imagestreams": {
			objType: &imageapi.ImageStream{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return osClient.ImageStreams(api.NamespaceAll).Watch(options)
			},
			isOS: true,
		},
		"users": {
			objType: &userapi.User{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return osClient.Users().Watch(options)
			},
			isOS: true,
		},
	}
}
