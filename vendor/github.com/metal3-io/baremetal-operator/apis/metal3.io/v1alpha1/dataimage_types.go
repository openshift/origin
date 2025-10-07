/*


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

const DataImageFinalizer = "dataimage.metal3.io"

// Contains the DataImage currently attached to the BMH.
type AttachedImageReference struct {
	URL string `json:"url"`
}

// Contains the count of errors and the last error message.
type DataImageError struct {
	Count   int    `json:"count"`
	Message string `json:"message"`
}

// DataImageSpec defines the desired state of DataImage.
type DataImageSpec struct {
	// Url is the address of the dataImage that we want to attach
	// to a BareMetalHost
	URL string `json:"url"`
}

// DataImageStatus defines the observed state of DataImage.
type DataImageStatus struct {
	// Time of last reconciliation
	// +optional
	LastReconciled *metav1.Time `json:"lastReconciled,omitempty"`

	// Currently attached DataImage
	AttachedImage AttachedImageReference `json:"attachedImage,omitempty"`

	// Error count and message when attaching/detaching
	Error DataImageError `json:"error,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DataImage is the Schema for the dataimages API.
type DataImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataImageSpec   `json:"spec,omitempty"`
	Status DataImageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DataImageList contains a list of DataImage.
type DataImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataImage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DataImage{}, &DataImageList{})
}
