package useranalytics

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

const onlineManagedID = "openshift.io/online-managed-id"

type watchListItem struct {
	objType   runtime.Object
	watchFunc func(options api.ListOptions) (watch.Interface, error)
	isOS      bool
}

// watchFuncList returns all the objects and watch functions we're using for analytics.
// projectHackFunc is a workaround that will be removed after OSE is updated and the Project client has a proper watch func.
func watchFuncList(kubeClient kclient.Interface, osClient osclient.Interface, projectHackFunc func(options api.ListOptions) (watch.Interface, error)) map[string]*watchListItem {
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
			objType: &api.Service{},
			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return kubeClient.Namespaces().Watch(options)
			},
		},

		// Openshift objects
		"projects": {
			objType:   &projectapi.Project{},
			watchFunc: projectHackFunc,
			isOS:      true,
		},
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
		//		"role-binding": {
		//			objType: &authorizationapi.RoleBinding{},
		//			listFunc: func(options api.ListOptions) (runtime.Object, error) {
		//				return osClient.RoleBindings(api.NamespaceAll).List(options)
		//			},
		//			watchFunc: func(options api.ListOptions) (watch.Interface, error) {
		//				return osClient.RoleBindings(api.NamespaceAll).Watch(options)
		//			},
		//		},
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
	}
}

// TODO - remove me when Online gets rebased with latest OSE and a proper Project watch.
// This is the production implementation while a mock implementation was made in the unit tests.
// The mock impl cannot be cast to RESTClient.
func RealProjectWatchFunc(oc osclient.Interface) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		restClient, ok := oc.(*osclient.Client)
		if !ok {
			return nil, fmt.Errorf("client is not RESTClient: %v", oc)
		}
		return restClient.Get().Prefix("watch").Resource("projects").VersionedParams(&options, api.ParameterCodec).Watch()
	}
}

// same comments as above apply to this hack
func RealUserWatchFunc(oc osclient.Interface) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		restClient, ok := oc.(*osclient.Client)
		if !ok {
			return nil, fmt.Errorf("client is not RESTClient: %v", oc)
		}
		return restClient.Get().Prefix("watch").Resource("users").VersionedParams(&options, api.ParameterCodec).Watch()
	}
}
