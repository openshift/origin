package identity

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

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

func (identityStrategy) PrepareForUpdate(obj, old runtime.Object) {}

// NamespaceScoped is false for users
func (identityStrategy) NamespaceScoped() bool {
	return false
}

func (identityStrategy) GenerateName(base string) string {
	return base
}

func (identityStrategy) PrepareForCreate(obj runtime.Object) {
	identity := obj.(*api.Identity)
	identity.Name = identityName(identity.ProviderName, identity.ProviderUserName)
}

// Validate validates a new user
func (identityStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	identity := obj.(*api.Identity)
	return validation.ValidateIdentity(identity)
}

// AllowCreateOnUpdate is false for identity
func (identityStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (identityStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an identity
func (identityStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
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
