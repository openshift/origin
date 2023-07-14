package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

const (
	// MachineFinalizer is set on PrepareForCreate callback.
	MachineFinalizer = "machine.machine.openshift.io"

	// MachineClusterLabelName is the label set on machines linked to a cluster.
	MachineClusterLabelName = "cluster.k8s.io/cluster-name"

	// MachineClusterIDLabel is the label that a machine must have to identify the
	// cluster to which it belongs.
	MachineClusterIDLabel = "machine.openshift.io/cluster-api-cluster"

	// IPClaimProtectionFinalizer is placed on an IPAddressClaim by the machine reconciler
	// when an IPAddressClaim associated with a machine is created. This finalizer is removed
	// from the IPAddressClaim when the associated machine is deleted.
	IPClaimProtectionFinalizer = "machine.openshift.io/ip-claim-protection"
)

type MachineStatusError string

const (
	// Represents that the combination of configuration in the MachineSpec
	// is not supported by this cluster. This is not a transient error, but
	// indicates a state that must be fixed before progress can be made.
	//
	// Example: the ProviderSpec specifies an instance type that doesn't exist,
	InvalidConfigurationMachineError MachineStatusError = "InvalidConfiguration"

	// This indicates that the MachineSpec has been updated in a way that
	// is not supported for reconciliation on this cluster. The spec may be
	// completely valid from a configuration standpoint, but the controller
	// does not support changing the real world state to match the new
	// spec.
	//
	// Example: the responsible controller is not capable of changing the
	// container runtime from docker to rkt.
	UnsupportedChangeMachineError MachineStatusError = "UnsupportedChange"

	// This generally refers to exceeding one's quota in a cloud provider,
	// or running out of physical machines in an on-premise environment.
	InsufficientResourcesMachineError MachineStatusError = "InsufficientResources"

	// There was an error while trying to create a Node to match this
	// Machine. This may indicate a transient problem that will be fixed
	// automatically with time, such as a service outage, or a terminal
	// error during creation that doesn't match a more specific
	// MachineStatusError value.
	//
	// Example: timeout trying to connect to GCE.
	CreateMachineError MachineStatusError = "CreateError"

	// There was an error while trying to update a Node that this
	// Machine represents. This may indicate a transient problem that will be
	// fixed automatically with time, such as a service outage,
	//
	// Example: error updating load balancers
	UpdateMachineError MachineStatusError = "UpdateError"

	// An error was encountered while trying to delete the Node that this
	// Machine represents. This could be a transient or terminal error, but
	// will only be observable if the provider's Machine controller has
	// added a finalizer to the object to more gracefully handle deletions.
	//
	// Example: cannot resolve EC2 IP address.
	DeleteMachineError MachineStatusError = "DeleteError"

	// TemplateClonedFromGroupKindAnnotation is the infrastructure machine
	// annotation that stores the group-kind of the infrastructure template resource
	// that was cloned for the machine. This annotation is set only during cloning a
	// template. Older/adopted machines will not have this annotation.
	TemplateClonedFromGroupKindAnnotation = "machine.openshift.io/cloned-from-groupkind"

	// TemplateClonedFromNameAnnotation is the infrastructure machine annotation that
	// stores the name of the infrastructure template resource
	// that was cloned for the machine. This annotation is set only during cloning a
	//  template. Older/adopted machines will not have this annotation.
	TemplateClonedFromNameAnnotation = "machine.openshift.io/cloned-from-name"

	// This error indicates that the machine did not join the cluster
	// as a new node within the expected timeframe after instance
	// creation at the provider succeeded
	//
	// Example use case: A controller that deletes Machines which do
	// not result in a Node joining the cluster within a given timeout
	// and that are managed by a MachineSet
	JoinClusterTimeoutMachineError = "JoinClusterTimeoutError"

	// IPAddressInvalidReason is set to indicate that the claimed IP address is not valid.
	IPAddressInvalidReason MachineStatusError = "IPAddressInvalid"
)

type ClusterStatusError string

const (
	// InvalidConfigurationClusterError indicates that the cluster
	// configuration is invalid.
	InvalidConfigurationClusterError ClusterStatusError = "InvalidConfiguration"

	// UnsupportedChangeClusterError indicates that the cluster
	// spec has been updated in an unsupported way. That cannot be
	// reconciled.
	UnsupportedChangeClusterError ClusterStatusError = "UnsupportedChange"

	// CreateClusterError indicates that an error was encountered
	// when trying to create the cluster.
	CreateClusterError ClusterStatusError = "CreateError"

	// UpdateClusterError indicates that an error was encountered
	// when trying to update the cluster.
	UpdateClusterError ClusterStatusError = "UpdateError"

	// DeleteClusterError indicates that an error was encountered
	// when trying to delete the cluster.
	DeleteClusterError ClusterStatusError = "DeleteError"
)

type MachineSetStatusError string

const (
	// Represents that the combination of configuration in the MachineTemplateSpec
	// is not supported by this cluster. This is not a transient error, but
	// indicates a state that must be fixed before progress can be made.
	//
	// Example: the ProviderSpec specifies an instance type that doesn't exist.
	InvalidConfigurationMachineSetError MachineSetStatusError = "InvalidConfiguration"
)

type MachineDeploymentStrategyType string

const (
	// Replace the old MachineSet by new one using rolling update
	// i.e. gradually scale down the old MachineSet and scale up the new one.
	RollingUpdateMachineDeploymentStrategyType MachineDeploymentStrategyType = "RollingUpdate"
)

const (
	// PhaseFailed indicates a state that will need to be fixed before progress can be made.
	// Failed machines have encountered a terminal error and must be deleted.
	// https://github.com/openshift/enhancements/blob/master/enhancements/machine-instance-lifecycle.md
	// e.g. Instance does NOT exist but Machine has providerID/addresses.
	// e.g. Cloud service returns a 4xx response.
	PhaseFailed string = "Failed"

	// PhaseProvisioning indicates the instance does NOT exist.
	// The machine has NOT been given a providerID or addresses.
	// Provisioning implies that the Machine API is in the process of creating the instance.
	PhaseProvisioning string = "Provisioning"

	// PhaseProvisioned indicates the instance exists.
	// The machine has been given a providerID and addresses.
	// The machine API successfully provisioned an instance which has not yet joined the cluster,
	// as such, the machine has NOT yet been given a nodeRef.
	PhaseProvisioned string = "Provisioned"

	// PhaseRunning indicates the instance exists and the node has joined the cluster.
	// The machine has been given a providerID, addresses, and a nodeRef.
	PhaseRunning string = "Running"

	// PhaseDeleting indicates the machine has a deletion timestamp and that the
	// Machine API is now in the process of removing the machine from the cluster.
	PhaseDeleting string = "Deleting"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Machine is the Schema for the machines API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Phase of machine"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".metadata.labels['machine\\.openshift\\.io/instance-type']",description="Type of instance"
// +kubebuilder:printcolumn:name="Region",type="string",JSONPath=".metadata.labels['machine\\.openshift\\.io/region']",description="Region associated with machine"
// +kubebuilder:printcolumn:name="Zone",type="string",JSONPath=".metadata.labels['machine\\.openshift\\.io/zone']",description="Zone associated with machine"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Machine age"
// +kubebuilder:printcolumn:name="Node",type="string",JSONPath=".status.nodeRef.name",description="Node associated with machine",priority=1
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID of machine created in cloud provider",priority=1
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".metadata.annotations['machine\\.openshift\\.io/instance-state']",description="State of instance",priority=1
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type Machine struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineSpec   `json:"spec,omitempty"`
	Status MachineStatus `json:"status,omitempty"`
}

// MachineSpec defines the desired state of Machine
type MachineSpec struct {
	// ObjectMeta will autopopulate the Node created. Use this to
	// indicate what labels, annotations, name prefix, etc., should be used
	// when creating the Node.
	// +optional
	ObjectMeta `json:"metadata,omitempty"`

	// LifecycleHooks allow users to pause operations on the machine at
	// certain predefined points within the machine lifecycle.
	// +optional
	LifecycleHooks LifecycleHooks `json:"lifecycleHooks,omitempty"`

	// The list of the taints to be applied to the corresponding Node in additive
	// manner. This list will not overwrite any other taints added to the Node on
	// an ongoing basis by other entities. These taints should be actively reconciled
	// e.g. if you ask the machine controller to apply a taint and then manually remove
	// the taint the machine controller will put it back) but not have the machine controller
	// remove any taints
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`

	// ProviderSpec details Provider-specific configuration to use during node creation.
	// +optional
	ProviderSpec ProviderSpec `json:"providerSpec"`

	// ProviderID is the identification ID of the machine provided by the provider.
	// This field must match the provider ID as seen on the node object corresponding to this machine.
	// This field is required by higher level consumers of cluster-api. Example use case is cluster autoscaler
	// with cluster-api as provider. Clean-up logic in the autoscaler compares machines to nodes to find out
	// machines at provider which could not get registered as Kubernetes nodes. With cluster-api as a
	// generic out-of-tree provider for autoscaler, this field is required by autoscaler to be
	// able to have a provider view of the list of machines. Another list of nodes is queried from the k8s apiserver
	// and then a comparison is done to find out unregistered machines and are marked for delete.
	// This field will be set by the actuators and consumed by higher level entities like autoscaler that will
	// be interfacing with cluster-api as generic provider.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`
}

// LifecycleHooks allow users to pause operations on the machine at
// certain prefedined points within the machine lifecycle.
type LifecycleHooks struct {
	// PreDrain hooks prevent the machine from being drained.
	// This also blocks further lifecycle events, such as termination.
	// +listType=map
	// +listMapKey=name
	// +optional
	PreDrain []LifecycleHook `json:"preDrain,omitempty"`

	// PreTerminate hooks prevent the machine from being terminated.
	// PreTerminate hooks be actioned after the Machine has been drained.
	// +listType=map
	// +listMapKey=name
	// +optional
	PreTerminate []LifecycleHook `json:"preTerminate,omitempty"`
}

// LifecycleHook represents a single instance of a lifecycle hook
type LifecycleHook struct {
	// Name defines a unique name for the lifcycle hook.
	// The name should be unique and descriptive, ideally 1-3 words, in CamelCase or
	// it may be namespaced, eg. foo.example.com/CamelCase.
	// Names must be unique and should only be managed by a single entity.
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`
	// +kubebuilder:validation:MinLength:=3
	// +kubebuilder:validation:MaxLength:=256
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Owner defines the owner of the lifecycle hook.
	// This should be descriptive enough so that users can identify
	// who/what is responsible for blocking the lifecycle.
	// This could be the name of a controller (e.g. clusteroperator/etcd)
	// or an administrator managing the hook.
	// +kubebuilder:validation:MinLength:=3
	// +kubebuilder:validation:MaxLength:=512
	// +kubebuilder:validation:Required
	Owner string `json:"owner"`
}

// MachineStatus defines the observed state of Machine
type MachineStatus struct {
	// NodeRef will point to the corresponding Node if it exists.
	// +optional
	NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// ErrorReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	ErrorReason *MachineStatusError `json:"errorReason,omitempty"`

	// ErrorMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// ProviderStatus details a Provider-specific status.
	// It is recommended that providers maintain their
	// own versioned API types that should be
	// serialized/deserialized from this field.
	// +optional
	// +kubebuilder:validation:XPreserveUnknownFields
	ProviderStatus *runtime.RawExtension `json:"providerStatus,omitempty"`

	// Addresses is a list of addresses assigned to the machine. Queried from cloud provider, if available.
	// +optional
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`

	// LastOperation describes the last-operation performed by the machine-controller.
	// This API should be useful as a history in terms of the latest operation performed on the
	// specific machine. It should also convey the state of the latest-operation for example if
	// it is still on-going, failed or completed successfully.
	// +optional
	LastOperation *LastOperation `json:"lastOperation,omitempty"`

	// Phase represents the current phase of machine actuation.
	// One of: Failed, Provisioning, Provisioned, Running, Deleting
	// +optional
	Phase *string `json:"phase,omitempty"`

	// Conditions defines the current state of the Machine
	Conditions Conditions `json:"conditions,omitempty"`
}

// LastOperation represents the detail of the last performed operation on the MachineObject.
type LastOperation struct {
	// Description is the human-readable description of the last operation.
	Description *string `json:"description,omitempty"`

	// LastUpdated is the timestamp at which LastOperation API was last-updated.
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// State is the current status of the last performed operation.
	// E.g. Processing, Failed, Successful etc
	State *string `json:"state,omitempty"`

	// Type is the type of operation which was last performed.
	// E.g. Create, Delete, Update etc
	Type *string `json:"type,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineList contains a list of Machine
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type MachineList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Machine `json:"items"`
}
