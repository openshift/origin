package subjectaccessreview

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/runtime"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationvalidation "github.com/openshift/origin/pkg/authorization/api/validation"
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
	return &authorizationapi.SubjectAccessReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	subjectAccessReview, ok := obj.(*authorizationapi.SubjectAccessReview)
	if !ok {
		return nil, kerrors.NewBadRequest(fmt.Sprintf("not a subjectAccessReview: %#v", obj))
	}
	if err := kutilerrors.NewAggregate(authorizationvalidation.ValidateSubjectAccessReview(subjectAccessReview)); err != nil {
		return nil, err
	}

	var userToCheck user.Info
	if (len(subjectAccessReview.User) == 0) && (len(subjectAccessReview.Groups) == 0) {
		// if no user or group was specified, use the info from the context
		ctxUser, exists := kapi.UserFrom(ctx)
		if !exists {
			return nil, kerrors.NewBadRequest("user missing from context")
		}
		userToCheck = ctxUser

	} else {
		userToCheck = &user.DefaultInfo{
			Name:   subjectAccessReview.User,
			Groups: subjectAccessReview.Groups.List(),
		}

	}

	namespace := kapi.NamespaceValue(ctx)
	requestContext := kapi.WithUser(ctx, userToCheck)

	attributes := &authorizer.DefaultAuthorizationAttributes{
		Verb:     subjectAccessReview.Verb,
		Resource: subjectAccessReview.Resource,
	}

	allowed, reason, err := r.authorizer.Authorize(requestContext, attributes)
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
