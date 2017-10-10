/*
Copyright 2016 The Kubernetes Authors.

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

package servicecatalog

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterServiceBroker represents an entity that provides ClusterServiceClasses for use in the
// service catalog. ClusterServiceBroker is backed by an OSBAPI v2 broker supporting the
// latest minor version of the v2 major version.
type ClusterServiceBroker struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   ClusterServiceBrokerSpec
	Status ClusterServiceBrokerStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterServiceBrokerList is a list of Brokers.
type ClusterServiceBrokerList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []ClusterServiceBroker
}

// ClusterServiceBrokerSpec represents a description of a Broker.
type ClusterServiceBrokerSpec struct {
	// URL is the address used to communicate with the ClusterServiceBroker.
	URL string

	// AuthInfo contains the data that the service catalog should use to authenticate
	// with the Service Broker.
	AuthInfo *ServiceBrokerAuthInfo

	// InsecureSkipTLSVerify disables TLS certificate verification when communicating with this Broker.
	// This is strongly discouraged.  You should use the CABundle instead.
	// +optional
	InsecureSkipTLSVerify bool

	// CABundle is a PEM encoded CA bundle which will be used to validate a Broker's serving certificate.
	// +optional
	CABundle []byte

	// RelistBehavior specifies the type of relist behavior the catalog should
	// exhibit when relisting ClusterServiceClasses available from a broker.
	RelistBehavior ServiceBrokerRelistBehavior

	// RelistDuration is the frequency by which a controller will relist the
	// broker when the RelistBehavior is set to ServiceBrokerRelistBehaviorDuration.
	RelistDuration *metav1.Duration

	// RelistRequests is a strictly increasing, non-negative integer counter that
	// can be manually incremented by a user to manually trigger a relist.
	RelistRequests int64
}

// ServiceBrokerRelistBehavior represents a type of broker relist behavior.
type ServiceBrokerRelistBehavior string

const (
	// ServiceBrokerRelistBehaviorDuration indicates that the broker will be
	// relisted automatically after the specified duration has passed.
	ServiceBrokerRelistBehaviorDuration ServiceBrokerRelistBehavior = "Duration"

	// ServiceBrokerRelistBehaviorManual indicates that the broker is only
	// relisted when the spec of the broker changes.
	ServiceBrokerRelistBehaviorManual ServiceBrokerRelistBehavior = "Manual"
)

// ServiceBrokerAuthInfo is a union type that contains information on one of the authentication methods
// the the service catalog and brokers may support, according to the OpenServiceBroker API
// specification (https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md).
type ServiceBrokerAuthInfo struct {
	// Basic provides configuration for basic authentication.
	Basic *BasicAuthConfig
	// BearerTokenAuthConfig provides configuration to send an opaque value as a bearer token.
	// The value is referenced from the 'token' field of the given secret.  This value should only
	// contain the token value and not the `Bearer` scheme.
	Bearer *BearerTokenAuthConfig

	// DEPRECATED: use `Basic` field for configuring basic authentication instead.
	// BasicAuthSecret is a reference to a Secret containing auth information the
	// catalog should use to authenticate to this ServiceBroker using basic auth.
	BasicAuthSecret *v1.ObjectReference
}

// BasicAuthConfig provides config for the basic authentication.
type BasicAuthConfig struct {
	// SecretRef is a reference to a Secret containing information the
	// catalog should use to authenticate to this ServiceBroker.
	//
	// Required at least one of the fields:
	// - Secret.Data["username"] - username used for authentication
	// - Secret.Data["password"] - password or token needed for authentication
	SecretRef *v1.ObjectReference
}

// BearerTokenAuthConfig provides config for the bearer token authentication.
type BearerTokenAuthConfig struct {
	// SecretRef is a reference to a Secret containing information the
	// catalog should use to authenticate to this ServiceBroker.
	//
	// Required field:
	// - Secret.Data["token"] - bearer token for authentication
	SecretRef *v1.ObjectReference
}

const (
	// BasicAuthUsernameKey is the key of the username for SecretTypeBasicAuth secrets
	BasicAuthUsernameKey = "username"
	// BasicAuthPasswordKey is the key of the password or token for SecretTypeBasicAuth secrets
	BasicAuthPasswordKey = "password"

	// BearerTokenKey is the key of the bearer token for SecretTypeBearerTokenAuth secrets
	BearerTokenKey = "token"
)

// ClusterServiceBrokerStatus represents the current status of a ClusterServiceBroker
type ClusterServiceBrokerStatus struct {
	Conditions []ServiceBrokerCondition

	// ReconciledGeneration is the 'Generation' of the ServiceBrokerSpec that
	// was last processed by the controller. The reconciled generation is updated
	// even if the controller failed to process the spec.
	ReconciledGeneration int64

	// OperationStartTime is the time at which the current operation began.
	OperationStartTime *metav1.Time
}

// ServiceBrokerCondition contains condition information for a Service Broker.
type ServiceBrokerCondition struct {
	// Type of the condition, currently ('Ready').
	Type ServiceBrokerConditionType

	// Status of the condition, one of ('True', 'False', 'Unknown').
	Status ConditionStatus

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	LastTransitionTime metav1.Time

	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	Reason string

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	Message string
}

// ServiceBrokerConditionType represents a broker condition value.
type ServiceBrokerConditionType string

const (
	// ServiceBrokerConditionReady represents the fact that a given broker condition
	// is in ready state.
	ServiceBrokerConditionReady ServiceBrokerConditionType = "Ready"

	// ServiceBrokerConditionFailed represents information about a final failure
	// that should not be retried.
	ServiceBrokerConditionFailed ServiceBrokerConditionType = "Failed"
)

// ConditionStatus represents a condition's status.
type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in
// the condition; "ConditionFalse" means a resource is not in the condition;
// "ConditionUnknown" means kubernetes can't decide if a resource is in the
// condition or not. In the future, we could add other intermediate
// conditions, e.g. ConditionDegraded.
const (
	// ConditionTrue represents the fact that a given condition is true
	ConditionTrue ConditionStatus = "True"

	// ConditionFalse represents the fact that a given condition is false
	ConditionFalse ConditionStatus = "False"

	// ConditionUnknown represents the fact that a given condition is unknown
	ConditionUnknown ConditionStatus = "Unknown"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterServiceClassList is a list of ClusterServiceClasses.
type ClusterServiceClassList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []ClusterServiceClass
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterServiceClass represents an offering in the service catalog.
type ClusterServiceClass struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   ClusterServiceClassSpec
	Status ClusterServiceClassStatus
}

// ClusterServiceClassSpec represents details about a ClusterServicePlan
type ClusterServiceClassSpec struct {
	// ClusterServiceBrokerName is the reference to the Broker that provides this
	// ClusterServiceClass.
	//
	// Immutable.
	ClusterServiceBrokerName string

	// ExternalName is the name of this object that the Service Broker
	// exposed this Service Class as. Mutable.
	ExternalName string

	// ExternalID is the identity of this object for use with the OSB API.
	//
	// Immutable.
	ExternalID string

	// Description is a short description of this ClusterServiceClass.
	Description string

	// Bindable indicates whether a user can create bindings to an ServiceInstance
	// provisioned from this service. ClusterServicePlan has an optional field called
	// Bindable which overrides the value of this field.
	Bindable bool

	// PlanUpdatable indicates whether instances provisioned from this
	// ClusterServiceClass may change ClusterServicePlans after being provisioned.
	PlanUpdatable bool

	// ExternalMetadata is a blob of information about the ClusterServiceClass, meant
	// to be user-facing content and display instructions.  This field may
	// contain platform-specific conventional values.
	ExternalMetadata *runtime.RawExtension

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// Tags is a list of strings that represent different classification
	// attributes of the ClusterServiceClass.  These are used in Cloud Foundry in a
	// way similar to Kubernetes labels, but they currently have no special
	// meaning in Kubernetes.
	Tags []string

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// Requires exposes a list of Cloud Foundry-specific 'permissions'
	// that must be granted to an instance of this service within Cloud
	// Foundry.  These 'permissions' have no meaning within Kubernetes and an
	// ServiceInstance provisioned from this ClusterServiceClass will not work correctly.
	Requires []string
}

// ClusterServiceClassStatus represents status information about a
// ClusterServiceClass.
type ClusterServiceClassStatus struct {
	// RemovedFromBrokerCatalog indicates that the broker removed the service from its
	// catalog.
	RemovedFromBrokerCatalog bool
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterServicePlanList is a list of ServicePlans.
type ClusterServicePlanList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []ClusterServicePlan
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterServicePlan represents a tier of a ClusterServiceClass.
type ClusterServicePlan struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   ClusterServicePlanSpec
	Status ClusterServicePlanStatus
}

// ClusterServicePlanSpec represents details about the ClusterServicePlan
type ClusterServicePlanSpec struct {
	// ClusterServiceBrokerName is the name of the ClusterServiceBroker that offers this
	// ClusterServicePlan.
	ClusterServiceBrokerName string

	// ExternalName is the name of this object that the Service Broker
	// exposed this Service Plan as. Mutable.
	ExternalName string

	// ExternalID is the identity of this object for use with the OSB API.
	//
	// Immutable.
	ExternalID string

	// Description is a short description of this ClusterServicePlan.
	Description string

	// Bindable indicates whether a user can create bindings to an ServiceInstance
	// using this ClusterServicePlan.  If set, overrides the value of the
	// ClusterServiceClass.Bindable field.
	Bindable *bool

	// Free indicates whether this ClusterServicePlan is available at no cost.
	Free bool

	// ExternalMetadata is a blob of information about the plan, meant to be
	// user-facing content and display instructions.  This field may contain
	// platform-specific conventional values.
	ExternalMetadata *runtime.RawExtension

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// ServiceInstanceCreateParameterSchema is the schema for the parameters
	// that may be supplied when provisioning a new ServiceInstance on this plan.
	ServiceInstanceCreateParameterSchema *runtime.RawExtension

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// ServiceInstanceUpdateParameterSchema is the schema for the parameters
	// that may be updated once an ServiceInstance has been provisioned on this plan.
	// This field only has meaning if the ClusterServiceClass is PlanUpdatable.
	ServiceInstanceUpdateParameterSchema *runtime.RawExtension

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// ServiceBindingCreateParameterSchema is the schema for the parameters that
	// may be supplied binding to an ServiceInstance on this plan.
	ServiceBindingCreateParameterSchema *runtime.RawExtension

	// ClusterServiceClassRef is a reference to the service class that
	// owns this plan.
	ClusterServiceClassRef v1.LocalObjectReference
}

// ClusterServicePlanStatus represents status information about a
// ClusterServicePlan.
type ClusterServicePlanStatus struct {
	// RemovedFromBrokerCatalog indicates that the broker removed the plan
	// from its catalog.
	RemovedFromBrokerCatalog bool
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceInstanceList is a list of instances.
type ServiceInstanceList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []ServiceInstance
}

// UserInfo holds information about the user that last changed a resource's spec.
type UserInfo struct {
	Username string
	UID      string
	Groups   []string
	Extra    map[string]ExtraValue
}

// ExtraValue contains additional information about a user that may be
// provided by the authenticator.
type ExtraValue []string

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceInstance represents a provisioned instance of a ClusterServiceClass.
type ServiceInstance struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   ServiceInstanceSpec
	Status ServiceInstanceStatus
}

// PlanReference defines the user specification for the desired
// ServicePlan and ServiceClass. Because there are multiple ways to
// specify the desired Class/Plan, this structure specifies the
// allowed ways to specify the intent.
type PlanReference struct {
	// ExternalClusterServiceClassName is the human-readable name of the
	// service as reported by the broker. Note that if the broker changes
	// the name of the ClusterServiceClass, it will not be reflected here,
	// and to see the current name of the ClusterServiceClass, you should
	// follow the ClusterServiceClassRef below.
	//
	// Immutable.
	ExternalClusterServiceClassName string
	// ExternalClusterServicePlanName is the human-readable name of the plan
	// as reported by the broker. Note that if the broker changes the name
	// of the ClusterServicePlan, it will not be reflected here, and to see
	// the current name of the ClusterServicePlan, you should follow the
	// ClusterServicePlanRef below.
	ExternalClusterServicePlanName string
}

// ServiceInstanceSpec represents the desired state of an Instance.
type ServiceInstanceSpec struct {
	PlanReference

	// ClusterServiceClassRef is a reference to the ClusterServiceClass
	// that the user selected.
	// This is set by the controller based on ExternalClusterServiceClassName
	ClusterServiceClassRef *v1.ObjectReference
	// ClusterServicePlanRef is a reference to the ClusterServicePlan
	// that the user selected.
	// This is set by the controller based on ExternalClusterServicePlanName
	ClusterServicePlanRef *v1.ObjectReference

	// Parameters is a set of the parameters to be passed to the underlying
	// broker. The inline YAML/JSON payload to be translated into equivalent
	// JSON object. If a top-level parameter name exists in multiples sources
	// among `Parameters` and `ParametersFrom` fields, it is considered to be
	// a user error in the specification
	//
	// The Parameters field is NOT secret or secured in any way and should
	// NEVER be used to hold sensitive information. To set parameters that
	// contain secret information, you should ALWAYS store that information
	// in a Secret and use the ParametersFrom field.
	//
	// +optional
	Parameters *runtime.RawExtension

	// List of sources to populate parameters.
	// If a top-level parameter name exists in multiples sources among
	// `Parameters` and `ParametersFrom` fields, it is
	// considered to be a user error in the specification
	// +optional
	ParametersFrom []ParametersFromSource

	// ExternalID is the identity of this object for use with the OSB API.
	//
	// Immutable.
	ExternalID string

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// UserInfo contains information about the user that last modified this
	// instance. This field is set by the API server and not settable by the
	// end-user. User-provided values for this field are not saved.
	// +optional
	UserInfo *UserInfo

	// UpdateRequests is a strictly increasing, non-negative integer counter that
	// can be manually incremented by a user to manually trigger an update. This
	// allows for parameters to be updated with any out-of-band changes that have
	// been made to the secrets from which the parameters are sourced.
	UpdateRequests int64
}

// ServiceInstanceStatus represents the current status of an Instance.
type ServiceInstanceStatus struct {
	// Conditions is an array of ServiceInstanceConditions capturing aspects of an
	// ServiceInstance's status.
	Conditions []ServiceInstanceCondition

	// AsyncOpInProgress is set to true if there is an ongoing async operation
	// against this ServiceInstance in progress.
	AsyncOpInProgress bool

	// OrphanMitigationInProgress is set to true if there is an ongoing orphan
	// mitigation operation against this ServiceInstance in progress.
	OrphanMitigationInProgress bool

	// LastOperation is the string that the broker may have returned when
	// an async operation started, it should be sent back to the broker
	// on poll requests as a query param.
	LastOperation *string

	// DashboardURL is the URL of a web-based management user interface for
	// the service instance.
	DashboardURL *string

	// CurrentOperation is the operation the Controller is currently performing
	// on the ServiceInstance.
	CurrentOperation ServiceInstanceOperation

	// ReconciledGeneration is the 'Generation' of the serviceInstanceSpec that
	// was last processed by the controller. The reconciled generation is updated
	// even if the controller failed to process the spec.
	ReconciledGeneration int64

	// OperationStartTime is the time at which the current operation began.
	OperationStartTime *metav1.Time

	// InProgressProperties is the properties state of the ServiceInstance when
	// a Provision or Update is in progress. If the current operation is a
	// Deprovision, this will be nil.
	InProgressProperties *ServiceInstancePropertiesState

	// ExternalProperties is the properties state of the ServiceInstance which the
	// broker knows about.
	ExternalProperties *ServiceInstancePropertiesState
}

// ServiceInstanceCondition contains condition information about an Instance.
type ServiceInstanceCondition struct {
	// Type of the condition, currently ('Ready').
	Type ServiceInstanceConditionType

	// Status of the condition, one of ('True', 'False', 'Unknown').
	Status ConditionStatus

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	LastTransitionTime metav1.Time

	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	Reason string

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	Message string
}

// ServiceInstanceConditionType represents a instance condition value.
type ServiceInstanceConditionType string

const (
	// ServiceInstanceConditionReady represents that a given InstanceCondition is in
	// ready state.
	ServiceInstanceConditionReady ServiceInstanceConditionType = "Ready"

	// ServiceInstanceConditionFailed represents information about a final failure
	// that should not be retried.
	ServiceInstanceConditionFailed ServiceInstanceConditionType = "Failed"
)

// ServiceInstanceOperation represents a type of operation the controller can
// be performing for a service instance in the OSB API.
type ServiceInstanceOperation string

const (
	// ServiceInstanceOperationProvision indicates that the ServiceInstance is
	// being Provisioned.
	ServiceInstanceOperationProvision ServiceInstanceOperation = "Provision"
	// ServiceInstanceOperationUpdate indicates that the ServiceInstance is
	// being Updated.
	ServiceInstanceOperationUpdate ServiceInstanceOperation = "Update"
	// ServiceInstanceOperationDeprovision indicates that the ServiceInstance is
	// being Deprovisioned.
	ServiceInstanceOperationDeprovision ServiceInstanceOperation = "Deprovision"
)

// ServiceInstancePropertiesState is the state of a ServiceInstance that
// the ServiceBroker knows about.
type ServiceInstancePropertiesState struct {
	// ExternalClusterServicePlanName is the name of the plan that the broker knows this
	// ServiceInstance to be on. This is the human readable plan name from the
	// OSB API.
	ExternalClusterServicePlanName string

	// Parameters is a blob of the parameters and their values that the broker
	// knows about for this ServiceInstance.  If a parameter was sourced from
	// a secret, its value will be "<redacted>" in this blob.
	Parameters *runtime.RawExtension

	// ParametersChecksum is the checksum of the parameters that were sent.
	ParametersChecksum string

	// UserInfo is information about the user that made the request.
	UserInfo *UserInfo
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceBindingList is a list of ServiceBindings.
type ServiceBindingList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []ServiceBinding
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceBinding represents a "used by" relationship between an application and an
// ServiceInstance.
type ServiceBinding struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   ServiceBindingSpec
	Status ServiceBindingStatus
}

// ServiceBindingSpec represents the desired state of a
// ServiceBinding.
//
// The spec field cannot be changed after a ServiceBinding is
// created.  Changes submitted to the spec field will be ignored.
type ServiceBindingSpec struct {
	// ServiceInstanceRef is the reference to the Instance this ServiceBinding is to.
	//
	// Immutable.
	ServiceInstanceRef v1.LocalObjectReference

	// Parameters is a set of the parameters to be passed to the underlying
	// broker. The inline YAML/JSON payload to be translated into equivalent
	// JSON object. If a top-level parameter name exists in multiples sources
	// among `Parameters` and `ParametersFrom` fields, it is considered to be
	// a user error in the specification.
	//
	// The Parameters field is NOT secret or secured in any way and should
	// NEVER be used to hold sensitive information. To set parameters that
	// contain secret information, you should ALWAYS store that information
	// in a Secret and use the ParametersFrom field.
	//
	// +optional
	Parameters *runtime.RawExtension

	// List of sources to populate parameters.
	// If a top-level parameter name exists in multiples sources among
	// `Parameters` and `ParametersFrom` fields, it is
	// considered to be a user error in the specification
	// +optional
	ParametersFrom []ParametersFromSource

	// SecretName is the name of the secret to create in the ServiceBinding's
	// namespace that will hold the credentials associated with the ServiceBinding.
	SecretName string

	// ExternalID is the identity of this object for use with the OSB API.
	//
	// Immutable.
	ExternalID string

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// UserInfo contains information about the user that last modified this
	// ServiceBinding. This field is set by the API server and not
	// settable by the end-user. User-provided values for this field are not saved.
	// +optional
	UserInfo *UserInfo
}

// ServiceBindingStatus represents the current status of a ServiceBinding.
type ServiceBindingStatus struct {
	Conditions []ServiceBindingCondition

	// CurrentOperation is the operation the Controller is currently performing
	// on the ServiceBinding.
	CurrentOperation ServiceBindingOperation

	// ReconciledGeneration is the 'Generation' of the
	// ServiceBindingSpec that was last processed by the controller.
	// The reconciled generation is updated even if the controller failed to
	// process the spec.
	ReconciledGeneration int64

	// OperationStartTime is the time at which the current operation began.
	OperationStartTime *metav1.Time

	// InProgressProperties is the properties state of the
	// ServiceBinding when a Bind is in progress. If the current
	// operation is an Unbind, this will be nil.
	InProgressProperties *ServiceBindingPropertiesState

	// ExternalProperties is the properties state of the
	// ServiceBinding which the broker knows about.
	ExternalProperties *ServiceBindingPropertiesState

	// OrphanMitigationInProgress is a flag that represents whether orphan
	// mitigation is in progress.
	OrphanMitigationInProgress bool
}

// ServiceBindingCondition condition information for a ServiceBinding.
type ServiceBindingCondition struct {
	// Type of the condition, currently ('Ready').
	Type ServiceBindingConditionType

	// Status of the condition, one of ('True', 'False', 'Unknown').
	Status ConditionStatus

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	LastTransitionTime metav1.Time

	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	Reason string

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	Message string
}

// ServiceBindingConditionType represents a ServiceBindingCondition value.
type ServiceBindingConditionType string

const (
	// ServiceBindingConditionReady represents a ServiceBindingCondition is in ready state.
	ServiceBindingConditionReady ServiceBindingConditionType = "Ready"

	// ServiceBindingConditionFailed represents a ServiceBindingCondition that has failed
	// completely and should not be retried.
	ServiceBindingConditionFailed ServiceBindingConditionType = "Failed"
)

// ServiceBindingOperation represents a type of operation
// the controller can be performing for a binding in the OSB API.
type ServiceBindingOperation string

const (
	// ServiceBindingOperationBind indicates that the
	// ServiceBinding is being bound.
	ServiceBindingOperationBind ServiceBindingOperation = "Bind"
	// ServiceBindingOperationUnbind indicates that the
	// ServiceBinding is being unbound.
	ServiceBindingOperationUnbind ServiceBindingOperation = "Unbind"
)

// These are internal finalizer values to service catalog, must be qualified name.
const (
	FinalizerServiceCatalog string = "kubernetes-incubator/service-catalog"
)

// ServiceBindingPropertiesState is the state of a
// ServiceBinding that the ServiceBroker knows about.
type ServiceBindingPropertiesState struct {
	// Parameters is a blob of the parameters and their values that the broker
	// knows about for this ServiceBinding.  If a parameter was
	// sourced from a secret, its value will be "<redacted>" in this blob.
	Parameters *runtime.RawExtension

	// ParametersChecksum is the checksum of the parameters that were sent.
	ParametersChecksum string

	// UserInfo is information about the user that made the request.
	UserInfo *UserInfo
}

// ParametersFromSource represents the source of a set of Parameters
type ParametersFromSource struct {
	// The Secret key to select from.
	// The value must be a JSON object.
	//+optional
	SecretKeyRef *SecretKeyReference
}

// SecretKeyReference references a key of a Secret.
type SecretKeyReference struct {
	// The name of the secret in the pod's namespace to select from.
	Name string
	// The key of the secret to select from.  Must be a valid secret key.
	Key string
}
