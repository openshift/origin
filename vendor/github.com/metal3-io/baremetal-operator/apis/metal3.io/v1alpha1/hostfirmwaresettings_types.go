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
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type SettingsMap map[string]string
type DesiredSettingsMap map[string]intstr.IntOrString

type SchemaReference struct {
	// `namespace` is the namespace of the where the schema is stored.
	Namespace string `json:"namespace"`
	// `name` is the reference to the schema.
	Name string `json:"name"`
}

type SettingsConditionType string

const (
	// Indicates that the settings in the Spec are different than Status.
	FirmwareSettingsChangeDetected SettingsConditionType = "ChangeDetected"

	// Indicates if the settings are valid and can be configured on the host.
	FirmwareSettingsValid SettingsConditionType = "Valid"
)

// HostFirmwareSettingsSpec defines the desired state of HostFirmwareSettings.
type HostFirmwareSettingsSpec struct {

	// Settings are the desired firmware settings stored as name/value pairs.
	// +patchStrategy=merge
	Settings DesiredSettingsMap `json:"settings" patchStrategy:"merge" required:"true"`
}

// HostFirmwareSettingsStatus defines the observed state of HostFirmwareSettings.
type HostFirmwareSettingsStatus struct {
	// FirmwareSchema is a reference to the Schema used to describe each
	// FirmwareSetting. By default, this will be a Schema in the same
	// Namespace as the settings but it can be overwritten in the Spec
	FirmwareSchema *SchemaReference `json:"schema,omitempty"`

	// Settings are the firmware settings stored as name/value pairs
	Settings SettingsMap `json:"settings" required:"true"`

	// Time that the status was last updated
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Track whether settings stored in the spec are valid based on the schema
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=hfs
//+kubebuilder:subresource:status

// HostFirmwareSettings is the Schema for the hostfirmwaresettings API.
type HostFirmwareSettings struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostFirmwareSettingsSpec   `json:"spec,omitempty"`
	Status HostFirmwareSettingsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HostFirmwareSettingsList contains a list of HostFirmwareSettings.
type HostFirmwareSettingsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostFirmwareSettings `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HostFirmwareSettings{}, &HostFirmwareSettingsList{})
}
