package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
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

// OperatorSpec contains common fields for an operator to need.  It is intended to be anonymous included
// inside of the Spec struct for you particular operator.
type OperatorSpec struct {
	// managementState indicates whether and how the operator should manage the component
	ManagementState ManagementState `json:"managementState" protobuf:"bytes,1,opt,name=managementState,casttype=ManagementState"`

	// imagePullSpec is the image to use for the component.
	ImagePullSpec string `json:"imagePullSpec" protobuf:"bytes,2,opt,name=imagePullSpec"`

	// version is the desired state in major.minor.micro-patch.  Usually patch is ignored.
	Version string `json:"version" protobuf:"bytes,3,opt,name=version"`

	// logging contains glog parameters for the component pods.  It's always a command line arg for the moment
	Logging LoggingConfig `json:"logging,omitempty" protobuf:"bytes,4,opt,name=logging"`
}

// LoggingConfig holds information about configuring logging
type LoggingConfig struct {
	// level is passed to glog.
	Level int64 `json:"level" protobuf:"varint,1,opt,name=level"`

	// vmodule is passed to glog.
	Vmodule string `json:"vmodule" protobuf:"bytes,2,opt,name=vmodule"`
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

// OperatorCondition is just the standard condition fields.
type OperatorCondition struct {
	Type               string          `json:"type" protobuf:"bytes,1,opt,name=type"`
	Status             ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=ConditionStatus"`
	LastTransitionTime metav1.Time     `json:"lastTransitionTime,omitempty" protobuf:"bytes,3,opt,name=lastTransitionTime"`
	Reason             string          `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`
	Message            string          `json:"message,omitempty" protobuf:"bytes,5,opt,name=message"`
}

// VersionAvailablity gives information about the synchronization and operational status of a particular version of the component
type VersionAvailablity struct {
	// version is the level this availability applies to
	Version string `json:"version" protobuf:"bytes,1,opt,name=version"`
	// updatedReplicas indicates how many replicas are at the desired state
	UpdatedReplicas int32 `json:"updatedReplicas" protobuf:"varint,2,opt,name=updatedReplicas"`
	// readyReplicas indicates how many replicas are ready and at the desired state
	ReadyReplicas int32 `json:"readyReplicas" protobuf:"varint,3,opt,name=readyReplicas"`
	// errors indicates what failures are associated with the operator trying to manage this version
	Errors []string `json:"errors" protobuf:"bytes,4,rep,name=errors"`
	// generations allows an operator to track what the generation of "important" resources was the last time we updated them
	Generations []GenerationHistory `json:"generations" protobuf:"bytes,5,rep,name=generations"`
}

// GenerationHistory keeps track of the generation for a given resource so that decisions about forced updated can be made.
type GenerationHistory struct {
	// group is the group of the thing you're tracking
	Group string `json:"group" protobuf:"bytes,1,opt,name=group"`
	// resource is the resource type of the thing you're tracking
	Resource string `json:"resource" protobuf:"bytes,2,opt,name=resource"`
	// namespace is where the thing you're tracking is
	Namespace string `json:"namespace" protobuf:"bytes,3,opt,name=namespace"`
	// name is the name of the thing you're tracking
	Name string `json:"name" protobuf:"bytes,4,opt,name=name"`
	// lastGeneration is the last generation of the workload controller involved
	LastGeneration int64 `json:"lastGeneration" protobuf:"varint,5,opt,name=lastGeneration"`
}

// OperatorStatus contains common fields for an operator to need.  It is intended to be anonymous included
// inside of the Status struct for you particular operator.
type OperatorStatus struct {
	// observedGeneration is the last generation change you've dealt with
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,1,opt,name=observedGeneration"`

	// conditions is a list of conditions and their status
	Conditions []OperatorCondition `json:"conditions,omitempty" protobuf:"bytes,2,rep,name=conditions"`

	// state indicates what the operator has observed to be its current operational status.
	State ManagementState `json:"state,omitempty" protobuf:"bytes,3,opt,name=state,casttype=ManagementState"`
	// taskSummary is a high level summary of what the controller is currently attempting to do.  It is high-level, human-readable
	// and not guaranteed in any way. (I needed this for debugging and realized it made a great summary).
	TaskSummary string `json:"taskSummary,omitempty" protobuf:"bytes,4,opt,name=taskSummary"`

	// currentVersionAvailability is availability information for the current version.  If it is unmanged or removed, this doesn't exist.
	CurrentAvailability *VersionAvailablity `json:"currentVersionAvailability,omitempty" protobuf:"bytes,5,opt,name=currentVersionAvailability"`
	// targetVersionAvailability is availability information for the target version if we are migrating
	TargetAvailability *VersionAvailablity `json:"targetVersionAvailability,omitempty" protobuf:"bytes,6,opt,name=targetVersionAvailability"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GenericOperatorConfig provides information to configure an operator
type GenericOperatorConfig struct {
	metav1.TypeMeta `json:",inline"`

	// ServingInfo is the HTTP serving information for the controller's endpoints
	ServingInfo configv1.HTTPServingInfo `json:"servingInfo,omitempty" protobuf:"bytes,1,opt,name=servingInfo"`

	// leaderElection provides information to elect a leader. Only override this if you have a specific need
	LeaderElection configv1.LeaderElection `json:"leaderElection,omitempty" protobuf:"bytes,2,opt,name=leaderElection"`

	// authentication allows configuration of authentication for the endpoints
	Authentication DelegatedAuthentication `json:"authentication,omitempty" protobuf:"bytes,3,opt,name=authentication"`
	// authorization allows configuration of authentication for the endpoints
	Authorization DelegatedAuthorization `json:"authorization,omitempty" protobuf:"bytes,4,opt,name=authorization"`
}

// DelegatedAuthentication allows authentication to be disabled.
type DelegatedAuthentication struct {
	// disabled indicates that authentication should be disabled.  By default it will use delegated authentication.
	Disabled bool `json:"disabled,omitempty" protobuf:"varint,1,opt,name=disabled"`
}

// DelegatedAuthorization allows authorization to be disabled.
type DelegatedAuthorization struct {
	// disabled indicates that authorization should be disabled.  By default it will use delegated authorization.
	Disabled bool `json:"disabled,omitempty" protobuf:"varint,1,opt,name=disabled"`
}
