package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// Auth system gets identity name and provider
// POST to UserIdentityMapping, get back error or a filled out UserIdentityMapping object

type User struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	FullName string `json:"fullName,omitempty" yaml:"fullName,omitempty"`
}

type UserList struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []User `json:"items,omitempty" yaml:"items,omitempty"`
}

type Identity struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Provider is the source of identity information - if empty, the default provider
	// is assumed.
	Provider string `json:"provider" yaml:"provider"`

	Extra map[string]string `json:"extra,omitempty" yaml:"extra,omitempty"`
}

type UserIdentityMapping struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Identity Identity `json:"identity,omitempty" yaml:"identity,omitempty"`
	User     User     `json:"user,omitempty" yaml:"user,omitempty"`
}

func (*User) IsAnAPIObject()                {}
func (*UserList) IsAnAPIObject()            {}
func (*Identity) IsAnAPIObject()            {}
func (*UserIdentityMapping) IsAnAPIObject() {}
