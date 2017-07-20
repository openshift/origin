package template

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
)

// +genclient=true

// Template contains the inputs needed to produce a Config.
type Template struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// message is an optional instructional message that will
	// be displayed when this template is instantiated.
	// This field should inform the user how to utilize the newly created resources.
	// Parameter substitution will be performed on the message before being
	// displayed so that generated credentials and other parameters can be
	// included in the output.
	Message string

	// parameters is an optional array of Parameters used during the
	// Template to Config transformation.
	Parameters []Parameter

	// objects is an array of resources to include in this template.
	// If a namespace value is hardcoded in the object, it will be removed
	// during template instantiation, however if the namespace value
	// is, or contains, a ${PARAMETER_REFERENCE}, the resolved
	// value after parameter substitution will be respected and the object
	// will be created in that namespace.
	Objects []runtime.Object

	// objectLabels is an optional set of labels that are applied to every
	// object during the Template to Config transformation.
	ObjectLabels map[string]string

	// completion contains the requirements used to detect successful or failed
	// instantiations of the Template.
	Completion *TemplateCompletion
}

// TemplateList is a list of Template objects.
type TemplateList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Template
}

// Parameter defines a name/value variable that is to be processed during
// the Template to Config transformation.
type Parameter struct {
	// Required: Parameter name must be set and it can be referenced in Template
	// Items using ${PARAMETER_NAME}
	Name string

	// Optional: The name that will show in UI instead of parameter 'Name'
	DisplayName string

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

// TemplateCompletion contains the requirements used to detect successful or
// failed instantiations of a Template.
type TemplateCompletion struct {
	// deadlineSeconds is the number of seconds after which a template
	// instantiation will be considered to be failed, if it has not already
	// succeeded.
	DeadlineSeconds int64

	// objects reference the objects created by the TemplateInstance.
	Objects []TemplateInstanceObject
}

// +genclient=true

// TemplateInstance requests and records the instantiation of a Template.
// TemplateInstance is part of an experimental API.
type TemplateInstance struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec describes the desired state of this TemplateInstance.
	Spec TemplateInstanceSpec

	// Status describes the current state of this TemplateInstance.
	Status TemplateInstanceStatus
}

// TemplateInstanceSpec describes the desired state of a TemplateInstance.
type TemplateInstanceSpec struct {
	// Template is a full copy of the template for instantiation.
	Template Template

	// Secret is a reference to a Secret object containing the necessary
	// template parameters.
	Secret *kapi.LocalObjectReference

	// Requester holds the identity of the agent requesting the template
	// instantiation.
	Requester *TemplateInstanceRequester
}

// TemplateInstanceRequester holds the identity of an agent requesting a
// template instantiation.
type TemplateInstanceRequester struct {
	// Username is the username of the agent requesting a template instantiation.
	Username string
}

// TemplateInstanceStatus describes the current state of a TemplateInstance.
type TemplateInstanceStatus struct {
	// Conditions represent the latest available observations of a
	// TemplateInstance's current state.
	Conditions []TemplateInstanceCondition

	// Objects reference the objects created by the TemplateInstance.
	Objects []TemplateInstanceObject
}

// TemplateInstanceCondition contains condition information for a
// TemplateInstance.
type TemplateInstanceCondition struct {
	// Type of the condition, currently Ready or InstantiateFailure.
	Type TemplateInstanceConditionType
	// Status of the condition, one of True, False or Unknown.
	Status kapi.ConditionStatus
	// LastTransitionTime is the last time a condition status transitioned from
	// one state to another.
	LastTransitionTime metav1.Time
	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	Reason string
	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	Message string
}

// TemplateInstanceConditionType is the type of condition pertaining to a
// TemplateInstance.
type TemplateInstanceConditionType string

const (
	// TemplateInstanceReady indicates the readiness of the template
	// instantiation or one of its objects.
	TemplateInstanceReady TemplateInstanceConditionType = "Ready"
	// TemplateInstanceInstantiateFailure indicates the failure of the template
	// instantiation or one of its objects.
	TemplateInstanceInstantiateFailure TemplateInstanceConditionType = "InstantiateFailure"
	// TemplateInstanceWaiting indicates waiting for readiness or failure of the
	// template instantiation or one of its objects.
	TemplateInstanceWaiting TemplateInstanceConditionType = "Waiting"
)

// TemplateInstanceObject references an object created by a TemplateInstance.
type TemplateInstanceObject struct {
	// ref is a reference to the created object.  When used under .spec, only
	// name and namespace are used; these can contain references to parameters
	// which will be substituted following the usual rules.
	Ref kapi.ObjectReference

	// successRequirements hold all the requirements that must be met to
	// consider the Template instantiation a success.  These requirements are
	// ANDed together.
	SuccessRequirements []TemplateCompletionRequirement

	// failureRequirements hold the requirements that if any is met will cause
	// the Template instantiation to be considered a failure.  These
	// requirements are ORed together.
	FailureRequirements []TemplateCompletionRequirement

	// conditions represent the latest available observations of the object's
	// current state.  This is not used in .spec.
	Conditions []TemplateInstanceCondition
}

// TemplateCompletionRequirement holds a single requirement that is matched to
// partially determine success or failure of a Template instantiation.
type TemplateCompletionRequirement struct {
	// jsonPath specifies a JSONPath expression which is run against an object.
	JSONPath *string

	// condition specifies the name of a condition to be looked up on an object.
	Condition *string

	// equals is the value which should be matched for the requirement to be
	// fulfilled.
	Equals string
}

// TemplateInstanceList is a list of TemplateInstance objects.
type TemplateInstanceList struct {
	metav1.TypeMeta
	metav1.ListMeta

	// Items is a list of Templateinstances
	Items []TemplateInstance
}

// +genclient=true
// +nonNamespaced=true

// BrokerTemplateInstance holds the service broker-related state associated with
// a TemplateInstance.  BrokerTemplateInstance is part of an experimental API.
type BrokerTemplateInstance struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec describes the state of this BrokerTemplateInstance.
	Spec BrokerTemplateInstanceSpec
}

// BrokerTemplateInstanceSpec describes the state of a BrokerTemplateInstance.
type BrokerTemplateInstanceSpec struct {
	// TemplateInstance is a reference to a TemplateInstance object residing
	// in a namespace.
	TemplateInstance kapi.ObjectReference

	// Secret is a reference to a Secret object residing in a namespace,
	// containing the necessary template parameters.
	Secret kapi.ObjectReference

	// BindingIDs is a list of 'binding_id's provided during successive bind
	// calls to the template service broker.
	BindingIDs []string
}

// BrokerTemplateInstanceList is a list of BrokerTemplateInstance objects.
type BrokerTemplateInstanceList struct {
	metav1.TypeMeta
	metav1.ListMeta

	// Items is a list of BrokerTemplateInstances
	Items []BrokerTemplateInstance
}
