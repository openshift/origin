package localsubjectaccessreview

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationvalidation "github.com/openshift/origin/pkg/authorization/api/validation"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	clusterSARRegistry subjectaccessreview.Registry
}

func NewREST(clusterSARRegistry subjectaccessreview.Registry) *REST {
	return &REST{clusterSARRegistry}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.LocalSubjectAccessReview{}
}

// Create transforms a LocalSAR into an ClusterSAR that is requesting a namespace.  That collapses the code paths.
// LocalSubjectAccessReview exists to allow clean expression of policy.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	localSAR, ok := obj.(*authorizationapi.LocalSubjectAccessReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a localSubjectAccessReview: %#v", obj))
	}
	if err := kutilerrors.NewAggregate(authorizationvalidation.ValidateLocalSubjectAccessReview(localSAR)); err != nil {
		return nil, err
	}
	if namespace := kapi.NamespaceValue(ctx); len(namespace) == 0 {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("namespace is required on this type: %v", namespace))
	} else if (len(localSAR.Action.Namespace) > 0) && (namespace != localSAR.Action.Namespace) {
		return nil, fielderrors.NewFieldInvalid("namespace", localSAR.Action.Namespace, fmt.Sprintf("namespace must be: %v", namespace))
	}

	// transform this into a SubjectAccessReview
	clusterSAR := &authorizationapi.SubjectAccessReview{
		Action: localSAR.Action,
		User:   localSAR.User,
		Groups: localSAR.Groups,
	}
	clusterSAR.Action.Namespace = kapi.NamespaceValue(ctx)

	return r.clusterSARRegistry.CreateSubjectAccessReview(kapi.WithNamespace(ctx, ""), clusterSAR)
}
