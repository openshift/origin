/*
Copyright 2022.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Memcached44553Spec defines the desired state of Memcached44553
type Memcached44553Spec struct {
	// +kubebuilder:validation:Minimum=0
	// Size is the size of the memcached deployment
	Size int32 `json:"size"`
}

// Memcached44553Status defines the observed state of Memcached44553
type Memcached44553Status struct {
	// Nodes are the names of the memcached pods
	Nodes []string `json:"nodes"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Memcached44553 is the Schema for the memcached44553s API
type Memcached44553 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Memcached44553Spec   `json:"spec,omitempty"`
	Status Memcached44553Status `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// Memcached44553List contains a list of Memcached44553
type Memcached44553List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Memcached44553 `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Memcached44553{}, &Memcached44553List{})
}
