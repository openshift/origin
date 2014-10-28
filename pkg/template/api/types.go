package api

import (
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

// Template contains the inputs needed to produce a Config.
type Template struct {
	kubeapi.TypeMeta `json:",inline" yaml:",inline"`

	// Required: Name identifies the Template.
	Name string `json:"name" yaml:"name"`

	// Optional: Description describes the Template.
	Description string `json:"description" yaml:"description"`

	// Required: Items is an array of Kubernetes resources of Service,
	// Pod and/or ReplicationController kind.
	// TODO: Handle unregistered types. Define custom []runtime.Object
	//       type and its unmarshaller instead of []runtime.Object.
	Items []runtime.EmbeddedObject `json:"items" yaml:"items"`

	// Optional: Parameters is an array of Parameters used during the
	// Template to Config transformation.
	Parameters []Parameter `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// Parameter defines a name/value variable that is to be processed during
// the Template to Config transformation.
type Parameter struct {
	// Required: Name uniquely identifies the Parameter. A TemplateProcessor
	// searches given Template for all occurances of the Parameter name, ie.
	// ${PARAM_NAME}, and replaces it with it's corresponding Parameter value.
	Name string `json:"name" yaml:"name"`

	// Optional: Description describes the Parameter.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Optional: Generate specifies the generator to be used to generate
	// random string from an input value specified by From field. The result
	// string is stored into Value field. If empty, no generator is being
	// used, leaving the result Value untouched.
	Generate string `json:"generate,omitempty" yaml:"generate,omitempty"`

	// Optional: From is an input value for the generator.
	From string `json:"from,omitempty" yaml:"from,omitempty"`

	// Optional: Value holds the Parameter data. The Value data can be
	// overwritten by the generator. The value replaces all occurances
	// of the Parameter ${Name} expression during the Template to Config
	// transformation.
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}
