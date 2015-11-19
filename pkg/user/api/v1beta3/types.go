package v1beta3

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1beta3"
)

// Auth system gets identity name and provider
// POST to UserIdentityMapping, get back error or a filled out UserIdentityMapping object

type User struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	FullName string `json:"fullName,omitempty"`

	Identities []string `json:"identities"`

	Groups []string `json:"groups"`
}

type UserList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`
	Items                []User `json:"items"`
}

type Identity struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// ProviderName is the source of identity information
	ProviderName string `json:"providerName"`

	// ProviderUserName uniquely represents this identity in the scope of the provider
	ProviderUserName string `json:"providerUserName"`

	// User is a reference to the user this identity is associated with
	// Both Name and UID must be set
	User kapi.ObjectReference `json:"user"`

	Extra map[string]string `json:"extra,omitempty"`
}

type IdentityList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`
	Items                []Identity `json:"items"`
}

type UserIdentityMapping struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	Identity kapi.ObjectReference `json:"identity,omitempty"`
	User     kapi.ObjectReference `json:"user,omitempty"`
}

// Group represents a referenceable set of Users
type Group struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// Users is the list of users in this group.
	Users []string `json:"users" description:"list of users in this group"`
}

type GroupList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`
	Items                []Group `json:"items" description:"list of groups"`
}

func (*GroupList) IsAnAPIObject()           {}
func (*Group) IsAnAPIObject()               {}
func (*User) IsAnAPIObject()                {}
func (*UserList) IsAnAPIObject()            {}
func (*Identity) IsAnAPIObject()            {}
func (*IdentityList) IsAnAPIObject()        {}
func (*UserIdentityMapping) IsAnAPIObject() {}
