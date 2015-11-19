package oauthauthorizetoken

import (
	"fmt"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/api/validation"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"
)

// strategy implements behavior for OAuthAuthorizeTokens
type strategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating OAuthAuthorizeToken
// objects via the REST API.
var Strategy = strategy{kapi.Scheme}

func (strategy) PrepareForUpdate(obj, old runtime.Object) {}

// NamespaceScoped is false for OAuth objects
func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

func (strategy) PrepareForCreate(obj runtime.Object) {
}

// Validate validates a new token
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	token := obj.(*api.OAuthAuthorizeToken)
	return validation.ValidateAuthorizeToken(token)
}

// AllowCreateOnUpdate is false for OAuth objects
func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

// Matchtoken returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		tokenObj, ok := obj.(*api.OAuthAuthorizeToken)
		if !ok {
			return false, fmt.Errorf("not a token")
		}
		fields := api.OAuthAuthorizeTokenToSelectableFields(tokenObj)
		return label.Matches(labels.Set(tokenObj.Labels)) && field.Matches(fields), nil
	})
}
