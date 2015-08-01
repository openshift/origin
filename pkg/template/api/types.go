package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

// Template contains the inputs needed to produce a Config.
type Template struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Optional: Parameters is an array of Parameters used during the
	// Template to Config transformation.
	Parameters []Parameter

	// Required: A list of resources to create
	Objects []runtime.Object

	// Optional: ObjectLabels is a set of labels that are applied to every
	// object during the Template to Config transformation
	ObjectLabels map[string]string
}

// TemplateList is a list of Template objects.
type TemplateList struct {
	kapi.TypeMeta
	kapi.ListMeta
	Items []Template
}

// Parameter defines a name/value variable that is to be processed during
// the Template to Config transformation.
type Parameter struct {
	// Required: Parameter name must be set and it can be referenced in Template
	// Items using ${PARAMETER_NAME}
	Name string

	// Optional: Parameter can have description
	Description string

	// Optional: Value holds the Parameter data. If specified, the generator
	// will be ignored. The value replaces all occurrences of the Parameter
	// ${Name} expression during the Template to Config transformation.
	Value string

	// Optional: Generate specifies the generator to be used to generate
	// random string from an input value specified by From field. The result
	// string is stored into Value field. If empty, no generator is being
	// used, leaving the result Value untouched.
	Generate string

	// Optional: From is an input value for the generator.
	From string

	// Optional: Indicates the parameter must have a value.  Defaults to false.
	Required bool
}
