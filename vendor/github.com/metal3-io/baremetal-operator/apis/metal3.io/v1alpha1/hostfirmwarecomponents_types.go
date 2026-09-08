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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FirmwareUpdate defines a firmware update specification.
type FirmwareUpdate struct {
	Component string `json:"component"`
	URL       string `json:"url"`
}

// FirmwareComponentStatus defines the status of a firmware component.
type FirmwareComponentStatus struct {
	Component          string      `json:"component"`
	InitialVersion     string      `json:"initialVersion"`
	CurrentVersion     string      `json:"currentVersion,omitempty"`
	LastVersionFlashed string      `json:"lastVersionFlashed,omitempty"`
	UpdatedAt          metav1.Time `json:"updatedAt,omitempty"`
}

type UpdatesConditionType string

const (
	// Indicates that the updates in the Spec are different than Status.
	HostFirmwareComponentsChangeDetected UpdatesConditionType = "ChangeDetected"

	// Indicates if the updates are valid and can be configured on the host.
	HostFirmwareComponentsValid UpdatesConditionType = "Valid"
)

// Firmware component constants.
const (
	// NICComponentPrefix is the prefix for NIC firmware components.
	NICComponentPrefix = "nic:"
)

// HostFirmwareComponentsSpec defines the desired state of HostFirmwareComponents.
type HostFirmwareComponentsSpec struct {
	Updates []FirmwareUpdate `json:"updates"`
}

// HostFirmwareComponentsStatus defines the observed state of HostFirmwareComponents.
type HostFirmwareComponentsStatus struct {
	// Updates is the list of all firmware components that should be updated
	// they are specified via name and url fields.
	// +optional
	Updates []FirmwareUpdate `json:"updates,omitempty"`

	// Components is the list of all available firmware components and their information.
	Components []FirmwareComponentStatus `json:"components,omitempty"`

	// Time that the status was last updated
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Track whether updates stored in the spec are valid based on the schema
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HostFirmwareComponents is the Schema for the hostfirmwarecomponents API.
type HostFirmwareComponents struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostFirmwareComponentsSpec   `json:"spec,omitempty"`
	Status HostFirmwareComponentsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HostFirmwareComponentsList contains a list of HostFirmwareComponents.
type HostFirmwareComponentsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostFirmwareComponents `json:"items"`
}

// Check whether the updates's names are valid.
func (host *HostFirmwareComponents) ValidateHostFirmwareComponents() error {
	allowedNames := map[string]struct{}{"bmc": {}, "bios": {}}
	for _, update := range host.Spec.Updates {
		componentName := update.Component
		if _, ok := allowedNames[componentName]; !ok && !strings.HasPrefix(componentName, NICComponentPrefix) {
			return fmt.Errorf("component %s is invalid, only 'bmc', 'bios', or names starting with '%s' are allowed as update names", update.Component, NICComponentPrefix)
		}
	}

	return nil
}

func init() {
	SchemeBuilder.Register(&HostFirmwareComponents{}, &HostFirmwareComponentsList{})
}
