package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerManager holds cluster-wide configuration shared by the controller managers
// in the system, among them especially kube-controller-manager.
// The resource is a singleton named "cluster".
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2668
// +openshift:file-pattern=cvoRunLevel=0000_10,operatorName=config-operator,operatorOrdering=01
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=controllermanagers,scope=Cluster
// +openshift:enable:FeatureGate=ControllerManagerConfig
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="controllermanager is a singleton, .metadata.name must be 'cluster'"
type ControllerManager struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`
	// spec holds user settable values for configuration.
	// This field is required but the spec may be empty to express no opinion.
	// +required
	Spec *ControllerManagerSpec `json:"spec,omitempty,omitzero"`
}

// ControllerManagerSpec defines the desired state of the controller managers
// +kubebuilder:validation:MinProperties=0
type ControllerManagerSpec struct {
	// volumeForceDetach controls the conditions under which kube-controller-manager
	// force detaches volumes from a node that is not healthy.
	// Volumes are always force detached when a node is marked out-of-service via the
	// "node.kubernetes.io/out-of-service" taint, as part of the non-graceful node
	// shutdown procedure. This field controls whether force detach is also triggered
	// when the maximum unmount time is exceeded (6 minutes).
	// Valid values are "OnUnmountTimeout" and "OnOutOfServiceTaintOnly".
	// When set to "OnUnmountTimeout", volumes are force detached from unhealthy nodes
	// once the maximum unmount time is exceeded, so that workloads using them can start
	// on other nodes. Force detaching a volume that is still in use by the node can
	// corrupt its data.
	// When set to "OnOutOfServiceTaintOnly", volumes are force detached only via the
	// out-of-service taint, and remain attached to an unhealthy node until it recovers
	// or the "node.kubernetes.io/out-of-service" taint is applied as part of the
	// non-graceful node shutdown procedure.
	// When omitted, this means the user has no opinion and the platform is left
	// to choose a reasonable default, which is subject to change over time.
	// The current default is "OnUnmountTimeout".
	// Changing this field causes kube-controller-manager to be redeployed with the new setting.
	// Rollout progress is reported by the kube-controller-manager cluster operator.
	// +optional
	VolumeForceDetach VolumeForceDetachPolicy `json:"volumeForceDetach,omitempty"`
}

// VolumeForceDetachPolicy describes the conditions under which volumes are
// force detached from an unhealthy node.
// Valid values are "OnUnmountTimeout" and "OnOutOfServiceTaintOnly".
// +enum
// +kubebuilder:validation:Enum=OnUnmountTimeout;OnOutOfServiceTaintOnly
type VolumeForceDetachPolicy string

const (
	// VolumeForceDetachOnUnmountTimeout force detaches volumes from unhealthy nodes
	// once the maximum unmount time is exceeded.
	VolumeForceDetachOnUnmountTimeout VolumeForceDetachPolicy = "OnUnmountTimeout"
	// VolumeForceDetachOnOutOfServiceTaintOnly force detaches volumes only when
	// the node is marked out-of-service, never based on the maximum unmount time.
	VolumeForceDetachOnOutOfServiceTaintOnly VolumeForceDetachPolicy = "OnOutOfServiceTaintOnly"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerManagerList is a collection of ControllerManager resources.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type ControllerManagerList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	// items is a list of ControllerManager resources
	Items []ControllerManager `json:"items"`
}
