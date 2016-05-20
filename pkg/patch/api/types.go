package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// Patch describes a patch you want to apply by POSTing it
type Patch struct {
	unversioned.TypeMeta

	// Spec describes the content of a patch and where to apply it
	Spec PatchSpec
	// Status gives the result of the patch operation
	Status PatchStatus
}

//  PatchSpec describe the content of a patch and where to apply it
type PatchSpec struct {
	// TargetGroup gives the API group of the resource you want to patch
	TargetGroup string
	// TargetVersion gives the API version of the resource you want to patch
	TargetVersion string
	// TargetResource gives the resource you want to patch
	TargetResource string
	// TargetNamespace gives the namespace of the resource you want to patch
	TargetNamespace string
	// TargetName gives the name of the resource you want to patch
	TargetName string

	// Type is the type of patch you're going to perform
	Type PatchType
	// Patch is the actual content of the patch you're applying
	Patch string
}

// PatchStatus gives the result of the patch operation
type PatchStatus struct {
	// Result is the API object bytes that were returned from the patch call
	Result runtime.Object
	// Error is any err you got back
	Error string
}

type PatchType string

var (
	StrategicMergePatch PatchType = "strategic"
)
