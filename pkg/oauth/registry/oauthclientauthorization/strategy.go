package oauthclientauthorization

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	scopeauthorizer "github.com/openshift/origin/pkg/authorization/authorizer/scope"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	"github.com/openshift/origin/pkg/oauth/apis/oauth/validation"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
)

// strategy implements behavior for OAuthClientAuthorization objects
type strategy struct {
	runtime.ObjectTyper

	clientGetter oauthclient.Getter
}

func NewStrategy(clientGetter oauthclient.Getter) strategy {
	return strategy{ObjectTyper: legacyscheme.Scheme, clientGetter: clientGetter}
}

var _ rest.GarbageCollectionDeleteStrategy = strategy{}

func (strategy) DefaultGarbageCollectionPolicy(ctx apirequest.Context) rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

func (strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	auth := obj.(*oauthapi.OAuthClientAuthorization)
	auth.Name = fmt.Sprintf("%s:%s", auth.UserName, auth.ClientName)
}

// NamespaceScoped is false for OAuth objects
func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

func (strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	auth := obj.(*oauthapi.OAuthClientAuthorization)
	auth.Name = fmt.Sprintf("%s:%s", auth.UserName, auth.ClientName)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new client
func (s strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	auth := obj.(*oauthapi.OAuthClientAuthorization)
	validationErrors := validation.ValidateClientAuthorization(auth)

	client, err := s.clientGetter.Get(auth.ClientName, metav1.GetOptions{})
	if err != nil {
		return append(validationErrors, field.InternalError(field.NewPath("clientName"), err))
	}
	if err := scopeauthorizer.ValidateScopeRestrictions(client, auth.Scopes...); err != nil {
		return append(validationErrors, field.InternalError(field.NewPath("clientName"), err))
	}

	return validationErrors
}

// ValidateUpdate validates a client auth update
func (s strategy) ValidateUpdate(ctx apirequest.Context, obj runtime.Object, old runtime.Object) field.ErrorList {
	clientAuth := obj.(*oauthapi.OAuthClientAuthorization)
	oldClientAuth := old.(*oauthapi.OAuthClientAuthorization)
	validationErrors := validation.ValidateClientAuthorizationUpdate(clientAuth, oldClientAuth)

	// only do a live client check if the scopes were increased by the update
	if containsNewScopes(clientAuth.Scopes, oldClientAuth.Scopes) {
		client, err := s.clientGetter.Get(clientAuth.ClientName, metav1.GetOptions{})
		if err != nil {
			return append(validationErrors, field.InternalError(field.NewPath("clientName"), err))
		}
		if err := scopeauthorizer.ValidateScopeRestrictions(client, clientAuth.Scopes...); err != nil {
			return append(validationErrors, field.InternalError(field.NewPath("clientName"), err))
		}
	}

	return validationErrors
}

func containsNewScopes(obj []string, old []string) bool {
	// an empty slice of scopes means all scopes, so we consider that a new scope
	newHasAllScopes := len(obj) == 0
	oldHasAllScopes := len(old) == 0
	if newHasAllScopes && !oldHasAllScopes {
		return true
	}

	newScopes := sets.NewString(obj...)
	oldScopes := sets.NewString(old...)
	return len(newScopes.Difference(oldScopes)) > 0
}

func (strategy) AllowCreateOnUpdate() bool {
	return true
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

// ClientAuthorizationName creates the computed name for a given username, clientName tuple
// This cannot change or client authorizations will be have unpredictably
func ClientAuthorizationName(userName, clientName string) string {
	return userName + ":" + clientName
}
