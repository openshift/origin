package subjectaccessreview

import (
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
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
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a subjectAccessReview: %#v", obj))
	}
	if err := kutilerrors.NewAggregate(authorizationvalidation.ValidateSubjectAccessReview(subjectAccessReview)); err != nil {
		return nil, err
	}
	// if a namespace is present on the request, then the namespace on the on the SAR is overwritten.
	// This is to support backwards compatibility.  To have gotten here in this state, it means that
	// the authorizer decided that a user could run an SAR against this namespace
	if namespace := kapi.NamespaceValue(ctx); len(namespace) > 0 {
		subjectAccessReview.Action.Namespace = namespace

	} else if err := r.isAllowed(ctx, subjectAccessReview); err != nil {
		// this check is mutually exclusive to the condition above.  localSAR and localRAR both clear the namespace before delegating their calls
		// We only need to check if the SAR is allowed **again** if the authorizer didn't already approve the request for a legacy call.
		return nil, err

	}

	var userToCheck user.Info
	if (len(subjectAccessReview.User) == 0) && (len(subjectAccessReview.Groups) == 0) {
		// if no user or group was specified, use the info from the context
		ctxUser, exists := kapi.UserFrom(ctx)
		if !exists {
			return nil, kapierrors.NewBadRequest("user missing from context")
		}
		userToCheck = ctxUser

	} else {
		userToCheck = &user.DefaultInfo{
			Name:   subjectAccessReview.User,
			Groups: subjectAccessReview.Groups.List(),
		}

	}

	requestContext := kapi.WithNamespace(kapi.WithUser(ctx, userToCheck), subjectAccessReview.Action.Namespace)
	attributes := authorizer.ToDefaultAuthorizationAttributes(subjectAccessReview.Action)
	allowed, reason, err := r.authorizer.Authorize(requestContext, attributes)
	if err != nil {
		return nil, err
	}

	response := &authorizationapi.SubjectAccessReviewResponse{
		Namespace: subjectAccessReview.Action.Namespace,
		Allowed:   allowed,
		Reason:    reason,
	}

	return response, nil
}

// isAllowed checks to see if the current user has rights to issue a LocalSubjectAccessReview on the namespace they're attempting to access
func (r *REST) isAllowed(ctx kapi.Context, sar *authorizationapi.SubjectAccessReview) error {
	localSARAttributes := authorizer.DefaultAuthorizationAttributes{
		Verb:              "create",
		Resource:          "localsubjectaccessreviews",
		RequestAttributes: sar,
	}
	allowed, reason, err := r.authorizer.Authorize(kapi.WithNamespace(ctx, sar.Action.Namespace), localSARAttributes)

	if err != nil {
		return kapierrors.NewForbidden(localSARAttributes.GetResource(), localSARAttributes.GetResourceName(), err)
	}
	if !allowed {
		forbiddenError, _ := kapierrors.NewForbidden(localSARAttributes.GetResource(), localSARAttributes.GetResourceName(), errors.New("") /*discarded*/).(*kapierrors.StatusError)
		forbiddenError.ErrStatus.Message = reason
		return forbiddenError
	}

	return nil
}
