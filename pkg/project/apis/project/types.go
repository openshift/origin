package project

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
)

// ProjectList is a list of Project objects.
type ProjectList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Project
}

const (
	// These are internal finalizer values to Origin
	FinalizerOrigin kapi.FinalizerName = "openshift.io/origin"
)

// ProjectSpec describes the attributes on a Project
type ProjectSpec struct {
	// Finalizers is an opaque list of values that must be empty to permanently remove object from storage
	Finalizers []kapi.FinalizerName
}

// ProjectStatus is information about the current status of a Project
type ProjectStatus struct {
	Phase kapi.NamespacePhase
}

// +genclient
// +genclient:nonNamespaced

// Project is a logical top-level container for a set of origin resources
type Project struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   ProjectSpec
	Status ProjectStatus
}

type ProjectRequest struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	DisplayName string
	Description string
}

// +genclient
// +genclient:nonNamespaced

// ProjectReservation prevents the creation of a project via the ProjectRequest endpoint.
// The name matches the namespace name that is reserved.
type ProjectReservation struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec ProjectReservationSpec
}

// ProjectReservationSpec provides metadata about the reservation
type ProjectReservationSpec struct {
	// ReservedFor is the username this name is reserved for.  It is optional and not forced
	// to any particular value.
	ReservedFor string
	// Reason is a human readable indication of why the project is reserved.
	Reason string
}

// ProjectReservationList is a list of ProjectReservation objects.
type ProjectReservationList struct {
	metav1.TypeMeta
	// Standard object's metadata.
	metav1.ListMeta
	// Items is the list of projectreservations
	Items []ProjectReservation
}

// These constants represent annotations keys affixed to projects
const (
	// ProjectNodeSelector is an annotation that holds the node selector;
	// the node selector annotation determines which nodes will have pods from this project scheduled to them
	ProjectNodeSelector = "openshift.io/node-selector"
	// ProjectRequester is the username that requested a given project.  Its not guaranteed to be present,
	// but it is set by the default project template.
	ProjectRequester = "openshift.io/requester"
)
