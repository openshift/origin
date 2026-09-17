package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Ingress contains configuration options specific to the Ingress Operator itself,
// including how it manages Gateway API integration.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:file-pattern=cvoRunLevel=0000_50,operatorName=ingress,operatorOrdering=02
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ingresses,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2890
// +openshift:capability=Ingress
// +openshift:enable:FeatureGate=GatewayAPIManagementMode
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="Ingress is a singleton; the .metadata.name field must be 'cluster'"
type Ingress struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +required
	metav1.ObjectMeta `json:"metadata"`

	// spec holds user settable values for configuration.
	// +required
	Spec IngressSpec `json:"spec,omitzero"`

	// status holds observed values from the cluster.
	// +optional
	Status IngressStatus `json:"status,omitzero"`
}

// IngressSpec is the specification of the desired behavior of the Ingress Operator.
// +kubebuilder:validation:MinProperties=1
type IngressSpec struct {
	// gatewayAPI holds configuration for Gateway API integration, including how the
	// ingress operator manages Gateway API CRDs, the OpenShift Gateway API
	// implementation, and its Gateway API controllers.
	//
	// +optional
	GatewayAPI GatewayAPIIngressConfig `json:"gatewayAPI,omitzero"`
}

// IngressStatus defines the observed status of the Ingress Operator.
// +kubebuilder:validation:MinProperties=1
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.observedGeneration) || (has(self.observedGeneration) && self.observedGeneration >= oldSelf.observedGeneration)",message="observedGeneration must remain set and only increase once set"
type IngressStatus struct {
	// conditions is a list of conditions and their status.
	//
	// Gateway API CRD management conditions are reported here with the "GatewayAPI" prefix:
	//
	// * "GatewayAPICRDsManaged" indicates whether the ingress operator is actively
	//   managing Gateway API CRDs.
	// * "GatewayAPICRDsPresent" indicates whether Gateway API CRDs exist on the
	//   cluster.
	// * "GatewayAPICRDsCompliant" indicates whether the installed CRDs match the
	//   version expected by this ingress operator release.
	//
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration represents the most recent generation observed by the operator and specifies the version of
	// the spec field currently being synced.
	//
	// When omitted, the operator has not yet observed the resource.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// GatewayAPIIngressConfig holds configuration for Gateway API integration in the
// Cluster Ingress Operator.
// +kubebuilder:validation:MinProperties=1
type GatewayAPIIngressConfig struct {
	// managementMode specifies how the Cluster Ingress Operator manages Gateway API
	// Custom Resource Definitions (CRDs), the OpenShift Gateway API implementation,
	// and its Gateway API controllers.
	//
	// Allowed values are "Managed" and "Unmanaged".
	//
	// When omitted or set to "Managed", the ingress operator installs, owns, and
	// upgrades the Gateway API CRDs, protects them with a Validating Admission
	// Policy, and deploys the OpenShift Gateway API implementation and its Gateway
	// API controllers.
	//
	// When set to "Unmanaged", the ingress operator does not install or manage
	// Gateway API CRDs and does not deploy the OpenShift Gateway API implementation
	// or its Gateway API controllers. The cluster administrator or a third-party
	// product is responsible for providing their own CRDs and Gateway controller.
	// The ingress operator reports observational status only.
	//
	// +optional
	ManagementMode GatewayAPIManagementMode `json:"managementMode,omitempty"`
}

// GatewayAPIManagementMode describes how the Cluster Ingress Operator manages
// Gateway API Custom Resource Definitions.
// +kubebuilder:validation:Enum=Managed;Unmanaged
type GatewayAPIManagementMode string

const (
	// GatewayAPIManagementModeManaged means the ingress operator installs, owns,
	// protects (via a Validating Admission Policy), and upgrades the Gateway API
	// CRDs, deploys the OpenShift Gateway API implementation, and runs its Gateway
	// API controllers. This is the default mode and the only fully supported
	// configuration.
	GatewayAPIManagementModeManaged GatewayAPIManagementMode = "Managed"

	// GatewayAPIManagementModeUnmanaged means the ingress operator does not
	// install or manage Gateway API CRDs, does not deploy the OpenShift Gateway
	// API implementation, and does not run its Gateway API controllers. The
	// cluster administrator or a third-party product is responsible for bringing
	// their own CRDs and Gateway controller. The ingress operator reports
	// observational status only.
	GatewayAPIManagementModeUnmanaged GatewayAPIManagementMode = "Unmanaged"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IngressList is a collection of Ingresses.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type IngressList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	// items is a list of Ingresses.
	// +optional
	Items []Ingress `json:"items,omitempty"`
}
