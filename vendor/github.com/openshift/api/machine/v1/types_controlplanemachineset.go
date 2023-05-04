package v1

import (
	configv1 "github.com/openshift/api/config/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControlPlaneMachineSet ensures that a specified number of control plane machine replicas are running at any given time.
// +k8s:openapi-gen=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".spec.replicas",description="Desired Replicas"
// +kubebuilder:printcolumn:name="Current",type="integer",JSONPath=".status.replicas",description="Current Replicas"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas",description="Ready Replicas"
// +kubebuilder:printcolumn:name="Updated",type="integer",JSONPath=".status.updatedReplicas",description="Updated Replicas"
// +kubebuilder:printcolumn:name="Unavailable",type="integer",JSONPath=".status.unavailableReplicas",description="Observed number of unavailable replicas"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".spec.state",description="ControlPlaneMachineSet state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="ControlPlaneMachineSet age"
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type ControlPlaneMachineSet struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneMachineSetSpec   `json:"spec,omitempty"`
	Status ControlPlaneMachineSetStatus `json:"status,omitempty"`
}

// ControlPlaneMachineSet represents the configuration of the ControlPlaneMachineSet.
type ControlPlaneMachineSetSpec struct {
	// State defines whether the ControlPlaneMachineSet is Active or Inactive.
	// When Inactive, the ControlPlaneMachineSet will not take any action on the
	// state of the Machines within the cluster.
	// When Active, the ControlPlaneMachineSet will reconcile the Machines and
	// will update the Machines as necessary.
	// Once Active, a ControlPlaneMachineSet cannot be made Inactive. To prevent
	// further action please remove the ControlPlaneMachineSet.
	// +kubebuilder:default:="Inactive"
	// +default="Inactive"
	// +kubebuilder:validation:XValidation:rule="oldSelf != 'Active' || self == oldSelf",message="state cannot be changed once Active"
	// +optional
	State ControlPlaneMachineSetState `json:"state,omitempty"`

	// Replicas defines how many Control Plane Machines should be
	// created by this ControlPlaneMachineSet.
	// This field is immutable and cannot be changed after cluster
	// installation.
	// The ControlPlaneMachineSet only operates with 3 or 5 node control planes,
	// 3 and 5 are the only valid values for this field.
	// +kubebuilder:validation:Enum:=3;5
	// +kubebuilder:default:=3
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="replicas is immutable"
	// +kubebuilder:validation:Required
	Replicas *int32 `json:"replicas"`

	// Strategy defines how the ControlPlaneMachineSet will update
	// Machines when it detects a change to the ProviderSpec.
	// +kubebuilder:default:={type: RollingUpdate}
	// +optional
	Strategy ControlPlaneMachineSetStrategy `json:"strategy,omitempty"`

	// Label selector for Machines. Existing Machines selected by this
	// selector will be the ones affected by this ControlPlaneMachineSet.
	// It must match the template's labels.
	// This field is considered immutable after creation of the resource.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="selector is immutable"
	// +kubebuilder:validation:Required
	Selector metav1.LabelSelector `json:"selector"`

	// Template describes the Control Plane Machines that will be created
	// by this ControlPlaneMachineSet.
	// +kubebuilder:validation:Required
	Template ControlPlaneMachineSetTemplate `json:"template"`
}

// ControlPlaneMachineSetState is an enumeration of the possible states of the
// ControlPlaneMachineSet resource. It allows it to be either Active or Inactive.
// +kubebuilder:validation:Enum:="Active";"Inactive"
type ControlPlaneMachineSetState string

const (
	// ControlPlaneMachineSetStateActive is the value used to denote the ControlPlaneMachineSet
	// should be active and should perform updates as required.
	ControlPlaneMachineSetStateActive ControlPlaneMachineSetState = "Active"

	// ControlPlaneMachineSetStateInactive is the value used to denote the ControlPlaneMachineSet
	// should be not active and should no perform any updates.
	ControlPlaneMachineSetStateInactive ControlPlaneMachineSetState = "Inactive"
)

// ControlPlaneMachineSetTemplate is a template used by the ControlPlaneMachineSet
// to create the Machines that it will manage in the future.
// +union
// + ---
// + This struct is a discriminated union which allows users to select the type of Machine
// + that the ControlPlaneMachineSet should create and manage.
// + For now, the only supported type is the OpenShift Machine API Machine, but in the future
// + we plan to expand this to allow other Machine types such as Cluster API Machines or a
// + future version of the Machine API Machine.
// +kubebuilder:validation:XValidation:rule="has(self.machineType) && self.machineType == 'machines_v1beta1_machine_openshift_io' ?  has(self.machines_v1beta1_machine_openshift_io) : !has(self.machines_v1beta1_machine_openshift_io)",message="machines_v1beta1_machine_openshift_io configuration is required when machineType is machines_v1beta1_machine_openshift_io, and forbidden otherwise"
type ControlPlaneMachineSetTemplate struct {
	// MachineType determines the type of Machines that should be managed by the ControlPlaneMachineSet.
	// Currently, the only valid value is machines_v1beta1_machine_openshift_io.
	// +unionDiscriminator
	// +kubebuilder:validation:Required
	MachineType ControlPlaneMachineSetMachineType `json:"machineType,omitempty"`

	// OpenShiftMachineV1Beta1Machine defines the template for creating Machines
	// from the v1beta1.machine.openshift.io API group.
	// +optional
	OpenShiftMachineV1Beta1Machine *OpenShiftMachineV1Beta1MachineTemplate `json:"machines_v1beta1_machine_openshift_io,omitempty"`
}

// ControlPlaneMachineSetMachineType is a enumeration of valid Machine types
// supported by the ControlPlaneMachineSet.
// +kubebuilder:validation:Enum:=machines_v1beta1_machine_openshift_io
type ControlPlaneMachineSetMachineType string

const (
	// OpenShiftMachineV1Beta1MachineType is the OpenShift Machine API v1beta1 Machine type.
	OpenShiftMachineV1Beta1MachineType ControlPlaneMachineSetMachineType = "machines_v1beta1_machine_openshift_io"
)

// OpenShiftMachineV1Beta1MachineTemplate is a template for the ControlPlaneMachineSet to create
// Machines from the v1beta1.machine.openshift.io API group.
type OpenShiftMachineV1Beta1MachineTemplate struct {
	// FailureDomains is the list of failure domains (sometimes called
	// availability zones) in which the ControlPlaneMachineSet should balance
	// the Control Plane Machines.
	// This will be merged into the ProviderSpec given in the template.
	// This field is optional on platforms that do not require placement information.
	// +optional
	FailureDomains FailureDomains `json:"failureDomains,omitempty"`

	// ObjectMeta is the standard object metadata
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// Labels are required to match the ControlPlaneMachineSet selector.
	// +kubebuilder:validation:Required
	ObjectMeta ControlPlaneMachineSetTemplateObjectMeta `json:"metadata"`

	// Spec contains the desired configuration of the Control Plane Machines.
	// The ProviderSpec within contains platform specific details
	// for creating the Control Plane Machines.
	// The ProviderSe should be complete apart from the platform specific
	// failure domain field. This will be overriden when the Machines
	// are created based on the FailureDomains field.
	// +kubebuilder:validation:Required
	Spec machinev1beta1.MachineSpec `json:"spec"`
}

// ControlPlaneMachineSetTemplateObjectMeta is a subset of the metav1.ObjectMeta struct.
// It allows users to specify labels and annotations that will be copied onto Machines
// created from this template.
type ControlPlaneMachineSetTemplateObjectMeta struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels.
	// This field must contain both the 'machine.openshift.io/cluster-api-machine-role' and 'machine.openshift.io/cluster-api-machine-type' labels, both with a value of 'master'.
	// It must also contain a label with the key 'machine.openshift.io/cluster-api-cluster'.
	// +kubebuilder:validation:XValidation:rule="'machine.openshift.io/cluster-api-machine-role' in self && self['machine.openshift.io/cluster-api-machine-role'] == 'master'",message="label 'machine.openshift.io/cluster-api-machine-role' is required, and must have value 'master'"
	// +kubebuilder:validation:XValidation:rule="'machine.openshift.io/cluster-api-machine-type' in self && self['machine.openshift.io/cluster-api-machine-type'] == 'master'",message="label 'machine.openshift.io/cluster-api-machine-type' is required, and must have value 'master'"
	// +kubebuilder:validation:XValidation:rule="'machine.openshift.io/cluster-api-cluster' in self",message="label 'machine.openshift.io/cluster-api-cluster' is required"
	// +kubebuilder:validation:Required
	Labels map[string]string `json:"labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ControlPlaneMachineSetStrategy defines the strategy for applying updates to the
// Control Plane Machines managed by the ControlPlaneMachineSet.
type ControlPlaneMachineSetStrategy struct {
	// Type defines the type of update strategy that should be
	// used when updating Machines owned by the ControlPlaneMachineSet.
	// Valid values are "RollingUpdate" and "OnDelete".
	// The current default value is "RollingUpdate".
	// +kubebuilder:default:="RollingUpdate"
	// +default="RollingUpdate"
	// +kubebuilder:validation:Enum:="RollingUpdate";"OnDelete"
	// +optional
	Type ControlPlaneMachineSetStrategyType `json:"type,omitempty"`

	// This is left as a struct to allow future rolling update
	// strategy configuration to be added later.
}

// ControlPlaneMachineSetStrategyType is an enumeration of different update strategies
// for the Control Plane Machines.
type ControlPlaneMachineSetStrategyType string

const (
	// RollingUpdate is the default update strategy type for a
	// ControlPlaneMachineSet. This will cause the ControlPlaneMachineSet to
	// first create a new Machine and wait for this to be Ready
	// before removing the Machine chosen for replacement.
	RollingUpdate ControlPlaneMachineSetStrategyType = "RollingUpdate"

	// Recreate causes the ControlPlaneMachineSet controller to first
	// remove a ControlPlaneMachine before creating its
	// replacement. This allows for scenarios with limited capacity
	// such as baremetal environments where additional capacity to
	// perform rolling updates is not available.
	Recreate ControlPlaneMachineSetStrategyType = "Recreate"

	// OnDelete causes the ControlPlaneMachineSet to only replace a
	// Machine once it has been marked for deletion. This strategy
	// makes the rollout of updated specifications into a manual
	// process. This allows users to test new configuration on
	// a single Machine without forcing the rollout of all of their
	// Control Plane Machines.
	OnDelete ControlPlaneMachineSetStrategyType = "OnDelete"
)

// FailureDomain represents the different configurations required to spread Machines
// across failure domains on different platforms.
// +union
// +kubebuilder:validation:XValidation:rule="has(self.platform) && self.platform == 'AWS' ?  has(self.aws) : !has(self.aws)",message="aws configuration is required when platform is AWS, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="has(self.platform) && self.platform == 'Azure' ?  has(self.azure) : !has(self.azure)",message="azure configuration is required when platform is Azure, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="has(self.platform) && self.platform == 'GCP' ?  has(self.gcp) : !has(self.gcp)",message="gcp configuration is required when platform is GCP, and forbidden otherwise"
type FailureDomains struct {
	// Platform identifies the platform for which the FailureDomain represents.
	// Currently supported values are AWS, Azure, and GCP.
	// +unionDiscriminator
	// +kubebuilder:validation:Required
	Platform configv1.PlatformType `json:"platform"`

	// AWS configures failure domain information for the AWS platform.
	// +optional
	AWS *[]AWSFailureDomain `json:"aws,omitempty"`

	// Azure configures failure domain information for the Azure platform.
	// +optional
	Azure *[]AzureFailureDomain `json:"azure,omitempty"`

	// GCP configures failure domain information for the GCP platform.
	// +optional
	GCP *[]GCPFailureDomain `json:"gcp,omitempty"`
}

// AWSFailureDomain configures failure domain information for the AWS platform.
// +kubebuilder:validation:MinProperties:=1
type AWSFailureDomain struct {
	// Subnet is a reference to the subnet to use for this instance.
	// +optional
	Subnet *AWSResourceReference `json:"subnet,omitempty"`

	// Placement configures the placement information for this instance.
	// +optional
	Placement AWSFailureDomainPlacement `json:"placement,omitempty"`
}

// AWSFailureDomainPlacement configures the placement information for the AWSFailureDomain.
type AWSFailureDomainPlacement struct {
	// AvailabilityZone is the availability zone of the instance.
	// +kubebuilder:validation:Required
	AvailabilityZone string `json:"availabilityZone"`
}

// AzureFailureDomain configures failure domain information for the Azure platform.
type AzureFailureDomain struct {
	// Availability Zone for the virtual machine.
	// If nil, the virtual machine should be deployed to no zone.
	// +kubebuilder:validation:Required
	Zone string `json:"zone"`
}

// GCPFailureDomain configures failure domain information for the GCP platform
type GCPFailureDomain struct {
	// Zone is the zone in which the GCP machine provider will create the VM.
	// +kubebuilder:validation:Required
	Zone string `json:"zone"`
}

// ControlPlaneMachineSetStatus represents the status of the ControlPlaneMachineSet CRD.
type ControlPlaneMachineSetStatus struct {
	// Conditions represents the observations of the ControlPlaneMachineSet's current state.
	// Known .status.conditions.type are: Available, Degraded and Progressing.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration is the most recent generation observed for this
	// ControlPlaneMachineSet. It corresponds to the ControlPlaneMachineSets's generation,
	// which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas is the number of Control Plane Machines created by the
	// ControlPlaneMachineSet controller.
	// Note that during update operations this value may differ from the
	// desired replica count.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of Control Plane Machines created by the
	// ControlPlaneMachineSet controller which are ready.
	// Note that this value may be higher than the desired number of replicas
	// while rolling updates are in-progress.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// UpdatedReplicas is the number of non-terminated Control Plane Machines
	// created by the ControlPlaneMachineSet controller that have the desired
	// provider spec and are ready.
	// This value is set to 0 when a change is detected to the desired spec.
	// When the update strategy is RollingUpdate, this will also coincide
	// with starting the process of updating the Machines.
	// When the update strategy is OnDelete, this value will remain at 0 until
	// a user deletes an existing replica and its replacement has become ready.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// UnavailableReplicas is the number of Control Plane Machines that are
	// still required before the ControlPlaneMachineSet reaches the desired
	// available capacity. When this value is non-zero, the number of
	// ReadyReplicas is less than the desired Replicas.
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControlPlaneMachineSetList contains a list of ControlPlaneMachineSet
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type ControlPlaneMachineSetList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ControlPlaneMachineSet `json:"items"`
}
