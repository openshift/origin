package oauthclientauthorization

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/api/validation"
)

// strategy implements behavior for OAuthClientAuthorization objects
type strategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating or updating OAuthClientAuthorization objects
// objects via the REST API.
var Strategy = strategy{kapi.Scheme}

func (strategy) PrepareForUpdate(obj, old runtime.Object) {
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

func (strategy) PrepareForCreate(obj runtime.Object) {
	auth := obj.(*api.OAuthClientAuthorization)
	auth.Name = fmt.Sprintf("%s:%s", auth.UserName, auth.ClientName)
}

// Validate validates a new client
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	auth := obj.(*api.OAuthClientAuthorization)
	return validation.ValidateClientAuthorization(auth)
}

// ValidateUpdate validates a client auth update
func (strategy) ValidateUpdate(ctx kapi.Context, obj runtime.Object, old runtime.Object) fielderrors.ValidationErrorList {
	clientAuth := obj.(*api.OAuthClientAuthorization)
	oldClientAuth := old.(*api.OAuthClientAuthorization)
	return validation.ValidateClientAuthorizationUpdate(clientAuth, oldClientAuth)
}

func (strategy) AllowCreateOnUpdate() bool {
	return true
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

// Matchtoken returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		clientObj, ok := obj.(*api.OAuthClientAuthorization)
		if !ok {
			return false, fmt.Errorf("not a client authorization")
		}
		fields := SelectableFields(clientObj)
		return label.Matches(labels.Set(clientObj.Labels)) && field.Matches(fields), nil
	})
}

// SelectableFields returns a label set that represents the object
func SelectableFields(obj *api.OAuthClientAuthorization) labels.Set {
	return labels.Set{
		"name":       obj.Name,
		"clientName": obj.ClientName,
		"userName":   obj.UserName,
		"userUID":    obj.UserUID,
	}
}
