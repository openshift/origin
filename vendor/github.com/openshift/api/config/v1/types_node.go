package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Node holds cluster-wide information about node specific features.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type Node struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +kubebuilder:validation:Required
	// +required
	Spec NodeSpec `json:"spec"`

	// status holds observed values.
	// +optional
	Status NodeStatus `json:"status"`
}

type NodeSpec struct {
	// CgroupMode determines the cgroups version on the node
	// +optional
	CgroupMode CgroupMode `json:"cgroupMode,omitempty"`

	// WorkerLatencyProfile determins the how fast the kubelet is updating
	// the status and corresponding reaction of the cluster
	// +optional
	WorkerLatencyProfile WorkerLatencyProfileType `json:"workerLatencyProfile,omitempty"`
}

type NodeStatus struct {
	// WorkerLatencyProfileStatus provides the current status of WorkerLatencyProfile
	// +optional
	WorkerLatencyProfileStatus WorkerLatencyProfileStatus `json:"workerLatencyProfileStatus,omitempty"`
}

type CgroupMode string

const (
	CgroupModeEmpty   CgroupMode = "" // Empty string indicates to honor user set value on the system that should not be overridden by OpenShift
	CgroupModeV1      CgroupMode = "v1"
	CgroupModeV2      CgroupMode = "v2"
	CgroupModeDefault CgroupMode = CgroupModeV1
)

type WorkerLatencyProfileType string

const (
	// Medium Kubelet Update Frequency (heart-beat) and Average Reaction Time to unresponsive Node
	MediumUpdateAverageReaction WorkerLatencyProfileType = "MediumUpdateAverageReaction"

	// Low Kubelet Update Frequency (heart-beat) and Slow Reaction Time to unresponsive Node
	LowUpdateSlowReaction WorkerLatencyProfileType = "LowUpdateSlowReaction"

	// Default values of relavent Kubelet, Kube Controller Manager and Kube API Server
	DefaultUpdateDefaultReaction WorkerLatencyProfileType = "Default"
)

// WorkerLatencyProfileStatus provides status information about the WorkerLatencyProfile rollout
type WorkerLatencyProfileStatus struct {
	// conditions describes the state of the WorkerLatencyProfile and related components
	// (Kubelet or Controller Manager or Kube API Server)
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []WorkerLatencyStatusCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// WorkerLatencyStatusConditionType is an aspect of WorkerLatencyProfile state.
type WorkerLatencyStatusConditionType string

const (
	// Progressing indicates that the updates to component (Kubelet or Controller
	// Manager or Kube API Server) is actively rolling out, propagating changes to the
	// respective arguments.
	WorkerLatencyProfileProgressing WorkerLatencyStatusConditionType = "Progressing"

	// Complete indicates whether the component (Kubelet or Controller Manager or Kube API Server)
	// is successfully updated the respective arguments.
	WorkerLatencyProfileComplete WorkerLatencyStatusConditionType = "Complete"

	// Degraded indicates that the component (Kubelet or Controller Manager or Kube API Server)
	// does not reach the state 'Complete' over a period of time
	// resulting in either a lower quality or absence of service.
	// If the component enters in this state, "Default" WorkerLatencyProfileType
	// rollout will be initiated to restore the respective default arguments of all
	// components.
	WorkerLatencyProfileDegraded WorkerLatencyStatusConditionType = "Degraded"
)

type WorkerLatencyStatusConditionOwner string

const (
	// Machine Config Operator will update condition status by setting this as owner
	MachineConfigOperator WorkerLatencyStatusConditionOwner = "MachineConfigOperator"

	// Kube Controller Manager Operator will update condition status  by setting this as owner
	KubeControllerManagerOperator WorkerLatencyStatusConditionOwner = "KubeControllerManagerOperator"

	// Kube API Server Operator will update condition status by setting this as owner
	KubeAPIServerOperator WorkerLatencyStatusConditionOwner = "KubeAPIServerOperator"
)

type WorkerLatencyStatusCondition struct {
	// Owner specifies the operator that is updating this condition
	// +kubebuilder:validation:Required
	// +required
	Owner WorkerLatencyStatusConditionOwner `json:"owner"`

	// type specifies the aspect reported by this condition.
	// +kubebuilder:validation:Required
	// +required
	Type WorkerLatencyStatusConditionType `json:"type"`

	// status of the condition, one of True, False, Unknown.
	// +kubebuilder:validation:Required
	// +required
	Status ConditionStatus `json:"status"`

	// lastTransitionTime is the time of the last update to the current status property.
	// +kubebuilder:validation:Required
	// +required
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// reason is the CamelCase reason for the condition's current status.
	// +optional
	Reason string `json:"reason,omitempty"`

	// message provides additional information about the current condition.
	// This is only to be consumed by humans.  It may contain Line Feed
	// characters (U+000A), which should be rendered as new lines.
	// +optional
	Message string `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type NodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Node `json:"items"`
}
