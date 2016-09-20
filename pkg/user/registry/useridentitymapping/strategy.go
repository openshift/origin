package useridentitymapping

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/api/validation"
)

// userIdentityMappingStrategy implements behavior for image repository mappings.
type userIdentityMappingStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating UserIdentityMapping
// objects via the REST API.
var Strategy = userIdentityMappingStrategy{kapi.Scheme}

// NamespaceScoped is true for image repository mappings.
func (s userIdentityMappingStrategy) NamespaceScoped() bool {
	return false
}

func (userIdentityMappingStrategy) GenerateName(base string) string {
	return base
}

func (userIdentityMappingStrategy) AllowCreateOnUpdate() bool {
	return true
}

func (userIdentityMappingStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s userIdentityMappingStrategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	mapping := obj.(*api.UserIdentityMapping)

	if len(mapping.Name) == 0 {
		mapping.Name = mapping.Identity.Name
	}
	mapping.Namespace = ""
	mapping.ResourceVersion = ""

	mapping.Identity.Namespace = ""
	mapping.Identity.Kind = ""
	mapping.Identity.UID = ""

	mapping.User.Namespace = ""
	mapping.User.Kind = ""
	mapping.User.UID = ""
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update
func (s userIdentityMappingStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	mapping := obj.(*api.UserIdentityMapping)

	if len(mapping.Name) == 0 {
		mapping.Name = mapping.Identity.Name
	}
	mapping.Namespace = ""

	mapping.Identity.Namespace = ""
	mapping.Identity.Kind = ""
	mapping.Identity.UID = ""

	mapping.User.Namespace = ""
	mapping.User.Kind = ""
	mapping.User.UID = ""
}

// Canonicalize normalizes the object after validation.
func (s userIdentityMappingStrategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new UserIdentityMapping.
func (s userIdentityMappingStrategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateUserIdentityMapping(obj.(*api.UserIdentityMapping))
}

// Validate validates an updated UserIdentityMapping.
func (s userIdentityMappingStrategy) ValidateUpdate(ctx kapi.Context, obj runtime.Object, old runtime.Object) field.ErrorList {
	return validation.ValidateUserIdentityMappingUpdate(obj.(*api.UserIdentityMapping), old.(*api.UserIdentityMapping))
}
