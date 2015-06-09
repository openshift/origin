package resourceaccessreview

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	authorizer authorizer.Authorizer
}

// NewREST creates a new REST for policies.
func NewREST(authorizer authorizer.Authorizer) *REST {
	return &REST{authorizer}
}

// New creates a new ResourceAccessReview object
func (r *REST) New() runtime.Object {
	return &authorizationapi.ResourceAccessReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	resourceAccessReview, ok := obj.(*authorizationapi.ResourceAccessReview)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("not a resourceAccessReview: %#v", obj))
	}

	namespace := kapi.NamespaceValue(ctx)

	attributes := &authorizer.DefaultAuthorizationAttributes{
		Verb:     resourceAccessReview.Verb,
		Resource: resourceAccessReview.Resource,
	}

	users, groups, err := r.authorizer.GetAllowedSubjects(ctx, attributes)
	if err != nil {
		return nil, err
	}

	response := &authorizationapi.ResourceAccessReviewResponse{
		Namespace: namespace,
		Users:     users,
		Groups:    groups,
	}

	return response, nil
}
