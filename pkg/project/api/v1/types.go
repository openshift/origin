package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

// ProjectList is a list of Project objects.
type ProjectList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`
	// Items is the list of projects
	Items []Project `json:"items"`
}

const (
	// These are internal finalizer values to Origin
	FinalizerOrigin kapi.FinalizerName = "openshift.io/origin"
)

// ProjectSpec describes the attributes on a Project
type ProjectSpec struct {
	// Finalizers is an opaque list of values that must be empty to permanently remove object from storage
	Finalizers []kapi.FinalizerName `json:"finalizers,omitempty"`
}

// ProjectStatus is information about the current status of a Project
type ProjectStatus struct {
	// Phase is the current lifecycle phase of the project
	Phase kapi.NamespacePhase `json:"phase,omitempty"`
}

// Project is a logical top-level container for a set of origin resources
type Project struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of the Namespace.
	Spec ProjectSpec `json:"spec,omitempty"`

	// Status describes the current status of a Namespace
	Status ProjectStatus `json:"status,omitempty"`
}

// ProjecRequest is the set of options necessary to fully qualify a project request
type ProjectRequest struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`
	// DisplayName is the display name to apply to a project
	DisplayName string `json:"displayName,omitempty"`
	// Description is the description to apply to a project
	Description string `json:"description,omitempty"`
}
