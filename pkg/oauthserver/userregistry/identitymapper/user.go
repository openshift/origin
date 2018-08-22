package identitymapper

import (
	kuser "k8s.io/apiserver/pkg/authentication/user"

	userapi "github.com/openshift/api/user/v1"
)

// userToInfo converts an OpenShift user API object into a Kubernetes user.Info.
// The resulting user.Info should only contain information that is static for the
// given user API object - namely the name and UID fields.  Information that is not
// static such as embedded group memberships or group memberships based on OpenShift
// groups should not be included.  It is handled dynamically during token authentication.
// Group memberships from a specific IDP flow should also not be included in this
// user.Info as this user.Info does not have a way to directly encode what IDP granted
// the given groups (other than silly Extra field hacks).  A struct that layers this
// information on top of the user.Info should be used for IDP flow group memberships.
// Thus the Groups and Extra fields should be left unset.
func userToInfo(user *userapi.User) kuser.Info {
	return &kuser.DefaultInfo{
		Name: user.Name,
		UID:  string(user.UID),
	}
}
