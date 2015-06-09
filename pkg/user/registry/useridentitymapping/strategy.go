package useridentitymapping

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

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

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s userIdentityMappingStrategy) PrepareForCreate(obj runtime.Object) {
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
func (s userIdentityMappingStrategy) PrepareForUpdate(obj, old runtime.Object) {
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

// Validate validates a new UserIdentityMapping.
func (s userIdentityMappingStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateUserIdentityMapping(obj.(*api.UserIdentityMapping))
}

// Validate validates an updated UserIdentityMapping.
func (s userIdentityMappingStrategy) ValidateUpdate(ctx kapi.Context, obj runtime.Object, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateUserIdentityMappingUpdate(obj.(*api.UserIdentityMapping), old.(*api.UserIdentityMapping))
}
