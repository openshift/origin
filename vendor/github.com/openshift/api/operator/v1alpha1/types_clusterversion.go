package v1alpha1

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterVersionOperator holds cluster-wide information about the Cluster Version Operator.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:file-pattern=cvoRunLevel=0000_00,operatorName=cluster-version-operator,operatorOrdering=01
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusterversionoperators,scope=Cluster
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2044
// +openshift:enable:FeatureGate=ClusterVersionOperatorConfiguration
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="ClusterVersionOperator is a singleton; the .metadata.name field must be 'cluster'"
type ClusterVersionOperator struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata"`

	// spec is the specification of the desired behavior of the Cluster Version Operator.
	// +required
	Spec ClusterVersionOperatorSpec `json:"spec"`

	// status is the most recently observed status of the Cluster Version Operator.
	// +optional
	Status ClusterVersionOperatorStatus `json:"status"`
}

// ClusterVersionOperatorSpec is the specification of the desired behavior of the Cluster Version Operator.
type ClusterVersionOperatorSpec struct {
	// operatorLogLevel is an intent based logging for the operator itself.  It does not give fine grained control, but it is a
	// simple way to manage coarse grained logging choices that operators have to interpret for themselves.
	//
	// Valid values are: "Normal", "Debug", "Trace", "TraceAll".
	// Defaults to "Normal".
	// +optional
	// +kubebuilder:default=Normal
	OperatorLogLevel operatorv1.LogLevel `json:"operatorLogLevel,omitempty"`
}

// ClusterVersionOperatorStatus defines the observed status of the Cluster Version Operator.
type ClusterVersionOperatorStatus struct {
	// observedGeneration represents the most recent generation observed by the operator and specifies the version of
	// the spec field currently being synced.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self >= oldSelf",message="observedGeneration must only increase"
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterVersionOperatorList is a collection of ClusterVersionOperators.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type ClusterVersionOperatorList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	// items is a list of ClusterVersionOperators.
	// +optional
	Items []ClusterVersionOperator `json:"items,omitempty"`
}
