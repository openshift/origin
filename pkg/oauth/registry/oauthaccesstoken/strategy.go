package oauthaccesstoken

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kstorage "k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	scopeauthorizer "github.com/openshift/origin/pkg/authorization/authorizer/scope"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	"github.com/openshift/origin/pkg/oauth/apis/oauth/validation"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
)

// strategy implements behavior for OAuthAccessTokens
type strategy struct {
	runtime.ObjectTyper

	clientGetter oauthclient.Getter
}

var _ rest.RESTCreateStrategy = strategy{}
var _ rest.RESTUpdateStrategy = strategy{}

func NewStrategy(clientGetter oauthclient.Getter) strategy {
	return strategy{ObjectTyper: kapi.Scheme, clientGetter: clientGetter}
}

func (strategy) DefaultGarbageCollectionPolicy() rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

func (strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {}

// NamespaceScoped is false for OAuth objects
func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

func (strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
}

// Validate validates a new token
func (s strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	token := obj.(*oauthapi.OAuthAccessToken)
	validationErrors := validation.ValidateAccessToken(token)

	client, err := s.clientGetter.GetClient(ctx, token.ClientName, &metav1.GetOptions{})
	if err != nil {
		return append(validationErrors, field.InternalError(field.NewPath("clientName"), err))
	}
	if err := scopeauthorizer.ValidateScopeRestrictions(client, token.Scopes...); err != nil {
		return append(validationErrors, field.InternalError(field.NewPath("clientName"), err))
	}

	return validationErrors
}

// ValidateUpdate validates an update
func (s strategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	oldToken := old.(*oauthapi.OAuthAccessToken)
	newToken := obj.(*oauthapi.OAuthAccessToken)
	return validation.ValidateAccessTokenUpdate(newToken, oldToken)
}

// AllowCreateOnUpdate is false for OAuth objects
func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(o runtime.Object) (labels.Set, fields.Set, bool, error) {
	obj, ok := o.(*oauthapi.OAuthAccessToken)
	if !ok {
		return nil, nil, false, fmt.Errorf("not an OAuthAccessToken")
	}
	return labels.Set(obj.Labels), SelectableFields(obj), obj.Initializers != nil, nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) kstorage.SelectionPredicate {
	return kstorage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *oauthapi.OAuthAccessToken) fields.Set {
	return oauthapi.OAuthAccessTokenToSelectableFields(obj)
}
