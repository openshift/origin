package api

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/registry/generic"
)

// GroupToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func GroupToSelectableFields(group *Group) fields.Set {
	return generic.ObjectMetaFieldsSet(&group.ObjectMeta, false)
}

// IdentityToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func IdentityToSelectableFields(identity *Identity) fields.Set {
	return generic.AddObjectMetaFieldsSet(fields.Set{
		"providerName":     identity.ProviderName,
		"providerUserName": identity.ProviderName,
		"user.name":        identity.User.Name,
		"user.uid":         string(identity.User.UID),
	}, &identity.ObjectMeta, false)
}

// UserToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func UserToSelectableFields(user *User) fields.Set {
	return generic.ObjectMetaFieldsSet(&user.ObjectMeta, false)
}
