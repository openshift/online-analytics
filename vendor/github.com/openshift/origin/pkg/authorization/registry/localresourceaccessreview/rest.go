package localresourceaccessreview

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationvalidation "github.com/openshift/origin/pkg/authorization/api/validation"
	"github.com/openshift/origin/pkg/authorization/registry/resourceaccessreview"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	clusterRARRegistry resourceaccessreview.Registry
}

func NewREST(clusterRARRegistry resourceaccessreview.Registry) *REST {
	return &REST{clusterRARRegistry}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.LocalResourceAccessReview{}
}

// Create transforms a LocalRAR into an ClusterRAR that is requesting a namespace.  That collapses the code paths.
// LocalResourceAccessReview exists to allow clean expression of policy.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	localRAR, ok := obj.(*authorizationapi.LocalResourceAccessReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a localResourceAccessReview: %#v", obj))
	}
	if errs := authorizationvalidation.ValidateLocalResourceAccessReview(localRAR); len(errs) > 0 {
		return nil, kapierrors.NewInvalid(authorizationapi.Kind(localRAR.Kind), "", errs)
	}
	if namespace := kapi.NamespaceValue(ctx); len(namespace) == 0 {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("namespace is required on this type: %v", namespace))
	} else if (len(localRAR.Action.Namespace) > 0) && (namespace != localRAR.Action.Namespace) {
		return nil, field.Invalid(field.NewPath("namespace"), localRAR.Action.Namespace, fmt.Sprintf("namespace must be: %v", namespace))
	}

	// transform this into a ResourceAccessReview
	clusterRAR := &authorizationapi.ResourceAccessReview{
		Action: localRAR.Action,
	}
	clusterRAR.Action.Namespace = kapi.NamespaceValue(ctx)

	return r.clusterRARRegistry.CreateResourceAccessReview(kapi.WithNamespace(ctx, ""), clusterRAR)
}
