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

// HardwareDataSpec defines the desired state of HardwareData.
type HardwareDataSpec struct {

	// The hardware discovered on the host during its inspection.
	HardwareDetails *HardwareDetails `json:"hardware,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=hardwaredata,scope=Namespaced,shortName=hd
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of HardwareData"

// HardwareData is the Schema for the hardwaredata API.
type HardwareData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HardwareDataSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// HardwareDataList contains a list of HardwareData.
type HardwareDataList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HardwareData `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HardwareData{}, &HardwareDataList{})
}
