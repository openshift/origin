package subjectaccessreview

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authenticationapi "github.com/openshift/origin/pkg/auth/api"
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
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	subjectAccessReview, ok := obj.(*authorizationapi.SubjectAccessReview)
	if !ok {
		return nil, fmt.Errorf("not a subjectAccessReview: %#v", obj)
	}

	user := authenticationapi.DefaultUserInfo{}

	if len(subjectAccessReview.Spec.Group) > 0 {
		user.Groups = append(user.Groups, subjectAccessReview.Spec.Group)
	} else {
		user.Name = subjectAccessReview.Spec.User
		user.Groups = subjectAccessReview.Spec.Groups
	}

	attributes := &authorizer.DefaultAuthorizationAttributes{
		User:         user,
		Verb:         subjectAccessReview.Spec.Verb,
		ResourceKind: subjectAccessReview.Spec.ResourceKind,
		Namespace:    kapi.Namespace(ctx),
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		allowed, reason, err := r.authorizer.Authorize(attributes)

		subjectAccessReview.Status = authorizationapi.SubjectAccessReviewStatus{
			Allowed: allowed,
			Reason:  reason,
		}
		if err != nil {
			subjectAccessReview.Status.EvaluationError = err.Error()
		}

		return subjectAccessReview, nil
	}), nil

}
