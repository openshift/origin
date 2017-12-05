package subjectaccessreview

import (
	"errors"
	"fmt"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationvalidation "github.com/openshift/origin/pkg/authorization/apis/authorization/validation"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/registry/util"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	authorizer kauthorizer.Authorizer
}

var _ rest.Creater = &REST{}

// NewREST creates a new REST for policies.
func NewREST(authorizer kauthorizer.Authorizer) *REST {
	return &REST{authorizer}
}

// New creates a new ResourceAccessReview object
func (r *REST) New() runtime.Object {
	return &authorizationapi.SubjectAccessReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx apirequest.Context, obj runtime.Object, _ rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
	subjectAccessReview, ok := obj.(*authorizationapi.SubjectAccessReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a subjectAccessReview: %#v", obj))
	}
	if errs := authorizationvalidation.ValidateSubjectAccessReview(subjectAccessReview); len(errs) > 0 {
		return nil, kapierrors.NewInvalid(authorizationapi.Kind(subjectAccessReview.Kind), "", errs)
	}

	requestingUser, ok := apirequest.UserFrom(ctx)
	if !ok {
		return nil, kapierrors.NewInternalError(errors.New("missing user on request"))
	}

	// if a namespace is present on the request, then the namespace on the on the SAR is overwritten.
	// This is to support backwards compatibility.  To have gotten here in this state, it means that
	// the authorizer decided that a user could run an SAR against this namespace
	if namespace := apirequest.NamespaceValue(ctx); len(namespace) > 0 {
		subjectAccessReview.Action.Namespace = namespace

	} else if err := r.isAllowed(requestingUser, subjectAccessReview); err != nil {
		// this check is mutually exclusive to the condition above.  localSAR and localRAR both clear the namespace before delegating their calls
		// We only need to check if the SAR is allowed **again** if the authorizer didn't already approve the request for a legacy call.
		return nil, err
	}

	var userToCheck *user.DefaultInfo
	if (len(subjectAccessReview.User) == 0) && (len(subjectAccessReview.Groups) == 0) {
		// if no user or group was specified, use the info from the context
		ctxUser, exists := apirequest.UserFrom(ctx)
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
			newSlice := make([]string, len(v))
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

	attributes := util.ToDefaultAuthorizationAttributes(userToCheck, subjectAccessReview.Action.Namespace, subjectAccessReview.Action)
	authorized, reason, err := r.authorizer.Authorize(attributes)
	response := &authorizationapi.SubjectAccessReviewResponse{
		Namespace: subjectAccessReview.Action.Namespace,
		Allowed:   authorized == kauthorizer.DecisionAllow,
		Reason:    reason,
	}
	if err != nil {
		response.EvaluationError = err.Error()
	}

	return response, nil
}

// isAllowed checks to see if the current user has rights to issue a LocalSubjectAccessReview on the namespace they're attempting to access
func (r *REST) isAllowed(user user.Info, sar *authorizationapi.SubjectAccessReview) error {
	var localSARAttributes kauthorizer.AttributesRecord
	// if they are running a personalSAR, create synthentic check for selfSAR
	if authorizer.IsPersonalAccessReviewFromSAR(sar) {
		localSARAttributes = kauthorizer.AttributesRecord{
			User:            user,
			Verb:            "create",
			Namespace:       sar.Action.Namespace,
			APIGroup:        "authorization.k8s.io",
			Resource:        "selfsubjectaccessreviews",
			ResourceRequest: true,
		}
	} else {
		localSARAttributes = kauthorizer.AttributesRecord{
			User:            user,
			Verb:            "create",
			Namespace:       sar.Action.Namespace,
			Resource:        "localsubjectaccessreviews",
			ResourceRequest: true,
		}
	}

	authorized, reason, err := r.authorizer.Authorize(localSARAttributes)

	if err != nil {
		return kapierrors.NewForbidden(authorizationapi.Resource(localSARAttributes.GetResource()), localSARAttributes.GetName(), err)
	}
	if authorized != kauthorizer.DecisionAllow {
		forbiddenError := kapierrors.NewForbidden(authorizationapi.Resource(localSARAttributes.GetResource()), localSARAttributes.GetName(), errors.New("") /*discarded*/)
		forbiddenError.ErrStatus.Message = reason
		return forbiddenError
	}

	return nil
}
