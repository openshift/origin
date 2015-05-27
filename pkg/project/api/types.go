package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// ProjectList is a list of Project objects.
type ProjectList struct {
	kapi.TypeMeta
	kapi.ListMeta
	Items []Project
}

// These are internal finalizer values to Origin
const (
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

// Project is a logical top-level container for a set of origin resources
type Project struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	Spec   ProjectSpec
	Status ProjectStatus
}

type ProjectRequest struct {
	kapi.TypeMeta
	kapi.ObjectMeta
	DisplayName string
}
