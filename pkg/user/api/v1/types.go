package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

// Auth system gets identity name and provider
// POST to UserIdentityMapping, get back error or a filled out UserIdentityMapping object

// +genclient=true

// User describes someone that makes requests to the API
type User struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// FullName is the full name of user
	FullName string `json:"fullName,omitempty" protobuf:"bytes,2,opt,name=fullName"`

	// Identities are the identities associated with this user
	Identities []string `json:"identities" protobuf:"bytes,3,rep,name=identities"`

	// Groups are the groups that this user is a member of
	Groups []string `json:"groups" protobuf:"bytes,4,rep,name=groups"`
}

// UserList is a collection of Users
type UserList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of users
	Items []User `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// Identity records a successful authentication of a user with an identity provider
type Identity struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// ProviderName is the source of identity information
	ProviderName string `json:"providerName" protobuf:"bytes,2,opt,name=providerName"`

	// ProviderUserName uniquely represents this identity in the scope of the provider
	ProviderUserName string `json:"providerUserName" protobuf:"bytes,3,opt,name=providerUserName"`

	// User is a reference to the user this identity is associated with
	// Both Name and UID must be set
	User kapi.ObjectReference `json:"user" protobuf:"bytes,4,opt,name=user"`

	// Extra holds extra information about this identity
	Extra map[string]string `json:"extra,omitempty" protobuf:"bytes,5,rep,name=extra"`
}

// IdentityList is a collection of Identities
type IdentityList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of identities
	Items []Identity `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// UserIdentityMapping maps a user to an identity
type UserIdentityMapping struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Identity is a reference to an identity
	Identity kapi.ObjectReference `json:"identity,omitempty" protobuf:"bytes,2,opt,name=identity"`
	// User is a reference to a user
	User kapi.ObjectReference `json:"user,omitempty" protobuf:"bytes,3,opt,name=user"`
}

// OptionalNames is an array that may also be left nil to distinguish between set and unset.
// +protobuf.nullable=true
type OptionalNames []string

// Group represents a referenceable set of Users
type Group struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Users is the list of users in this group.
	Users OptionalNames `json:"users" protobuf:"bytes,2,rep,name=users"`
}

// GroupList is a collection of Groups
type GroupList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of groups
	Items []Group `json:"items" protobuf:"bytes,2,rep,name=items"`
}
