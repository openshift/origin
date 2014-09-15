package api

import (
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// Auth system gets identity name and provider
// POST to UserIdentityMapping, get back error or a filled out UserIdentityMapping object

type User struct {
	kubeapi.JSONBase `json:",inline" yaml:",inline"`
	Labels           map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// Name is a human readable string uniquely representing this user at any time.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// UID is a unique in time and space value which uniquely identifies this user.
	UID string `json:"uid,omitempty" yaml:"uid,omitempty"`

	FullName string `json:"fullName,omitempty" yaml:"fullName,omitempty"`
}

type UserList struct {
	kubeapi.JSONBase `json:",inline" yaml:",inline"`
	Items            []User `json:"items,omitempty" yaml:"items,omitempty"`
}

type Identity struct {
	kubeapi.JSONBase `json:",inline" yaml:",inline"`
	Labels           map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// Name is the unique identifier of a user within a given provider
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Provider is the source of identity information - if empty, the default provider
	// is assumed.
	Provider string `json:"provider" yaml:"provider"`

	Extra map[string]string `json:"extra,omitempty" yaml:"extra,omitempty"`
}

type UserIdentityMapping struct {
	kubeapi.JSONBase `json:",inline" yaml:",inline"`

	Identity Identity `json:"identity,omitempty" yaml:"identity,omitempty"`
	User     User     `json:"user,omitempty" yaml:"user,omitempty"`
}

func (*User) IsAnAPIObject()                {}
func (*UserList) IsAnAPIObject()            {}
func (*Identity) IsAnAPIObject()            {}
func (*UserIdentityMapping) IsAnAPIObject() {}
