package oauthclient

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

// strategy implements behavior for OAuthClient objects
type strategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating or updating OAuthClient objects
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

// Validate validates a new client
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	token := obj.(*api.OAuthClient)
	return validation.ValidateClient(token)
}

// ValidateUpdate validates a client update
func (strategy) ValidateUpdate(ctx kapi.Context, obj runtime.Object, old runtime.Object) fielderrors.ValidationErrorList {
	client := obj.(*api.OAuthClient)
	oldClient := old.(*api.OAuthClient)
	return validation.ValidateClientUpdate(client, oldClient)
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
		clientObj, ok := obj.(*api.OAuthClient)
		if !ok {
			return false, fmt.Errorf("not a client")
		}
		fields := SelectableFields(clientObj)
		return label.Matches(labels.Set(clientObj.Labels)) && field.Matches(fields), nil
	})
}

// SelectableFields returns a label set that represents the object
func SelectableFields(obj *api.OAuthClient) labels.Set {
	return labels.Set{
		"name": obj.Name,
	}
}
