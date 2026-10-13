/*
Copyright 2023.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FRRNodeStateSpec defines the desired state of FRRNodeState.
type FRRNodeStateSpec struct {
}

// FRRNodeStateStatus defines the observed state of FRRNodeState.
type FRRNodeStateStatus struct {
	// RunningConfig represents the current FRR running config, which is the configuration the FRR instance is currently running with.
	RunningConfig string `json:"runningConfig,omitempty"`
	// LastConversionResult is the status of the last translation between the `FRRConfiguration`s resources and FRR's configuration, contains "success" or an error.
	LastConversionResult string `json:"lastConversionResult,omitempty"`
	// LastReloadResult represents the status of the last configuration update operation by FRR, contains "success" or an error.
	LastReloadResult string `json:"lastReloadResult,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// FRRNodeState exposes the status of the FRR instance running on each node.
type FRRNodeState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FRRNodeStateSpec   `json:"spec,omitempty"`
	Status FRRNodeStateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FRRNodeStateList contains a list of FRRNodeStatus.
type FRRNodeStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FRRNodeState `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FRRNodeState{}, &FRRNodeStateList{})
}
