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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/pkg/api/v1"
)

// +genclient=true
// +nonNamespaced=true

// ServiceBroker represents an entity that provides ServiceClasses for use in the
// service catalog.
type ServiceBroker struct {
	metav1.TypeMeta `json:",inline"`
	// Non-namespaced.  The name of this resource in etcd is in ObjectMeta.Name.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceBrokerSpec   `json:"spec"`
	Status ServiceBrokerStatus `json:"status"`
}

// ServiceBrokerList is a list of Brokers.
type ServiceBrokerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ServiceBroker `json:"items"`
}

// ServiceBrokerSpec represents a description of a Broker.
type ServiceBrokerSpec struct {
	// URL is the address used to communicate with the ServiceBroker.
	URL string `json:"url"`

	// AuthInfo contains the data that the service catalog should use to authenticate
	// with the ServiceBroker.
	AuthInfo *ServiceBrokerAuthInfo `json:"authInfo,omitempty"`

	// InsecureSkipTLSVerify disables TLS certificate verification when communicating with this Broker.
	// This is strongly discouraged.  You should use the CABundle instead.
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`
	// CABundle is a PEM encoded CA bundle which will be used to validate a Broker's serving certificate.
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// RelistBehavior specifies the type of relist behavior the catalog should
	// exhibit when relisting ServiceClasses available from a broker.
	RelistBehavior ServiceBrokerRelistBehavior `json:"relistBehavior"`

	// RelistDuration is the frequency by which a controller will relist the
	// broker when the RelistBehavior is set to ServiceBrokerRelistBehaviorDuration.
	RelistDuration *metav1.Duration `json:"relistDuration"`

	// RelistRequests is a strictly increasing, non-negative integer counter that
	// can be manually incremented by a user to manually trigger a relist.
	RelistRequests int64 `json:"relistRequests"`
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
	Basic *BasicAuthConfig `json:"basic,omitempty"`
	// BearerTokenAuthConfig provides configuration to send an opaque value as a bearer token.
	// The value is referenced from the 'token' field of the given secret.  This value should only
	// contain the token value and not the `Bearer` scheme.
	Bearer *BearerTokenAuthConfig `json:"bearer,omitempty"`

	// DEPRECATED: use `Basic` field for configuring basic authentication instead.
	// BasicAuthSecret is a reference to a Secret containing auth information the
	// catalog should use to authenticate to this ServiceBroker using basic auth.
	BasicAuthSecret *v1.ObjectReference `json:"basicAuthSecret,omitempty"`
}

// BasicAuthConfig provides config for the basic authentication.
type BasicAuthConfig struct {
	// SecretRef is a reference to a Secret containing information the
	// catalog should use to authenticate to this ServiceBroker.
	//
	// Required at least one of the fields:
	// - Secret.Data["username"] - username used for authentication
	// - Secret.Data["password"] - password or token needed for authentication
	SecretRef *v1.ObjectReference `json:"secretRef,omitempty"`
}

// BearerTokenAuthConfig provides config for the bearer token authentication.
type BearerTokenAuthConfig struct {
	// SecretRef is a reference to a Secret containing information the
	// catalog should use to authenticate to this ServiceBroker.
	//
	// Required field:
	// - Secret.Data["token"] - bearer token for authentication
	SecretRef *v1.ObjectReference `json:"secretRef,omitempty"`
}

const (
	// BasicAuthUsernameKey is the key of the username for SecretTypeBasicAuth secrets
	BasicAuthUsernameKey = "username"
	// BasicAuthPasswordKey is the key of the password or token for SecretTypeBasicAuth secrets
	BasicAuthPasswordKey = "password"

	// BearerTokenKey is the key of the bearer token for SecretTypeBearerTokenAuth secrets
	BearerTokenKey = "token"
)

// ServiceBrokerStatus represents the current status of a Broker.
type ServiceBrokerStatus struct {
	Conditions []ServiceBrokerCondition `json:"conditions"`

	// ReconciledGeneration is the 'Generation' of the serviceBrokerSpec that
	// was last processed by the controller. The reconciled generation is updated
	// even if the controller failed to process the spec.
	ReconciledGeneration int64 `json:"reconciledGeneration"`

	// OperationStartTime is the time at which the current operation began.
	OperationStartTime *metav1.Time `json:"operationStartTime,omitempty"`
}

// ServiceBrokerCondition contains condition information for a Broker.
type ServiceBrokerCondition struct {
	// Type of the condition, currently ('Ready').
	Type ServiceBrokerConditionType `json:"type"`

	// Status of the condition, one of ('True', 'False', 'Unknown').
	Status ConditionStatus `json:"status"`

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	Reason string `json:"reason"`

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	Message string `json:"message"`
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

// ServiceClassList is a list of ServiceClasses.
type ServiceClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ServiceClass `json:"items"`
}

// +genclient=true
// +nonNamespaced=true

// ServiceClass represents an offering in the service catalog.
type ServiceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// ServiceBrokerName is the reference to the Broker that provides this
	// ServiceClass.
	//
	// Immutable.
	ServiceBrokerName string `json:"brokerName"`

	// Description is a short description of this ServiceClass.
	Description string `json:"description"`

	// Bindable indicates whether a user can create bindings to an ServiceInstance
	// provisioned from this service. ServicePlan has an optional field called
	// Bindable which overrides the value of this field.
	Bindable bool `json:"bindable"`

	// Plans is the list of ServicePlans for this ServiceClass.  All
	// ServiceClasses have at least one ServicePlan.
	Plans []ServicePlan `json:"plans"`

	// PlanUpdatable indicates whether instances provisioned from this
	// ServiceClass may change ServicePlans after being provisioned.
	PlanUpdatable bool `json:"planUpdatable"`

	// ExternalID is the identity of this object for use with the OSB API.
	//
	// Immutable.
	ExternalID string `json:"externalID"`

	// ExternalMetadata is a blob of information about the ServiceClass, meant
	// to be user-facing content and display instructions.  This field may
	// contain platform-specific conventional values.
	ExternalMetadata *runtime.RawExtension `json:"externalMetadata, omitempty"`

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// Tags is a list of strings that represent different classification
	// attributes of the ServiceClass.  These are used in Cloud Foundry in a
	// way similar to Kubernetes labels, but they currently have no special
	// meaning in Kubernetes.
	Tags []string `json:"tags,omitempty"`

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// Requires exposes a list of Cloud Foundry-specific 'permissions'
	// that must be granted to an instance of this service within Cloud
	// Foundry.  These 'permissions' have no meaning within Kubernetes and an
	// ServiceInstance provisioned from this ServiceClass will not work correctly.
	Requires []string `json:"requires,omitempty"`
}

// ServicePlan represents a tier of a ServiceClass.
type ServicePlan struct {
	// Name is the CLI-friendly name of this ServicePlan.
	Name string `json:"name"`

	// ExternalID is the identity of this object for use with the OSB API.
	//
	// Immutable.
	ExternalID string `json:"externalID"`

	// Description is a short description of this ServicePlan.
	Description string `json:"description"`

	// Bindable indicates whether a user can create bindings to an ServiceInstance
	// using this ServicePlan.  If set, overrides the value of the
	// ServiceClass.Bindable field.
	Bindable *bool `json:"bindable,omitempty"`

	// Free indicates whether this plan is available at no cost.
	Free bool `json:"free"`

	// ExternalMetadata is a blob of information about the plan, meant to be
	// user-facing content and display instructions.  This field may contain
	// platform-specific conventional values.
	ExternalMetadata *runtime.RawExtension `json:"externalMetadata, omitempty"`

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// ServiceInstanceCreateParameterSchema is the schema for the parameters
	// that may be supplied when provisioning a new ServiceInstance on this plan.
	ServiceInstanceCreateParameterSchema *runtime.RawExtension `json:"instanceCreateParameterSchema,omitempty"`

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// ServiceInstanceUpdateParameterSchema is the schema for the parameters
	// that may be updated once an ServiceInstance has been provisioned on this plan.
	// This field only has meaning if the ServiceClass is PlanUpdatable.
	ServiceInstanceUpdateParameterSchema *runtime.RawExtension `json:"instanceUpdateParameterSchema,omitempty"`

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// ServiceInstanceCredentialCreateParameterSchema is the schema for the parameters that
	// may be supplied binding to an ServiceInstance on this plan.
	ServiceInstanceCredentialCreateParameterSchema *runtime.RawExtension `json:"serviceInstanceCredentialCreateParameterSchema,omitempty"`
}

// ServiceInstanceList is a list of instances.
type ServiceInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ServiceInstance `json:"items"`
}

// UserInfo holds information about the user that last changed a resource's spec.
type UserInfo struct {
	Username string                `json:"username"`
	UID      string                `json:"uid"`
	Groups   []string              `json:"groups,omitempty"`
	Extra    map[string]ExtraValue `json:"extra,omitempty"`
}

// ExtraValue contains additional information about a user that may be
// provided by the authenticator.
type ExtraValue []string

// +genclient=true

// ServiceInstance represents a provisioned instance of a ServiceClass.
// Currently, the spec field cannot be changed once a ServiceInstance is
// created.  Spec changes submitted by users will be ignored.
//
// In the future, this will be allowed and will represent the intention that
// the ServiceInstance should have the plan and/or parameters updated at the
// ServiceBroker.
type ServiceInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceInstanceSpec   `json:"spec"`
	Status ServiceInstanceStatus `json:"status"`
}

// ServiceInstanceSpec represents the desired state of an Instance.
type ServiceInstanceSpec struct {
	// ServiceClassName is the reference to the ServiceClass this ServiceInstance
	// should be provisioned from.
	//
	// Immutable.
	ServiceClassName string `json:"serviceClassName"`

	// PlanName is the name of the ServicePlan this ServiceInstance should be
	// provisioned from.
	// If omitted and there is only one plan in the specified ServiceClass
	// it will be used.
	// If omitted and there are more than one plan in the specified ServiceClass
	// the request will be rejected.
	PlanName string `json:"planName,omitempty"`

	// Parameters is a set of the parameters to be
	// passed to the underlying broker.
	// The inline YAML/JSON payload to be translated into equivalent
	// JSON object.
	// If a top-level parameter name exists in multiples sources among
	// `Parameters` and `ParametersFrom` fields, it is
	// considered to be a user error in the specification
	// +optional
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`

	// List of sources to populate parameters.
	// If a top-level parameter name exists in multiples sources among
	// `Parameters` and `ParametersFrom` fields, it is
	// considered to be a user error in the specification
	// +optional
	ParametersFrom []ParametersFromSource `json:"parametersFrom,omitempty"`

	// ExternalID is the identity of this object for use with the OSB SB API.
	//
	// Immutable.
	ExternalID string `json:"externalID"`

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// UserInfo contains information about the user that last modified this
	// instance. This field is set by the API server and not settable by the
	// end-user. User-provided values for this field are not saved.
	// +optional
	UserInfo *UserInfo `json:"userInfo"`
}

// ServiceInstanceStatus represents the current status of an Instance.
type ServiceInstanceStatus struct {
	// Conditions is an array of ServiceInstanceConditions capturing aspects of an
	// ServiceInstance's status.
	Conditions []ServiceInstanceCondition `json:"conditions"`

	// AsyncOpInProgress is set to true if there is an ongoing async operation
	// against this Service Instance in progress.
	AsyncOpInProgress bool `json:"asyncOpInProgress"`

	// LastOperation is the string that the broker may have returned when
	// an async operation started, it should be sent back to the broker
	// on poll requests as a query param.
	LastOperation *string `json:"lastOperation,omitempty"`

	// DashboardURL is the URL of a web-based management user interface for
	// the service instance.
	DashboardURL *string `json:"dashboardURL,omitempty"`

	// ReconciledGeneration is the 'Generation' of the serviceInstanceSpec that
	// was last processed by the controller. The reconciled generation is updated
	// even if the controller failed to process the spec.
	ReconciledGeneration int64 `json:"reconciledGeneration"`

	// OperationStartTime is the time at which the current operation began.
	OperationStartTime *metav1.Time `json:"operationStartTime,omitempty"`
}

// ServiceInstanceCondition contains condition information about an Instance.
type ServiceInstanceCondition struct {
	// Type of the condition, currently ('Ready').
	Type ServiceInstanceConditionType `json:"type"`

	// Status of the condition, one of ('True', 'False', 'Unknown').
	Status ConditionStatus `json:"status"`

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	Reason string `json:"reason"`

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	Message string `json:"message"`
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

// ServiceInstanceCredentialList is a list of ServiceInstanceCredentials.
type ServiceInstanceCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ServiceInstanceCredential `json:"items"`
}

// +genclient=true

// ServiceInstanceCredential represents a "used by" relationship between an application and an
// ServiceInstance.
type ServiceInstanceCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceInstanceCredentialSpec   `json:"spec"`
	Status ServiceInstanceCredentialStatus `json:"status"`
}

// ServiceInstanceCredentialSpec represents the desired state of a
// ServiceInstanceCredential.
//
// The spec field cannot be changed after a ServiceInstanceCredential is
// created.  Changes submitted to the spec field will be ignored.
type ServiceInstanceCredentialSpec struct {
	// ServiceInstanceRef is the reference to the Instance this ServiceInstanceCredential is to.
	//
	// Immutable.
	ServiceInstanceRef v1.LocalObjectReference `json:"instanceRef"`

	// Parameters is a set of the parameters to be
	// passed to the underlying broker.
	// The inline YAML/JSON payload to be translated into equivalent
	// JSON object.
	// If a top-level parameter name exists in multiples sources among
	// `Parameters` and `ParametersFrom` fields, it is
	// considered to be a user error in the specification
	// +optional
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`

	// List of sources to populate parameters.
	// If a top-level parameter name exists in multiples sources among
	// `Parameters` and `ParametersFrom` fields, it is
	// considered to be a user error in the specification
	// +optional
	ParametersFrom []ParametersFromSource `json:"parametersFrom,omitempty"`

	// SecretName is the name of the secret to create in the ServiceInstanceCredential's
	// namespace that will hold the credentials associated with the ServiceInstanceCredential.
	SecretName string `json:"secretName,omitempty"`

	// ExternalID is the identity of this object for use with the OSB API.
	//
	// Immutable.
	ExternalID string `json:"externalID"`

	// Currently, this field is ALPHA: it may change or disappear at any time
	// and its data will not be migrated.
	//
	// UserInfo contains information about the user that last modified this
	// ServiceInstanceCredential. This field is set by the API server and not
	// settable by the end-user. User-provided values for this field are not saved.
	// +optional
	UserInfo *UserInfo `json:"userInfo"`
}

// ServiceInstanceCredentialStatus represents the current status of a ServiceInstanceCredential.
type ServiceInstanceCredentialStatus struct {
	Conditions []ServiceInstanceCredentialCondition `json:"conditions"`

	// ReconciledGeneration is the 'Generation' of the
	// serviceInstanceCredentialSpec that was last processed by the controller.
	// The reconciled generation is updated even if the controller failed to
	// process the spec.
	ReconciledGeneration int64 `json:"reconciledGeneration"`

	// OperationStartTime is the time at which the current operation began.
	OperationStartTime *metav1.Time `json:"operationStartTime,omitempty"`
}

// ServiceInstanceCredentialCondition condition information for a ServiceInstanceCredential.
type ServiceInstanceCredentialCondition struct {
	// Type of the condition, currently ('Ready').
	Type ServiceInstanceCredentialConditionType `json:"type"`

	// Status of the condition, one of ('True', 'False', 'Unknown').
	Status ConditionStatus `json:"status"`

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	Reason string `json:"reason"`

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	Message string `json:"message"`
}

// ServiceInstanceCredentialConditionType represents a ServiceInstanceCredentialCondition value.
type ServiceInstanceCredentialConditionType string

const (
	// ServiceInstanceCredentialConditionReady represents a binding condition is in ready state.
	ServiceInstanceCredentialConditionReady ServiceInstanceCredentialConditionType = "Ready"

	// ServiceInstanceCredentialConditionFailed represents a ServiceInstanceCredentialCondition that has failed
	// completely and should not be retried.
	ServiceInstanceCredentialConditionFailed ServiceInstanceCredentialConditionType = "Failed"
)

// These are external finalizer values to service catalog, must be qualified name.
const (
	FinalizerServiceCatalog string = "kubernetes-incubator/service-catalog"
)

// ParametersFromSource represents the source of a set of Parameters
type ParametersFromSource struct {
	// The Secret key to select from.
	// The value must be a JSON object.
	//+optional
	SecretKeyRef *SecretKeyReference `json:"secretKeyRef,omitempty"`
}

// SecretKeyReference references a key of a Secret.
type SecretKeyReference struct {
	// The name of the secret in the pod's namespace to select from.
	Name string `json:"name"`
	// The key of the secret to select from.  Must be a valid secret key.
	Key string `json:"key"`
}
