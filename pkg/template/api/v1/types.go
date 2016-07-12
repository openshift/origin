package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
)

// +genclient=true

// Template contains the inputs needed to produce a Config.
type Template struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// message is an optional instructional message that will
	// be displayed when this template is instantiated.
	// This field should inform the user how to utilize the newly created resources.
	// Parameter substitution will be performed on the message before being
	// displayed so that generated credentials and other parameters can be
	// included in the output.
	Message string `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`

	// objects is an array of resources to include in this template.
	Objects []runtime.RawExtension `json:"objects" protobuf:"bytes,3,rep,name=objects"`

	// parameters is an optional array of Parameters used during the
	// Template to Config transformation.
	Parameters []Parameter `json:"parameters,omitempty" protobuf:"bytes,4,rep,name=parameters"`

	// labels is a optional set of labels that are applied to every
	// object during the Template to Config transformation.
	ObjectLabels map[string]string `json:"labels,omitempty" protobuf:"bytes,5,rep,name=labels"`
}

// TemplateList is a list of Template objects.
type TemplateList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is a list of templates
	Items []Template `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// Parameter defines a name/value variable that is to be processed during
// the Template to Config transformation.
type Parameter struct {
	// Name must be set and it can be referenced in Template
	// Items using ${PARAMETER_NAME}. Required.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Optional: The name that will show in UI instead of parameter 'Name'
	DisplayName string `json:"displayName,omitempty" protobuf:"bytes,2,opt,name=displayName"`

	// Description of a parameter. Optional.
	Description string `json:"description,omitempty" protobuf:"bytes,3,opt,name=description"`

	// Value holds the Parameter data. If specified, the generator will be
	// ignored. The value replaces all occurrences of the Parameter ${Name}
	// expression during the Template to Config transformation. Optional.
	Value string `json:"value,omitempty" protobuf:"bytes,4,opt,name=value"`

	// generate specifies the generator to be used to generate random string
	// from an input value specified by From field. The result string is
	// stored into Value field. If empty, no generator is being used, leaving
	// the result Value untouched. Optional.
	//
	// The only supported generator is "expression", which accepts a "from"
	// value in the form of a simple regular expression containing the
	// range expression "[a-zA-Z0-9]", and the length expression "a{length}".
	//
	// Examples:
	//
	// from             | value
	// -----------------------------
	// "test[0-9]{1}x"  | "test7x"
	// "[0-1]{8}"       | "01001100"
	// "0x[A-F0-9]{4}"  | "0xB3AF"
	// "[a-zA-Z0-9]{8}" | "hW4yQU5i"
	//
	Generate string `json:"generate,omitempty" protobuf:"bytes,5,opt,name=generate"`

	// From is an input value for the generator. Optional.
	From string `json:"from,omitempty" protobuf:"bytes,6,opt,name=from"`

	// Optional: Indicates the parameter must have a value.  Defaults to false.
	Required bool `json:"required,omitempty" protobuf:"varint,7,opt,name=required"`
}
