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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Memcached44295Spec defines the desired state of Memcached44295
type Memcached44295Spec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Size defines the number of Memcached instances
	Size int32 `json:"size,omitempty"`

	// Foo is an example field of Memcached44295. Edit memcached44295_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// Memcached44295Status defines the observed state of Memcached44295
type Memcached44295Status struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Nodes []string `json:"nodes,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Memcached44295 is the Schema for the memcached44295s API
type Memcached44295 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Memcached44295Spec   `json:"spec,omitempty"`
	Status Memcached44295Status `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// Memcached44295List contains a list of Memcached44295
type Memcached44295List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Memcached44295 `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Memcached44295{}, &Memcached44295List{})
}
