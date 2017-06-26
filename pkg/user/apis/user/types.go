package user

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
)

// Auth system gets identity name and provider
// POST to UserIdentityMapping, get back error or a filled out UserIdentityMapping object

// +genclient=true

type User struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	FullName string

	Identities []string

	Groups []string
}

type UserList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []User
}

type Identity struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// ProviderName is the source of identity information
	ProviderName string

	// ProviderUserName uniquely represents this identity in the scope of the provider
	ProviderUserName string

	// User is a reference to the user this identity is associated with
	// Both Name and UID must be set
	User kapi.ObjectReference

	Extra map[string]string
}

type IdentityList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Identity
}

type UserIdentityMapping struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Identity kapi.ObjectReference
	User     kapi.ObjectReference
}

// Group represents a referenceable set of Users
type Group struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Users []string
}

type GroupList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Group
}
