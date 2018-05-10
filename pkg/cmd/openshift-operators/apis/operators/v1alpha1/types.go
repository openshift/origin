package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ManagementState string

const (
	// Managed means that the operator is actively managing its resources and trying to keep the component active
	Managed ManagementState = "Managed"
	// Unmanaged means that the operator is not taking any action related to the component
	Unmanaged ManagementState = "Unmanaged"
	// Removed means that the operator is actively managing its resources and trying to remove all traces of the component
	Removed ManagementState = "Removed"
)

type OperatorSpec struct {
	// managementState indicates whether and how the operator should manage the component
	ManagementState ManagementState `json:"managementState"`

	// imagePullSpec is the image to use for the component.
	ImagePullSpec string `json:"imagePullSpec"`

	// version is the desired state in major.minor.micro-patch.  Usually patch is ignored.
	Version string `json:"version"`

	// logging contains glog parameters for the component pods.  It's always a command line arg for the moment
	Logging LoggingConfig `json:"logging,omitempty"`
}

type LoggingConfig struct {
	// level is passed to glog.
	Level int64 `json:"level"`

	// vmodule is passed to glog.
	Vmodule string `json:"vmodule"`
}

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"

	OperatorStatusTypeAvailable      = "Available"
	OperatorStatusTypeMigrating      = "Migrating"
	OperatorStatusTypeSyncSuccessful = "SyncSuccessful"
)

type OperatorCondition struct {
	Type               string          `json:"type"`
	Status             ConditionStatus `json:"status"`
	LastTransitionTime metav1.Time     `json:"lastTransitionTime,omitempty"`
	Reason             string          `json:"reason,omitempty"`
	Message            string          `json:"message,omitempty"`
}

// VersionAvailablity gives information about the synchronization and operational status of a particular version of the component
type VersionAvailablity struct {
	// version is the level this availability applies to
	Version string `json:"version"`
	// updatedReplicas indicates how many replicas are at the desired state
	UpdatedReplicas int32 `json:"updatedReplicas"`
	// readyReplicas indicates how many replicas are ready and at the desired state
	ReadyReplicas int32 `json:"readyReplicas"`
	// errors indicates what failures are associated with the operator trying to manage this version
	Errors []string `json:"errors"`
	// generations allows an operator to track what the generation of "important" resources was the last time we updated them
	Generations []GenerationHistory `json:"generations"`
}

type GenerationHistory struct {
	// group is the group of the thing you're tracking
	Group string `json:"group"`
	// resource is the resource type of the thing you're tracking
	Resource string `json:"resource"`
	// namespace is where the thing you're tracking is
	Namespace string `json:"namespace"`
	// name is the name of the thing you're tracking
	Name string `json:"name"`
	// lastGeneration is the last generation of the workload controller involved
	LastGeneration int64 `json:"lastGeneration"`
}

type OperatorStatus struct {
	// observedGeneration is the last generation change you've dealt with
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions is a list of conditions and their status
	Conditions []OperatorCondition `json:"conditions,omitempty"`

	// state indicates what the operator has observed to be its current operational status.
	State ManagementState `json:"state,omitempty"`
	// taskSummary is a high level summary of what the controller is currently attempting to do.  It is high-level, human-readable
	// and not guaranteed in any way. (I needed this for debugging and realized it made a great summary).
	TaskSummary string `json:"taskSummary,omitempty"`

	// currentVersionAvailability is availability information for the current version.  If it is unmanged or removed, this doesn't exist.
	CurrentAvailability *VersionAvailablity `json:"currentVersionAvailability,omitempty"`
	// targetVersionAvailability is availability information for the target version if we are migrating
	TargetAvailability *VersionAvailablity `json:"targetVersionAvailability,omitempty"`
}
