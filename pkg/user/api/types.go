package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// Auth system gets identity name and provider
// POST to UserIdentityMapping, get back error or a filled out UserIdentityMapping object

type User struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	FullName string `json:"fullName,omitempty"`
}

type UserList struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

type Identity struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Provider is the source of identity information - if empty, the default provider
	// is assumed.
	Provider string `json:"provider"`

	// UserName uniquely represents this identity in the scope of the identity provider
	UserName string `json:"userName"`

	Extra map[string]string `json:"extra,omitempty"`
}

type UserIdentityMapping struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	Identity Identity `json:"identity,omitempty"`
	User     User     `json:"user,omitempty"`
}

func (*User) IsAnAPIObject()                {}
func (*UserList) IsAnAPIObject()            {}
func (*Identity) IsAnAPIObject()            {}
func (*UserIdentityMapping) IsAnAPIObject() {}
