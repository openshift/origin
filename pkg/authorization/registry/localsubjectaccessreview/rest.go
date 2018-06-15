package localsubjectaccessreview

import (
	"fmt"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationvalidation "github.com/openshift/origin/pkg/authorization/apis/authorization/validation"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	clusterSARRegistry subjectaccessreview.Registry
}

var _ rest.Creater = &REST{}

func NewREST(clusterSARRegistry subjectaccessreview.Registry) *REST {
	return &REST{clusterSARRegistry}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.LocalSubjectAccessReview{}
}

// Create transforms a LocalSAR into an ClusterSAR that is requesting a namespace.  That collapses the code paths.
// LocalSubjectAccessReview exists to allow clean expression of policy.
func (r *REST) Create(ctx apirequest.Context, obj runtime.Object, _ rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
	localSAR, ok := obj.(*authorizationapi.LocalSubjectAccessReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a localSubjectAccessReview: %#v", obj))
	}
	if errs := authorizationvalidation.ValidateLocalSubjectAccessReview(localSAR); len(errs) > 0 {
		return nil, kapierrors.NewInvalid(authorizationapi.Kind(localSAR.Kind), "", errs)
	}
	if namespace := apirequest.NamespaceValue(ctx); len(namespace) == 0 {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("namespace is required on this type: %v", namespace))
	} else if (len(localSAR.Action.Namespace) > 0) && (namespace != localSAR.Action.Namespace) {
		return nil, field.Invalid(field.NewPath("namespace"), localSAR.Action.Namespace, fmt.Sprintf("namespace must be: %v", namespace))
	}

	// transform this into a SubjectAccessReview
	clusterSAR := &authorizationapi.SubjectAccessReview{
		Action: localSAR.Action,
		User:   localSAR.User,
		Groups: localSAR.Groups,
		Scopes: localSAR.Scopes,
	}
	clusterSAR.Action.Namespace = apirequest.NamespaceValue(ctx)

	return r.clusterSARRegistry.CreateSubjectAccessReview(apirequest.WithNamespace(ctx, ""), clusterSAR)
}
