package resourceaccessreview

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	authorizer authorizer.Authorizer
}

// NewREST creates a new REST for policies.
func NewREST(authorizer authorizer.Authorizer) apiserver.RESTStorage {
	return &REST{authorizer}
}

// New creates a new ResourceAccessReview object
func (r *REST) New() runtime.Object {
	return &authorizationapi.ResourceAccessReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	resourceAccessReview, ok := obj.(*authorizationapi.ResourceAccessReview)
	if !ok {
		return nil, fmt.Errorf("not a resourceAccessReview: %#v", obj)
	}

	attributes := &authorizer.DefaultAuthorizationAttributes{
		Verb:         resourceAccessReview.Spec.Verb,
		ResourceKind: resourceAccessReview.Spec.ResourceKind,
		Namespace:    kapi.Namespace(ctx),
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		users, groups, err := r.authorizer.GetAllowedSubjects(attributes)
		if err != nil {
			return nil, err
		}

		resourceAccessReview.Status = authorizationapi.ResourceAccessReviewStatus{
			Users:  users,
			Groups: groups,
		}

		return resourceAccessReview, nil
	}), nil

}
