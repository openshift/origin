/*
Copyright 2017 The Kubernetes Authors.

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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodPreset is a policy resource that defines additional runtime
// requirements for a Pod.
type PodPreset struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec PodPresetSpec `json:"spec,omitempty"`
}

// PodPresetSpec is a description of a pod preset.
type PodPresetSpec struct {
	// Selector is a label query over a set of resources, in this case pods.
	// Required.
	Selector metav1.LabelSelector `json:"selector"`

	// Env defines the collection of EnvVar to inject into containers.
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`

	// EnvFrom defines the collection of EnvFromSource to inject into containers.
	// +optional
	EnvFrom []v1.EnvFromSource `json:"envFrom,omitempty"`

	// Volumes defines the collection of Volume to inject into the pod.
	// +optional
	Volumes []v1.Volume `json:"volumes,omitempty"`

	// VolumeMounts defines the collection of VolumeMount to inject into containers.
	// +optional
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodPresetList is a list of PodPreset objects.
type PodPresetList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata, omitempty"`

	Items []PodPreset `json:"items"`
}
