package project

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Project is a logical top-level container for a set of origin resources
type Project struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   ProjectSpec
	Status ProjectStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced
// +genclient:skipVerbs=get,list,create,update,patch,delete,deleteCollection,watch
// +genclient:method=Create,verb=create,result=Project

type ProjectRequest struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	DisplayName string
	Description string
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
