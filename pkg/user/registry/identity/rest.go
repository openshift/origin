package identity

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/api/validation"
)

// identityStrategy implements behavior for Identities
type identityStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating Identity
// objects via the REST API.
var Strategy = identityStrategy{kapi.Scheme}

// NamespaceScoped is false for users
func (identityStrategy) NamespaceScoped() bool {
	return false
}

func (identityStrategy) GenerateName(base string) string {
	return base
}

func (identityStrategy) ResetBeforeCreate(obj runtime.Object) {
	identity := obj.(*api.Identity)
	identity.Name = IdentityName(identity.ProviderName, identity.ProviderUserName)
}

// Validate validates a new user
func (identityStrategy) Validate(obj runtime.Object) kerrs.ValidationErrorList {
	identity := obj.(*api.Identity)
	return validation.ValidateIdentity(identity)
}

// AllowCreateOnUpdate is false for identity
func (identityStrategy) AllowCreateOnUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an identity
func (identityStrategy) ValidateUpdate(obj, old runtime.Object) kerrs.ValidationErrorList {
	return validation.ValidateIdentityUpdate(obj.(*api.Identity), old.(*api.Identity))
}

// MatchIdentity returns a generic matcher for a given label and field selector.
func MatchIdentity(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		identityObj, ok := obj.(*api.Identity)
		if !ok {
			return false, fmt.Errorf("not an identity")
		}
		fields := IdentityToSelectableFields(identityObj)
		return label.Matches(labels.Set(identityObj.Labels)) && field.Matches(fields), nil
	})
}

// IdentityToSelectableFields returns a label set that represents the object
func IdentityToSelectableFields(identity *api.Identity) labels.Set {
	return labels.Set{
		"name":             identity.Name,
		"providerName":     identity.ProviderName,
		"providerUserName": identity.ProviderName,
		"user.name":        identity.User.Name,
		"user.uid":         string(identity.User.UID),
	}
}
