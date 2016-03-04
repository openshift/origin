package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

// Auth system gets identity name and provider
// POST to UserIdentityMapping, get back error or a filled out UserIdentityMapping object

// User describes someone that makes requests to the API
type User struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// FullName is the full name of user
	FullName string `json:"fullName,omitempty"`

	// Identities are the identities associated with this user
	Identities []string `json:"identities"`

	// Groups are the groups that this user is a member of
	Groups []string `json:"groups"`
}

// UserList is a collection of Users
type UserList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`
	// Items is the list of users
	Items []User `json:"items"`
}

// Identity records a successful authentication of a user with an identity provider
type Identity struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// ProviderName is the source of identity information
	ProviderName string `json:"providerName"`

	// ProviderUserName uniquely represents this identity in the scope of the provider
	ProviderUserName string `json:"providerUserName"`

	// User is a reference to the user this identity is associated with
	// Both Name and UID must be set
	User kapi.ObjectReference `json:"user"`

	// Extra holds extra information about this identity
	Extra map[string]string `json:"extra,omitempty"`
}

// IdentityList is a collection of Identities
type IdentityList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`
	// Items is the list of identities
	Items []Identity `json:"items"`
}

// UserIdentityMapping maps a user to an identity
type UserIdentityMapping struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Identity is a reference to an identity
	Identity kapi.ObjectReference `json:"identity,omitempty"`
	// User is a reference to a user
	User kapi.ObjectReference `json:"user,omitempty"`
}

// Group represents a referenceable set of Users
type Group struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Users is the list of users in this group.
	Users []string `json:"users"`
}

// GroupList is a collection of Groups
type GroupList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`
	// Items is the list of groups
	Items []Group `json:"items"`
}
