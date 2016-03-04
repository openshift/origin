package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
)

// Template contains the inputs needed to produce a Config.
type Template struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Objects is an array of objects to include in this template. Required.
	Objects []runtime.RawExtension `json:"objects"`

	// Optional: Parameters is an array of Parameters used during the
	// Template to Config transformation.
	Parameters []Parameter `json:"parameters,omitempty"`

	// Labels is a set of labels that are applied to every
	// object during the Template to Config transformation. Optional
	Labels map[string]string `json:"labels,omitempty"`
}

// TemplateList is a list of Template objects.
type TemplateList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of templates
	Items []Template `json:"items"`
}

// Parameter defines a name/value variable that is to be processed during
// the Template to Config transformation.
type Parameter struct {
	// Name must be set and it can be referenced in Template
	// Items using ${PARAMETER_NAME}. Required.
	Name string `json:"name"`

	// Optional: The name that will show in UI instead of parameter 'Name'
	DisplayName string `json:"displayName,omitempty"`

	// Description of a parameter. Optional.
	Description string `json:"description,omitempty"`

	// Value holds the Parameter data. If specified, the generator will be
	// ignored. The value replaces all occurrences of the Parameter ${Name}
	// expression during the Template to Config transformation. Optional.
	Value string `json:"value,omitempty"`

	// Generate specifies the generator to be used to generate random string
	// from an input value specified by From field. The result string is
	// stored into Value field. If empty, no generator is being used, leaving
	// the result Value untouched. Optional.
	Generate string `json:"generate,omitempty"`

	// From is an input value for the generator. Optional.
	From string `json:"from,omitempty"`

	// Optional: Indicates the parameter must have a value.  Defaults to false.
	Required bool `json:"required,omitempty"`
}
