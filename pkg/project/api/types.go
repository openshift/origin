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

// ProjectSpec describes the attributes on a Project
type ProjectSpec struct {
}

// ProjectStatus is information about the current status of a Project
type ProjectStatus struct {
	Phase kapi.NamespacePhase
}

// Project is a logical top-level container for a set of origin resources
type Project struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// TODO: remove me
	DisplayName string
	Spec        ProjectSpec
	Status      ProjectStatus
}
