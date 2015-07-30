package v1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

// Template contains the inputs needed to produce a Config.
type Template struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Objects is an array of objects to include in this template. Required.
	Objects []runtime.RawExtension `json:"objects" description:"list of objects to include in the template"`

	// Optional: Parameters is an array of Parameters used during the
	// Template to Config transformation.
	Parameters []Parameter `json:"parameters,omitempty" description:"optional: list of parameters used during template to config transformation"`

	// Labels is a set of labels that are applied to every
	// object during the Template to Config transformation. Optional
	Labels map[string]string `json:"labels,omitempty" description:"optional: list of lables that are applied to every object during the template to config transformation"`
}

// TemplateList is a list of Template objects.
type TemplateList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of templates
	Items []Template `json:"items" description:"list of templates"`
}

// Parameter defines a name/value variable that is to be processed during
// the Template to Config transformation.
type Parameter struct {
	// Name must be set and it can be referenced in Template
	// Items using ${PARAMETER_NAME}. Required.
	Name string `json:"name" description:"name of the parameter"`

	// Description of a parameter. Optional.
	Description string `json:"description,omitempty" description:"optional: describes the parameter"`

	// Value holds the Parameter data. If specified, the generator will be
	// ignored. The value replaces all occurrences of the Parameter ${Name}
	// expression during the Template to Config transformation. Optional.
	Value string `json:"value,omitempty" description:"optional: holds the parameter data.  if specified, the generator is ignored.  the value replaces all occurrences of the parameter ${Name} expression during template to config transformation"`

	// Generate specifies the generator to be used to generate random string
	// from an input value specified by From field. The result string is
	// stored into Value field. If empty, no generator is being used, leaving
	// the result Value untouched. Optional.
	Generate string `json:"generate,omitempty" description:"optional: generate specifies the generator to be used to generate random string from an input value specified by the from field.  the result string is stored in the value field. if not specified, the value field is untouched"`

	// From is an input value for the generator. Optional.
	From string `json:"from,omitempty" description:"input value for the generator"`

	// Optional: Indicates the parameter must have a value.  Defaults to false.
	Required bool `json:"required,omitempty" description:"indicates the parameter must have a non-empty value or be generated"`
}
