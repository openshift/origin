package subjectaccessreview

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
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
	return &authorizationapi.SubjectAccessReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	subjectAccessReview, ok := obj.(*authorizationapi.SubjectAccessReview)
	if !ok {
		return nil, fmt.Errorf("not a subjectAccessReview: %#v", obj)
	}

	namespace := kapi.NamespaceValue(ctx)

	user := &user.DefaultInfo{
		Name:   subjectAccessReview.User,
		Groups: subjectAccessReview.Groups,
	}

	attributes := &authorizer.DefaultAuthorizationAttributes{
		User:      user,
		Verb:      subjectAccessReview.Verb,
		Resource:  subjectAccessReview.Resource,
		Namespace: namespace,
	}

	allowed, reason, err := r.authorizer.Authorize(attributes)
	if err != nil {
		return nil, err
	}

	response := &authorizationapi.SubjectAccessReviewResponse{
		Namespace: namespace,
		Allowed:   allowed,
		Reason:    reason,
	}

	return response, nil
}
