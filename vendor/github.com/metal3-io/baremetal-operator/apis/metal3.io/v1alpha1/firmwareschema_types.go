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
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Additional data describing the firmware setting.
type SettingSchema struct {

	// The type of setting.
	// +kubebuilder:validation:Enum=Enumeration;String;Integer;Boolean;Password
	//nolint:tagliatelle
	AttributeType string `json:"attribute_type,omitempty"`

	//nolint:tagliatelle
	// The allowable value for an Enumeration type setting.
	AllowableValues []string `json:"allowable_values,omitempty"`

	// The lowest value for an Integer type setting.
	//nolint:tagliatelle
	LowerBound *int `json:"lower_bound,omitempty"`

	// The highest value for an Integer type setting.
	//nolint:tagliatelle
	UpperBound *int `json:"upper_bound,omitempty"`

	// Minimum length for a String type setting.
	//nolint:tagliatelle
	MinLength *int `json:"min_length,omitempty"`

	// Maximum length for a String type setting.
	//nolint:tagliatelle
	MaxLength *int `json:"max_length,omitempty"`

	// Whether or not this setting is read only.
	//nolint:tagliatelle
	ReadOnly *bool `json:"read_only,omitempty"`

	// Whether or not this setting's value is unique to this node, e.g.
	// a serial number.
	Unique *bool `json:"unique,omitempty"`
}

type SchemaSettingError struct {
	name    string
	message string
}

func (e SchemaSettingError) Error() string {
	return fmt.Sprintf("Setting %s is invalid, %s", e.name, e.message)
}

func (schema *SettingSchema) Validate(name string, value intstr.IntOrString) error {
	if schema.ReadOnly != nil && *schema.ReadOnly {
		return SchemaSettingError{name: name, message: "it is ReadOnly"}
	}

	if strings.Contains(name, "Password") {
		return SchemaSettingError{name: name, message: "Password fields can't be set"}
	}

	// Check if valid based on type
	switch schema.AttributeType {
	case "Enumeration":
		for _, av := range schema.AllowableValues {
			if value.String() == av {
				return nil
			}
		}
		return SchemaSettingError{name: name, message: "unknown enumeration value - " + value.String()}

	case "Integer":
		if value.Type == intstr.String {
			if _, err := strconv.Atoi(value.String()); err != nil {
				return SchemaSettingError{name: name, message: fmt.Sprintf("String %s entered while integer expected", value.String())}
			}
		}
		if schema.LowerBound != nil && value.IntValue() < *schema.LowerBound {
			return SchemaSettingError{name: name, message: fmt.Sprintf("integer %d is below minimum value %d", value.IntValue(), *schema.LowerBound)}
		}
		if schema.UpperBound != nil && value.IntValue() > *schema.UpperBound {
			return SchemaSettingError{name: name, message: fmt.Sprintf("integer %d is above maximum value %d", value.IntValue(), *schema.UpperBound)}
		}
		return nil

	case "String":
		strLen := len(value.String())
		if schema.MinLength != nil && strLen < *schema.MinLength {
			return SchemaSettingError{name: name, message: fmt.Sprintf("string %s length is below minimum length %d", value.String(), *schema.MinLength)}
		}
		if schema.MaxLength != nil && strLen > *schema.MaxLength {
			return SchemaSettingError{name: name, message: fmt.Sprintf("string %s length is above maximum length %d", value.String(), *schema.MaxLength)}
		}
		return nil

	case "Boolean":
		if value.String() == "true" || value.String() == "false" {
			return nil
		}
		return SchemaSettingError{name: name, message: value.String() + " is not a boolean"}

	case "Password":
		// Prevent sets of password types
		return SchemaSettingError{name: name, message: "passwords are immutable"}

	case "":
		// allow the set as BIOS registry fields may not have been available
		return nil

	default:
		// Unexpected attribute type
		return SchemaSettingError{name: name, message: "unexpected attribute type " + schema.AttributeType}
	}
}

// FirmwareSchemaSpec defines the desired state of FirmwareSchema.
type FirmwareSchemaSpec struct {

	// The hardware vendor associated with this schema
	// +optional
	HardwareVendor string `json:"hardwareVendor,omitempty"`

	// The hardware model associated with this schema
	// +optional
	HardwareModel string `json:"hardwareModel,omitempty"`

	// Map of firmware name to schema
	Schema map[string]SettingSchema `json:"schema" required:"true"`
}

//+kubebuilder:object:root=true

// FirmwareSchema is the Schema for the firmwareschemas API.
type FirmwareSchema struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FirmwareSchemaSpec `json:"spec,omitempty"`
}

// Check whether the setting's name and value is valid using the schema.
func (host *FirmwareSchema) ValidateSetting(name string, value intstr.IntOrString, schemas map[string]SettingSchema) error {
	schema, ok := schemas[name]
	if !ok {
		return SchemaSettingError{name: name, message: "it is not in the associated schema"}
	}

	return schema.Validate(name, value)
}

//+kubebuilder:object:root=true

// FirmwareSchemaList contains a list of FirmwareSchema.
type FirmwareSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FirmwareSchema `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FirmwareSchema{}, &FirmwareSchemaList{})
}
