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
	// If a namespace value is hardcoded in the object, it will be removed
	// during template instantiation, however if the namespace value
	// is, or contains, a ${PARAMETER_REFERENCE}, the resolved
	// value after parameter substitution will be respected and the object
	// will be created in that namespace.
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

// +genclient=true

// TemplateInstance requests and records the instantiation of a Template.
// TemplateInstance is part of an experimental API.
type TemplateInstance struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// spec describes the desired state of this TemplateInstance.
	Spec TemplateInstanceSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`

	// status describes the current state of this TemplateInstance.
	Status TemplateInstanceStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

// TemplateInstanceSpec describes the desired state of a TemplateInstance.
type TemplateInstanceSpec struct {
	// template is a full copy of the template for instantiation.
	Template Template `json:"template" protobuf:"bytes,1,opt,name=template"`

	// secret is a reference to a Secret object containing the necessary
	// template parameters.
	Secret kapi.LocalObjectReference `json:"secret" protobuf:"bytes,2,opt,name=secret"`

	// requester holds the identity of the agent requesting the template
	// instantiation.
	Requester *TemplateInstanceRequester `json:"requester" protobuf:"bytes,3,opt,name=requester"`
}

// TemplateInstanceRequester holds the identity of an agent requesting a
// template instantiation.
type TemplateInstanceRequester struct {
	// username is the username of the agent requesting a template instantiation.
	Username string `json:"username" protobuf:"bytes,1,opt,name=username"`
}

// TemplateInstanceStatus describes the current state of a TemplateInstance.
type TemplateInstanceStatus struct {
	// conditions represent the latest available observations of a
	// TemplateInstance's current state.
	Conditions []TemplateInstanceCondition `json:"conditions" protobuf:"bytes,1,rep,name=conditions"`
}

// TemplateInstanceCondition contains condition information for a
// TemplateInstance.
type TemplateInstanceCondition struct {
	// Type of the condition, currently Ready or InstantiateFailure.
	Type TemplateInstanceConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=TemplateInstanceConditionType"`
	// Status of the condition, one of True, False or Unknown.
	Status kapi.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status"`
	// LastTransitionTime is the last time a condition status transitioned from
	// one state to another.
	LastTransitionTime unversioned.Time `json:"lastTransitionTime" protobuf:"bytes,3,opt,name=lastTransitionTime"`
	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	Reason string `json:"reason" protobuf:"bytes,4,opt,name=reason"`
	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	Message string `json:"message" protobuf:"bytes,5,opt,name=message"`
}

// TemplateInstanceConditionType is the type of condition pertaining to a
// TemplateInstance.
type TemplateInstanceConditionType string

const (
	// TemplateInstanceReady indicates the readiness of the template
	// instantiation.
	TemplateInstanceReady TemplateInstanceConditionType = "Ready"
	// TemplateInstanceInstantiateFailure indicates the failure of the template
	// instantiation
	TemplateInstanceInstantiateFailure TemplateInstanceConditionType = "InstantiateFailure"
)

// TemplateInstanceList is a list of TemplateInstance objects.
type TemplateInstanceList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object metadata.
	unversioned.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// items is a list of Templateinstances
	Items []TemplateInstance `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// +genclient=true
// +nonNamespaced=true

// BrokerTemplateInstance holds the service broker-related state associated with
// a TemplateInstance.  BrokerTemplateInstance is part of an experimental API.
type BrokerTemplateInstance struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// spec describes the state of this BrokerTemplateInstance.
	Spec BrokerTemplateInstanceSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// BrokerTemplateInstanceSpec describes the state of a BrokerTemplateInstance.
type BrokerTemplateInstanceSpec struct {
	// templateinstance is a reference to a TemplateInstance object residing
	// in a namespace.
	TemplateInstance kapi.ObjectReference `json:"templateInstance" protobuf:"bytes,1,opt,name=templateInstance"`

	// secret is a reference to a Secret object residing in a namespace,
	// containing the necessary template parameters.
	Secret kapi.ObjectReference `json:"secret" protobuf:"bytes,2,opt,name=secret"`

	// bindingids is a list of 'binding_id's provided during successive bind
	// calls to the template service broker.
	BindingIDs []string `json:"bindingIDs" protobuf:"bytes,3,rep,name=bindingIDs"`
}

// BrokerTemplateInstanceList is a list of BrokerTemplateInstance objects.
type BrokerTemplateInstanceList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object metadata.
	unversioned.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// items is a list of BrokerTemplateInstances
	Items []BrokerTemplateInstance `json:"items" protobuf:"bytes,2,rep,name=items"`
}
