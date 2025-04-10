package v1

import (
	configv1 "github.com/openshift/api/config/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=controlplanemachinesets,scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".spec.replicas",description="Desired Replicas"
// +kubebuilder:printcolumn:name="Current",type="integer",JSONPath=".status.replicas",description="Current Replicas"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas",description="Ready Replicas"
// +kubebuilder:printcolumn:name="Updated",type="integer",JSONPath=".status.updatedReplicas",description="Updated Replicas"
// +kubebuilder:printcolumn:name="Unavailable",type="integer",JSONPath=".status.unavailableReplicas",description="Observed number of unavailable replicas"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".spec.state",description="ControlPlaneMachineSet state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="ControlPlaneMachineSet age"
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/1112
// +openshift:file-pattern=cvoRunLevel=0000_10,operatorName=control-plane-machine-set,operatorOrdering=01
// +openshift:capability=MachineAPI
// +kubebuilder:metadata:annotations="exclude.release.openshift.io/internal-openshift-hosted=true"
// +kubebuilder:metadata:annotations=include.release.openshift.io/self-managed-high-availability=true

// ControlPlaneMachineSet ensures that a specified number of control plane machine replicas are running at any given time.
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
	// machineNamePrefix is the prefix used when creating machine names.
	// Each machine name will consist of this prefix, followed by
	// a randomly generated string of 5 characters, and the index of the machine.
	// It must be a lowercase RFC 1123 subdomain, consisting of lowercase
	// alphanumeric characters, hyphens ('-'), and periods ('.').
	// Each block, separated by periods, must start and end with an alphanumeric character.
	// Hyphens are not allowed at the start or end of a block, and consecutive periods are not permitted.
	// The prefix must be between 1 and 245 characters in length.
	// For example, if machineNamePrefix is set to 'control-plane',
	// and three machines are created, their names might be:
	// control-plane-abcde-0, control-plane-fghij-1, control-plane-klmno-2
	// +openshift:validation:FeatureGateAwareXValidation:featureGate=CPMSMachineNamePrefix,rule="!format.dns1123Subdomain().validate(self).hasValue()",message="a lowercase RFC 1123 subdomain must consist of lowercase alphanumeric characters, hyphens ('-'), and periods ('.'). Each block, separated by periods, must start and end with an alphanumeric character. Hyphens are not allowed at the start or end of a block, and consecutive periods are not permitted."
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=245
	// +openshift:enable:FeatureGate=CPMSMachineNamePrefix
	// +optional
	MachineNamePrefix string `json:"machineNamePrefix,omitempty"`

	// state defines whether the ControlPlaneMachineSet is Active or Inactive.
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

	// replicas defines how many Control Plane Machines should be
	// created by this ControlPlaneMachineSet.
	// This field is immutable and cannot be changed after cluster
	// installation.
	// The ControlPlaneMachineSet only operates with 3 or 5 node control planes,
	// 3 and 5 are the only valid values for this field.
	// +kubebuilder:validation:Enum:=3;5
	// +kubebuilder:default:=3
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="replicas is immutable"
	// +required
	Replicas *int32 `json:"replicas"`

	// strategy defines how the ControlPlaneMachineSet will update
	// Machines when it detects a change to the ProviderSpec.
	// +kubebuilder:default:={type: RollingUpdate}
	// +optional
	Strategy ControlPlaneMachineSetStrategy `json:"strategy,omitempty"`

	// Label selector for Machines. Existing Machines selected by this
	// selector will be the ones affected by this ControlPlaneMachineSet.
	// It must match the template's labels.
	// This field is considered immutable after creation of the resource.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="selector is immutable"
	// +required
	Selector metav1.LabelSelector `json:"selector"`

	// template describes the Control Plane Machines that will be created
	// by this ControlPlaneMachineSet.
	// +required
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
	// machineType determines the type of Machines that should be managed by the ControlPlaneMachineSet.
	// Currently, the only valid value is machines_v1beta1_machine_openshift_io.
	// +unionDiscriminator
	// +required
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
	// failureDomains is the list of failure domains (sometimes called
	// availability zones) in which the ControlPlaneMachineSet should balance
	// the Control Plane Machines.
	// This will be merged into the ProviderSpec given in the template.
	// This field is optional on platforms that do not require placement information.
	// +optional
	FailureDomains *FailureDomains `json:"failureDomains,omitempty"`

	// ObjectMeta is the standard object metadata
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// Labels are required to match the ControlPlaneMachineSet selector.
	// +required
	ObjectMeta ControlPlaneMachineSetTemplateObjectMeta `json:"metadata"`

	// spec contains the desired configuration of the Control Plane Machines.
	// The ProviderSpec within contains platform specific details
	// for creating the Control Plane Machines.
	// The ProviderSe should be complete apart from the platform specific
	// failure domain field. This will be overriden when the Machines
	// are created based on the FailureDomains field.
	// +required
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
	// +required
	Labels map[string]string `json:"labels"`

	// annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ControlPlaneMachineSetStrategy defines the strategy for applying updates to the
// Control Plane Machines managed by the ControlPlaneMachineSet.
type ControlPlaneMachineSetStrategy struct {
	// type defines the type of update strategy that should be
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
// +kubebuilder:validation:XValidation:rule="has(self.platform) && self.platform == 'OpenStack' ?  has(self.openstack) : !has(self.openstack)",message="openstack configuration is required when platform is OpenStack, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="has(self.platform) && self.platform == 'VSphere' ?  has(self.vsphere) : !has(self.vsphere)",message="vsphere configuration is required when platform is VSphere, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="has(self.platform) && self.platform == 'Nutanix' ?  has(self.nutanix) : !has(self.nutanix)",message="nutanix configuration is required when platform is Nutanix, and forbidden otherwise"
type FailureDomains struct {
	// platform identifies the platform for which the FailureDomain represents.
	// Currently supported values are AWS, Azure, GCP, OpenStack, VSphere and Nutanix.
	// +unionDiscriminator
	// +required
	Platform configv1.PlatformType `json:"platform"`

	// aws configures failure domain information for the AWS platform.
	// +listType=atomic
	// +optional
	AWS *[]AWSFailureDomain `json:"aws,omitempty"`

	// azure configures failure domain information for the Azure platform.
	// +listType=atomic
	// +optional
	Azure *[]AzureFailureDomain `json:"azure,omitempty"`

	// gcp configures failure domain information for the GCP platform.
	// +listType=atomic
	// +optional
	GCP *[]GCPFailureDomain `json:"gcp,omitempty"`

	// vsphere configures failure domain information for the VSphere platform.
	// +listType=map
	// +listMapKey=name
	// +optional
	VSphere []VSphereFailureDomain `json:"vsphere,omitempty"`

	// openstack configures failure domain information for the OpenStack platform.
	// +optional
	//
	// + ---
	// + Unlike other platforms, OpenStack failure domains can be empty.
	// + Some OpenStack deployments may not have availability zones or root volumes.
	// + Therefore we'll check the length of the list to determine if it's empty instead
	// + of nil if it would be a pointer.
	// +listType=atomic
	// +optional
	OpenStack []OpenStackFailureDomain `json:"openstack,omitempty"`

	// nutanix configures failure domain information for the Nutanix platform.
	// +listType=map
	// +listMapKey=name
	// +optional
	Nutanix []NutanixFailureDomainReference `json:"nutanix,omitempty"`
}

// AWSFailureDomain configures failure domain information for the AWS platform.
// +kubebuilder:validation:MinProperties:=1
type AWSFailureDomain struct {
	// subnet is a reference to the subnet to use for this instance.
	// +optional
	Subnet *AWSResourceReference `json:"subnet,omitempty"`

	// placement configures the placement information for this instance.
	// +optional
	Placement AWSFailureDomainPlacement `json:"placement,omitempty"`
}

// AWSFailureDomainPlacement configures the placement information for the AWSFailureDomain.
type AWSFailureDomainPlacement struct {
	// availabilityZone is the availability zone of the instance.
	// +required
	AvailabilityZone string `json:"availabilityZone"`
}

// AzureFailureDomain configures failure domain information for the Azure platform.
type AzureFailureDomain struct {
	// Availability Zone for the virtual machine.
	// If nil, the virtual machine should be deployed to no zone.
	// +required
	Zone string `json:"zone"`

	// subnet is the name of the network subnet in which the VM will be created.
	// When omitted, the subnet value from the machine providerSpec template will be used.
	// +kubebuilder:validation:MaxLength=80
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9](?:[a-zA-Z0-9._-]*[a-zA-Z0-9_])?$`
	// +optional
	Subnet string `json:"subnet,omitempty"`
}

// GCPFailureDomain configures failure domain information for the GCP platform
type GCPFailureDomain struct {
	// zone is the zone in which the GCP machine provider will create the VM.
	// +required
	Zone string `json:"zone"`
}

// VSphereFailureDomain configures failure domain information for the vSphere platform
type VSphereFailureDomain struct {
	// name of the failure domain in which the vSphere machine provider will create the VM.
	// Failure domains are defined in a cluster's config.openshift.io/Infrastructure resource.
	// When balancing machines across failure domains, the control plane machine set will inject configuration from the
	// Infrastructure resource into the machine providerSpec to allocate the machine to a failure domain.
	// +required
	Name string `json:"name"`
}

// OpenStackFailureDomain configures failure domain information for the OpenStack platform.
// +kubebuilder:validation:MinProperties:=1
// +kubebuilder:validation:XValidation:rule="!has(self.availabilityZone) || !has(self.rootVolume) || has(self.rootVolume.availabilityZone)",message="rootVolume.availabilityZone is required when availabilityZone is set"
type OpenStackFailureDomain struct {
	// availabilityZone is the nova availability zone in which the OpenStack machine provider will create the VM.
	// If not specified, the VM will be created in the default availability zone specified in the nova configuration.
	// Availability zone names must NOT contain : since it is used by admin users to specify hosts where instances
	// are launched in server creation. Also, it must not contain spaces otherwise it will lead to node that belongs
	// to this availability zone register failure, see kubernetes/cloud-provider-openstack#1379 for further information.
	// The maximum length of availability zone name is 63 as per labels limits.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[^: ]*$`
	// +kubebuilder:validation:MaxLength=63
	// +optional
	AvailabilityZone string `json:"availabilityZone,omitempty"`

	// rootVolume contains settings that will be used by the OpenStack machine provider to create the root volume attached to the VM.
	// If not specified, no root volume will be created.
	//
	// + ---
	// + RootVolume must be a pointer to allow us to require at least one valid property is set within the failure domain.
	// + If it were a reference then omitempty doesn't work and the minProperties validations are no longer valid.
	// +optional
	RootVolume *RootVolume `json:"rootVolume,omitempty"`
}

// NutanixFailureDomainReference refers to the failure domain of the Nutanix platform.
type NutanixFailureDomainReference struct {
	// name of the failure domain in which the nutanix machine provider will create the VM.
	// Failure domains are defined in a cluster's config.openshift.io/Infrastructure resource.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern=`[a-z0-9]([-a-z0-9]*[a-z0-9])?`
	Name string `json:"name"`
}

// RootVolume represents the volume metadata to boot from.
// The original RootVolume struct is defined in the v1alpha1 but it's not best practice to use it directly here so we define a new one
// that should stay in sync with the original one.
type RootVolume struct {
	// availabilityZone specifies the Cinder availability zone where the root volume will be created.
	// If not specifified, the root volume will be created in the availability zone specified by the volume type in the cinder configuration.
	// If the volume type (configured in the OpenStack cluster) does not specify an availability zone, the root volume will be created in the default availability
	// zone specified in the cinder configuration. See https://docs.openstack.org/cinder/latest/admin/availability-zone-type.html for more details.
	// If the OpenStack cluster is deployed with the cross_az_attach configuration option set to false, the root volume will have to be in the same
	// availability zone as the VM (defined by OpenStackFailureDomain.AvailabilityZone).
	// Availability zone names must NOT contain spaces otherwise it will lead to volume that belongs to this availability zone register failure,
	// see kubernetes/cloud-provider-openstack#1379 for further information.
	// The maximum length of availability zone name is 63 as per labels limits.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[^ ]*$`
	// +optional
	AvailabilityZone string `json:"availabilityZone,omitempty"`

	// volumeType specifies the type of the root volume that will be provisioned.
	// The maximum length of a volume type name is 255 characters, as per the OpenStack limit.
	// + ---
	// + Historically, the installer has always required a volume type to be specified when deploying
	// + the control plane with a root volume. This is because the default volume type in Cinder is not guaranteed
	// + to be available, therefore we prefer the user to be explicit about the volume type to use.
	// + We apply the same logic in CPMS: if the failure domain specifies a root volume, we require the user to specify a volume type.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	VolumeType string `json:"volumeType"`
}

// ControlPlaneMachineSetStatus represents the status of the ControlPlaneMachineSet CRD.
type ControlPlaneMachineSetStatus struct {
	// conditions represents the observations of the ControlPlaneMachineSet's current state.
	// Known .status.conditions.type are: Available, Degraded and Progressing.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the most recent generation observed for this
	// ControlPlaneMachineSet. It corresponds to the ControlPlaneMachineSets's generation,
	// which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// replicas is the number of Control Plane Machines created by the
	// ControlPlaneMachineSet controller.
	// Note that during update operations this value may differ from the
	// desired replica count.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// readyReplicas is the number of Control Plane Machines created by the
	// ControlPlaneMachineSet controller which are ready.
	// Note that this value may be higher than the desired number of replicas
	// while rolling updates are in-progress.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// updatedReplicas is the number of non-terminated Control Plane Machines
	// created by the ControlPlaneMachineSet controller that have the desired
	// provider spec and are ready.
	// This value is set to 0 when a change is detected to the desired spec.
	// When the update strategy is RollingUpdate, this will also coincide
	// with starting the process of updating the Machines.
	// When the update strategy is OnDelete, this value will remain at 0 until
	// a user deletes an existing replica and its replacement has become ready.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// unavailableReplicas is the number of Control Plane Machines that are
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
