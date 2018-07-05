/*
Copyright 2018 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!
// Created by "kubebuilder create resource" for you to implement the Idler resource schema definition
// as a go struct.
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NB: slice types *must* be omitempty, otherwise the CRD
// validator will complain that slices can't be null (since nil slices
// become null, not empty arrays).

// IdlerSpec defines the desired state of Idler
type IdlerSpec struct {
	// WantIdle represents the desired state of idling
	WantIdle bool `json:"wantIdle"`
	// TargetScalables contains the collection of scalables that
	// are idled/unidled together.
	TargetScalables []CrossGroupObjectReference `json:"targetScalables,omitempty"`
	// TriggerServiceNames contains the collection of services that shold
	// trigger unidling.  Their corresponding endpoints objects will be
	// used to determine whether or not unidling is successful.
	TriggerServiceNames []string `json:"triggerServiceNames,omitempty"`
}

// IdlerStatus defines the observed state of Idler
type IdlerStatus struct {
	// Idled represents the current state of idling
	Idled bool `json:"idled"`
	// UnidleScales contains the previous scales of idled scalables
	UnidledScales []UnidleInfo `json:"unidledScales,omitempty"`
	// InactiveServiceNames contains services in the process of
	// unidling that have not yet become active.
	InactiveServiceNames []string `json:"inactiveServiceNames,omitempty"`
}

// UnidleInfo represents the information needed to restore an idled object
// to its unidled state.
type UnidleInfo struct {
	CrossGroupObjectReference `json:",inline"`
	// PreviousScale represents the replica count of this object before it
	// was idled.
	PreviousScale int32 `json:"previousScale"`
}

// CrossGroupObjectReference references an object in the same namespace as
// the current "context", but potentially in a different API group.
type CrossGroupObjectReference struct {
	// TODO(directxman12): ask deads/liggitt if we're still
	// going to fight the Group vs APIVersion battle...

	// Group is the API group that the given resource belongs to.
	Group string `json:"group"`
	// Resource is the type of resource that this references.
	Resource string `json:"resource"`
	// Name is the name of the object that we're referencing.
	Name string `json:"name"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Idler
// +k8s:openapi-gen=true
// +resource:path=idlers
type Idler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IdlerSpec   `json:"spec,omitempty"`
	Status IdlerStatus `json:"status,omitempty"`
}
