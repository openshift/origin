package api

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/registry/generic"
)

// OAuthAccessTokenToSelectableFields returns a label set that represents the object
func OAuthAccessTokenToSelectableFields(obj *OAuthAccessToken) fields.Set {
	return generic.AddObjectMetaFieldsSet(fields.Set{
		"clientName":     obj.ClientName,
		"userName":       obj.UserName,
		"userUID":        obj.UserUID,
		"authorizeToken": obj.AuthorizeToken,
	}, &obj.ObjectMeta, false)
}

// OAuthAuthorizeTokenToSelectableFields returns a label set that represents the object
func OAuthAuthorizeTokenToSelectableFields(obj *OAuthAuthorizeToken) fields.Set {
	return generic.AddObjectMetaFieldsSet(fields.Set{
		"clientName": obj.ClientName,
		"userName":   obj.UserName,
		"userUID":    obj.UserUID,
	}, &obj.ObjectMeta, false)
}

// OAuthClientToSelectableFields returns a label set that represents the object
func OAuthClientToSelectableFields(obj *OAuthClient) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

// OAuthClientAuthorizationToSelectableFields returns a label set that represents the object
func OAuthClientAuthorizationToSelectableFields(obj *OAuthClientAuthorization) fields.Set {
	return generic.AddObjectMetaFieldsSet(fields.Set{
		"clientName": obj.ClientName,
		"userName":   obj.UserName,
		"userUID":    obj.UserUID,
	}, &obj.ObjectMeta, false)
}
