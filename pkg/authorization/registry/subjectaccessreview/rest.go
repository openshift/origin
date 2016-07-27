package subjectaccessreview

import (
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/runtime"

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
	if errs := authorizationvalidation.ValidateSubjectAccessReview(subjectAccessReview); len(errs) > 0 {
		return nil, kapierrors.NewInvalid(authorizationapi.Kind(subjectAccessReview.Kind), "", errs)
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

	var userToCheck *user.DefaultInfo
	if (len(subjectAccessReview.User) == 0) && (len(subjectAccessReview.Groups) == 0) {
		// if no user or group was specified, use the info from the context
		ctxUser, exists := kapi.UserFrom(ctx)
		if !exists {
			return nil, kapierrors.NewBadRequest("user missing from context")
		}
		// make a copy, we don't want to risk changing the original
		newExtra := map[string][]string{}
		for k, v := range ctxUser.GetExtra() {
			if v == nil {
				newExtra[k] = nil
				continue
			}
			newSlice := make([]string, len(v), len(v))
			copy(newSlice, v)
			newExtra[k] = newSlice
		}

		userToCheck = &user.DefaultInfo{
			Name:   ctxUser.GetName(),
			Groups: ctxUser.GetGroups(),
			UID:    ctxUser.GetUID(),
			Extra:  newExtra,
		}

	} else {
		userToCheck = &user.DefaultInfo{
			Name:   subjectAccessReview.User,
			Groups: subjectAccessReview.Groups.List(),
			Extra:  map[string][]string{},
		}
	}

	switch {
	case subjectAccessReview.Scopes == nil:
		// leave the scopes alone.  on a self-sar, this means "use incoming request", on regular-sar it means, "use no scope restrictions"
	case len(subjectAccessReview.Scopes) == 0:
		// this always means "use no scope restrictions", so delete them
		delete(userToCheck.Extra, authorizationapi.ScopesKey)

	case len(subjectAccessReview.Scopes) > 0:
		// this always means, "use these scope restrictions", so force the value
		userToCheck.Extra[authorizationapi.ScopesKey] = subjectAccessReview.Scopes
	}

	requestContext := kapi.WithNamespace(kapi.WithUser(ctx, userToCheck), subjectAccessReview.Action.Namespace)
	attributes := authorizer.ToDefaultAuthorizationAttributes(subjectAccessReview.Action)
	allowed, reason, err := r.authorizer.Authorize(requestContext, attributes)
	response := &authorizationapi.SubjectAccessReviewResponse{
		Namespace: subjectAccessReview.Action.Namespace,
		Allowed:   allowed,
		Reason:    reason,
	}
	if err != nil {
		response.EvaluationError = err.Error()
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
		return kapierrors.NewForbidden(authorizationapi.Resource(localSARAttributes.GetResource()), localSARAttributes.GetResourceName(), err)
	}
	if !allowed {
		forbiddenError := kapierrors.NewForbidden(authorizationapi.Resource(localSARAttributes.GetResource()), localSARAttributes.GetResourceName(), errors.New("") /*discarded*/)
		forbiddenError.ErrStatus.Message = reason
		return forbiddenError
	}

	return nil
}
