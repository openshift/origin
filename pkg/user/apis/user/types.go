package user

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

// Auth system gets identity name and provider
// POST to UserIdentityMapping, get back error or a filled out UserIdentityMapping object

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type User struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	FullName string

	Identities []string

	Groups []string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type UserList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []User
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type IdentityList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Identity
}

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=get,create,update,delete
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type UserIdentityMapping struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Identity kapi.ObjectReference
	User     kapi.ObjectReference
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Group represents a referenceable set of Users
type Group struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Users []string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GroupList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Group
}
