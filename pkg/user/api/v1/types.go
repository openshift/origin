package v1

import kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"

// Auth system gets identity name and provider
// POST to UserIdentityMapping, get back error or a filled out UserIdentityMapping object

type User struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	FullName string `json:"fullName,omitempty" description:"full name of user"`

	Identities []string `json:"identities" description:"list of identities"`

	Groups []string `json:"groups" description:"list of groups"`
}

type UserList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []User `json:"items" description:"list of users"`
}

type Identity struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// ProviderName is the source of identity information
	ProviderName string `json:"providerName" description:"source of identity information"`

	// ProviderUserName uniquely represents this identity in the scope of the provider
	ProviderUserName string `json:"providerUserName" description:"uniquely represents this identity in the scope of the provider"`

	// User is a reference to the user this identity is associated with
	// Both Name and UID must be set
	User kapi.ObjectReference `json:"user" description:"reference to the user this identity is associated with.  both name and uid must be set"`

	Extra map[string]string `json:"extra,omitempty" description:"extra information for this identity"`
}

type IdentityList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []Identity `json:"items" description:"list of identities"`
}

type UserIdentityMapping struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	Identity kapi.ObjectReference `json:"identity,omitempty" description:"reference to an identity"`
	User     kapi.ObjectReference `json:"user,omitempty" description:"reference to a user"`
}

// Group represents a referenceable set of Users
type Group struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Users is the list of users in this group.
	Users []string `json:"users" description:"list of users in this group"`
}

type GroupList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []Group `json:"items" description:"list of groups"`
}

func (*GroupList) IsAnAPIObject()           {}
func (*Group) IsAnAPIObject()               {}
func (*User) IsAnAPIObject()                {}
func (*UserList) IsAnAPIObject()            {}
func (*Identity) IsAnAPIObject()            {}
func (*IdentityList) IsAnAPIObject()        {}
func (*UserIdentityMapping) IsAnAPIObject() {}
