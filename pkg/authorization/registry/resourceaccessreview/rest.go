package resourceaccessreview

import (
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
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
	return &authorizationapi.ResourceAccessReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	resourceAccessReview, ok := obj.(*authorizationapi.ResourceAccessReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a resourceAccessReview: %#v", obj))
	}
	if errs := authorizationvalidation.ValidateResourceAccessReview(resourceAccessReview); len(errs) > 0 {
		return nil, kapierrors.NewInvalid(authorizationapi.Kind(resourceAccessReview.Kind), "", errs)
	}
	// if a namespace is present on the request, then the namespace on the on the RAR is overwritten.
	// This is to support backwards compatibility.  To have gotten here in this state, it means that
	// the authorizer decided that a user could run an RAR against this namespace
	if namespace := kapi.NamespaceValue(ctx); len(namespace) > 0 {
		resourceAccessReview.Action.Namespace = namespace

	} else if err := r.isAllowed(ctx, resourceAccessReview); err != nil {
		// this check is mutually exclusive to the condition above.  localSAR and localRAR both clear the namespace before delegating their calls
		// We only need to check if the RAR is allowed **again** if the authorizer didn't already approve the request for a legacy call.
		return nil, err
	}

	requestContext := kapi.WithNamespace(ctx, resourceAccessReview.Action.Namespace)
	attributes := authorizer.ToDefaultAuthorizationAttributes(resourceAccessReview.Action)
	users, groups, _ := r.authorizer.GetAllowedSubjects(requestContext, attributes)

	response := &authorizationapi.ResourceAccessReviewResponse{
		Namespace: resourceAccessReview.Action.Namespace,
		Users:     users,
		Groups:    groups,
	}

	return response, nil
}

// isAllowed checks to see if the current user has rights to issue a LocalSubjectAccessReview on the namespace they're attempting to access
func (r *REST) isAllowed(ctx kapi.Context, rar *authorizationapi.ResourceAccessReview) error {
	localRARAttributes := authorizer.DefaultAuthorizationAttributes{
		Verb:     "create",
		Resource: "localresourceaccessreviews",
	}
	allowed, reason, err := r.authorizer.Authorize(kapi.WithNamespace(ctx, rar.Action.Namespace), localRARAttributes)

	if err != nil {
		return kapierrors.NewForbidden(authorizationapi.Resource(localRARAttributes.GetResource()), localRARAttributes.GetResourceName(), err)
	}
	if !allowed {
		forbiddenError, _ := kapierrors.NewForbidden(authorizationapi.Resource(localRARAttributes.GetResource()), localRARAttributes.GetResourceName(), errors.New("") /*discarded*/).(*kapierrors.StatusError)
		forbiddenError.ErrStatus.Message = reason
		return forbiddenError
	}

	return nil
}
