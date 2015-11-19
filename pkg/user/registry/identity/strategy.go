package identity

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"

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

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		identityObj, ok := obj.(*api.Identity)
		if !ok {
			return false, fmt.Errorf("not an identity")
		}
		fields := api.IdentityToSelectableFields(identityObj)
		return label.Matches(labels.Set(identityObj.Labels)) && field.Matches(fields), nil
	})
}
