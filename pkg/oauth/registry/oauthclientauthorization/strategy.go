package oauthclientauthorization

import (
	"fmt"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/api/validation"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	scopeauthorizer "github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
)

// strategy implements behavior for OAuthClientAuthorization objects
type strategy struct {
	runtime.ObjectTyper

	clientGetter oauthclient.Getter
}

func NewStrategy(clientGetter oauthclient.Getter) strategy {
	return strategy{ObjectTyper: kapi.Scheme, clientGetter: clientGetter}
}

func (strategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	auth := obj.(*api.OAuthClientAuthorization)
	auth.Name = fmt.Sprintf("%s:%s", auth.UserName, auth.ClientName)
}

// NamespaceScoped is false for OAuth objects
func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

func (strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	auth := obj.(*api.OAuthClientAuthorization)
	auth.Name = fmt.Sprintf("%s:%s", auth.UserName, auth.ClientName)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new client
func (s strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	auth := obj.(*api.OAuthClientAuthorization)
	validationErrors := validation.ValidateClientAuthorization(auth)

	client, err := s.clientGetter.GetClient(ctx, auth.ClientName)
	if err != nil {
		return append(validationErrors, field.InternalError(field.NewPath("clientName"), err))
	}
	if err := scopeauthorizer.ValidateScopeRestrictions(client, auth.Scopes...); err != nil {
		return append(validationErrors, field.InternalError(field.NewPath("clientName"), err))
	}

	return validationErrors
}

// ValidateUpdate validates a client auth update
func (s strategy) ValidateUpdate(ctx kapi.Context, obj runtime.Object, old runtime.Object) field.ErrorList {
	clientAuth := obj.(*api.OAuthClientAuthorization)
	oldClientAuth := old.(*api.OAuthClientAuthorization)
	validationErrors := validation.ValidateClientAuthorizationUpdate(clientAuth, oldClientAuth)

	client, err := s.clientGetter.GetClient(ctx, clientAuth.ClientName)
	if err != nil {
		return append(validationErrors, field.InternalError(field.NewPath("clientName"), err))
	}
	if err := scopeauthorizer.ValidateScopeRestrictions(client, clientAuth.Scopes...); err != nil {
		return append(validationErrors, field.InternalError(field.NewPath("clientName"), err))
	}

	return validationErrors
}

func (strategy) AllowCreateOnUpdate() bool {
	return true
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(o runtime.Object) (labels.Set, fields.Set, error) {
			obj, ok := o.(*api.OAuthClientAuthorization)
			if !ok {
				return nil, nil, fmt.Errorf("not a OAuthClientAuthorization")
			}
			return labels.Set(obj.Labels), SelectableFields(obj), nil
		},
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *api.OAuthClientAuthorization) fields.Set {
	return api.OAuthClientAuthorizationToSelectableFields(obj)
}
