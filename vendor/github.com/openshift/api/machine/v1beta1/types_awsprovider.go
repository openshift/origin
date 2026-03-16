package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AWSMachineProviderConfig is the Schema for the awsmachineproviderconfigs API
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AWSMachineProviderConfig struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// ami is the reference to the AMI from which to create the machine instance.
	AMI AWSResourceReference `json:"ami"`
	// instanceType is the type of instance to create. Example: m4.xlarge
	InstanceType string `json:"instanceType"`
	// cpuOptions defines CPU-related settings for the instance, including the confidential computing policy.
	// When omitted, this means no opinion and the AWS platform is left to choose a reasonable default.
	// More info:
	// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CpuOptionsRequest.html,
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/cpu-options-supported-instances-values.html
	// +optional
	CPUOptions *CPUOptions `json:"cpuOptions,omitempty,omitzero"`
	// tags is the set of tags to add to apply to an instance, in addition to the ones
	// added by default by the actuator. These tags are additive. The actuator will ensure
	// these tags are present, but will not remove any other tags that may exist on the
	// instance.
	// +optional
	Tags []TagSpecification `json:"tags,omitempty"`
	// iamInstanceProfile is a reference to an IAM role to assign to the instance
	// +optional
	IAMInstanceProfile *AWSResourceReference `json:"iamInstanceProfile,omitempty"`
	// userDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance
	// +optional
	UserDataSecret *corev1.LocalObjectReference `json:"userDataSecret,omitempty"`
	// credentialsSecret is a reference to the secret with AWS credentials. Otherwise, defaults to permissions
	// provided by attached IAM role where the actuator is running.
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
	// keyName is the name of the KeyPair to use for SSH
	// +optional
	KeyName *string `json:"keyName,omitempty"`
	// deviceIndex is the index of the device on the instance for the network interface attachment.
	// Defaults to 0.
	DeviceIndex int64 `json:"deviceIndex"`
	// publicIp specifies whether the instance should get a public IP. If not present,
	// it should use the default of its subnet.
	// +optional
	PublicIP *bool `json:"publicIp,omitempty"`
	// networkInterfaceType specifies the type of network interface to be used for the primary
	// network interface.
	// Valid values are "ENA", "EFA", and omitted, which means no opinion and the platform
	// chooses a good default which may change over time.
	// The current default value is "ENA".
	// Please visit https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/efa.html to learn more
	// about the AWS Elastic Fabric Adapter interface option.
	// +kubebuilder:validation:Enum:="ENA";"EFA"
	// +optional
	NetworkInterfaceType AWSNetworkInterfaceType `json:"networkInterfaceType,omitempty"`
	// securityGroups is an array of references to security groups that should be applied to the
	// instance.
	// +optional
	SecurityGroups []AWSResourceReference `json:"securityGroups,omitempty"`
	// subnet is a reference to the subnet to use for this instance
	Subnet AWSResourceReference `json:"subnet"`
	// placement specifies where to create the instance in AWS
	Placement Placement `json:"placement"`
	// loadBalancers is the set of load balancers to which the new instance
	// should be added once it is created.
	// +optional
	LoadBalancers []LoadBalancerReference `json:"loadBalancers,omitempty"`
	// blockDevices is the set of block device mapping associated to this instance,
	// block device without a name will be used as a root device and only one device without a name is allowed
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/block-device-mapping-concepts.html
	// +optional
	BlockDevices []BlockDeviceMappingSpec `json:"blockDevices,omitempty"`
	// spotMarketOptions allows users to configure instances to be run using AWS Spot instances.
	// +optional
	SpotMarketOptions *SpotMarketOptions `json:"spotMarketOptions,omitempty"`
	// metadataServiceOptions allows users to configure instance metadata service interaction options.
	// If nothing specified, default AWS IMDS settings will be applied.
	// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceMetadataOptionsRequest.html
	// +optional
	MetadataServiceOptions MetadataServiceOptions `json:"metadataServiceOptions,omitempty"`
	// placementGroupName specifies the name of the placement group in which to launch the instance.
	// The placement group must already be created and may use any placement strategy.
	// When omitted, no placement group is used when creating the EC2 instance.
	// +optional
	PlacementGroupName string `json:"placementGroupName,omitempty"`
	// placementGroupPartition is the partition number within the placement group in which to launch the instance.
	// This must be an integer value between 1 and 7. It is only valid if the placement group, referred in
	// `PlacementGroupName` was created with strategy set to partition.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=7
	// +optional
	PlacementGroupPartition *int32 `json:"placementGroupPartition,omitempty"`
	// capacityReservationId specifies the target Capacity Reservation into which the instance should be launched.
	// The field size should be greater than 0 and the field input must start with cr-***
	// +optional
	CapacityReservationID string `json:"capacityReservationId"`
	// marketType specifies the type of market for the EC2 instance.
	// Valid values are OnDemand, Spot, CapacityBlock and omitted.
	//
	// Defaults to OnDemand.
	// When SpotMarketOptions is provided, the marketType defaults to "Spot".
	//
	// When set to OnDemand the instance runs as a standard OnDemand instance.
	// When set to Spot the instance runs as a Spot instance.
	// When set to CapacityBlock the instance utilizes pre-purchased compute capacity (capacity blocks) with AWS Capacity Reservations.
	// If this value is selected, capacityReservationID must be specified to identify the target reservation.
	// +optional
	MarketType MarketType `json:"marketType,omitempty"`

	// Tombstone: This field was moved into the Placement struct to belong w/ the Tenancy field due to involvement with the setting.
	// hostPlacement configures placement on AWS Dedicated Hosts. This allows admins to assign instances to specific host
	// for a variety of needs including for regulatory compliance, to leverage existing per-socket or per-core software licenses (BYOL),
	// and to gain visibility and control over instance placement on a physical server.
	// When omitted, the instance is not constrained to a dedicated host.
	// +openshift:enable:FeatureGate=AWSDedicatedHosts
	// +optional
	//HostPlacement *HostPlacement `json:"hostPlacement,omitempty"`
}

// AWSConfidentialComputePolicy represents the confidential compute configuration for the instance.
// +kubebuilder:validation:Enum=Disabled;AMDEncryptedVirtualizationNestedPaging
type AWSConfidentialComputePolicy string

const (
	// AWSConfidentialComputePolicyDisabled disables confidential computing for the instance.
	AWSConfidentialComputePolicyDisabled AWSConfidentialComputePolicy = "Disabled"
	// AWSConfidentialComputePolicySEVSNP enables AMD SEV-SNP as the confidential computing technology for the instance.
	AWSConfidentialComputePolicySEVSNP AWSConfidentialComputePolicy = "AMDEncryptedVirtualizationNestedPaging"
)

// CPUOptions defines CPU-related settings for the instance, including the confidential computing policy.
// If provided, it must not be empty — at least one field must be set.
// +kubebuilder:validation:MinProperties=1
type CPUOptions struct {
	// confidentialCompute specifies whether confidential computing should be enabled for the instance,
	// and, if so, which confidential computing technology to use.
	// Valid values are: Disabled, AMDEncryptedVirtualizationNestedPaging and omitted.
	// When set to Disabled, confidential computing will be disabled for the instance.
	// When set to AMDEncryptedVirtualizationNestedPaging, AMD SEV-SNP will be used as the confidential computing technology for the instance.
	// In this case, ensure the following conditions are met:
	// 1) The selected instance type supports AMD SEV-SNP.
	// 2) The selected AWS region supports AMD SEV-SNP.
	// 3) The selected AMI supports AMD SEV-SNP.
	// More details can be checked at https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/sev-snp.html
	// When omitted, this means no opinion and the AWS platform is left to choose a reasonable default,
	// which is subject to change without notice. The current default is Disabled.
	// +optional
	ConfidentialCompute *AWSConfidentialComputePolicy `json:"confidentialCompute,omitempty"`
}

// BlockDeviceMappingSpec describes a block device mapping
type BlockDeviceMappingSpec struct {
	// The device name exposed to the machine (for example, /dev/sdh or xvdh).
	// +optional
	DeviceName *string `json:"deviceName,omitempty"`
	// Parameters used to automatically set up EBS volumes when the machine is
	// launched.
	// +optional
	EBS *EBSBlockDeviceSpec `json:"ebs,omitempty"`
	// Suppresses the specified device included in the block device mapping of the
	// AMI.
	// +optional
	NoDevice *string `json:"noDevice,omitempty"`
	// The virtual device name (ephemeralN). Machine store volumes are numbered
	// starting from 0. An machine type with 2 available machine store volumes
	// can specify mappings for ephemeral0 and ephemeral1.The number of available
	// machine store volumes depends on the machine type. After you connect to
	// the machine, you must mount the volume.
	//
	// Constraints: For M3 machines, you must specify machine store volumes in
	// the block device mapping for the machine. When you launch an M3 machine,
	// we ignore any machine store volumes specified in the block device mapping
	// for the AMI.
	// +optional
	VirtualName *string `json:"virtualName,omitempty"`
}

// EBSBlockDeviceSpec describes a block device for an EBS volume.
// https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/EbsBlockDevice
type EBSBlockDeviceSpec struct {
	// Indicates whether the EBS volume is deleted on machine termination.
	//
	// Deprecated: setting this field has no effect.
	// +optional
	DeprecatedDeleteOnTermination *bool `json:"deleteOnTermination,omitempty"`
	// Indicates whether the EBS volume is encrypted. Encrypted Amazon EBS volumes
	// may only be attached to machines that support Amazon EBS encryption.
	// +optional
	Encrypted *bool `json:"encrypted,omitempty"`
	// Indicates the KMS key that should be used to encrypt the Amazon EBS volume.
	// +optional
	KMSKey AWSResourceReference `json:"kmsKey,omitempty"`
	// The number of I/O operations per second (IOPS) that the volume supports.
	// For io1, this represents the number of IOPS that are provisioned for the
	// volume. For gp2, this represents the baseline performance of the volume and
	// the rate at which the volume accumulates I/O credits for bursting. For more
	// information about General Purpose SSD baseline performance, I/O credits,
	// and bursting, see Amazon EBS Volume Types (http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html)
	// in the Amazon Elastic Compute Cloud User Guide.
	//
	// Minimal and maximal IOPS for io1 and gp2 are constrained. Please, check
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html
	// for precise boundaries for individual volumes.
	//
	// Condition: This parameter is required for requests to create io1 volumes;
	// it is not used in requests to create gp2, st1, sc1, or standard volumes.
	// +optional
	Iops *int64 `json:"iops,omitempty"`
	// throughputMib to provision in MiB/s supported for the volume type. Not applicable to all types.
	//
	// This parameter is valid only for gp3 volumes.
	// Valid Range: Minimum value of 125. Maximum value of 2000.
	//
	// When omitted, this means no opinion, and the platform is left to
	// choose a reasonable default, which is subject to change over time.
	// The current default is 125.
	//
	// +kubebuilder:validation:Minimum:=125
	// +kubebuilder:validation:Maximum:=2000
	// +optional
	ThroughputMib *int32 `json:"throughputMib,omitempty"`
	// The size of the volume, in GiB.
	//
	// Constraints: 1-16384 for General Purpose SSD (gp2), 4-16384 for Provisioned
	// IOPS SSD (io1), 500-16384 for Throughput Optimized HDD (st1), 500-16384 for
	// Cold HDD (sc1), and 1-1024 for Magnetic (standard) volumes. If you specify
	// a snapshot, the volume size must be equal to or larger than the snapshot
	// size.
	//
	// Default: If you're creating the volume from a snapshot and don't specify
	// a volume size, the default is the snapshot size.
	// +optional
	VolumeSize *int64 `json:"volumeSize,omitempty"`
	// volumeType can be of type gp2, gp3, io1, st1, sc1, or standard.
	// Default: standard
	// +optional
	VolumeType *string `json:"volumeType,omitempty"`
}

// SpotMarketOptions defines the options available to a user when configuring
// Machines to run on Spot instances.
// Most users should provide an empty struct.
type SpotMarketOptions struct {
	// The maximum price the user is willing to pay for their instances
	// Default: On-Demand price
	// +optional
	MaxPrice *string `json:"maxPrice,omitempty"`
}

type MetadataServiceAuthentication string

const (
	// MetadataServiceAuthenticationRequired enforces sending of a signed token header with any instance metadata retrieval (GET) requests.
	// Enforces IMDSv2 usage.
	MetadataServiceAuthenticationRequired = "Required"
	// MetadataServiceAuthenticationOptional allows IMDSv1 usage along with IMDSv2
	MetadataServiceAuthenticationOptional = "Optional"
)

// MetadataServiceOptions defines the options available to a user when configuring
// Instance Metadata Service (IMDS) Options.
type MetadataServiceOptions struct {
	// authentication determines whether or not the host requires the use of authentication when interacting with the metadata service.
	// When using authentication, this enforces v2 interaction method (IMDSv2) with the metadata service.
	// When omitted, this means the user has no opinion and the value is left to the platform to choose a good
	// default, which is subject to change over time. The current default is optional.
	// At this point this field represents `HttpTokens` parameter from `InstanceMetadataOptionsRequest` structure in AWS EC2 API
	// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceMetadataOptionsRequest.html
	// +kubebuilder:validation:Enum=Required;Optional
	// +optional
	Authentication MetadataServiceAuthentication `json:"authentication,omitempty"`
}

// AWSResourceReference is a reference to a specific AWS resource by ID, ARN, or filters.
// Only one of ID, ARN or Filters may be specified. Specifying more than one will result in
// a validation error.
type AWSResourceReference struct {
	// id of resource
	// +optional
	ID *string `json:"id,omitempty"`
	// arn of resource
	// +optional
	ARN *string `json:"arn,omitempty"`
	// filters is a set of filters used to identify a resource
	// +optional
	Filters []Filter `json:"filters,omitempty"`
}

// Placement indicates where to create the instance in AWS
// +kubebuilder:validation:XValidation:rule="has(self.tenancy) && self.tenancy == 'host' ? true : !has(self.host)",message="host may only be specified when tenancy is host"
type Placement struct {
	// region is the region to use to create the instance
	// +optional
	Region string `json:"region,omitempty"`
	// availabilityZone is the availability zone of the instance
	// +optional
	AvailabilityZone string `json:"availabilityZone,omitempty"`
	// tenancy indicates if instance should run on shared or single-tenant hardware. There are
	// supported 3 options: default, dedicated and host.
	// When set to default Runs on shared multi-tenant hardware.
	// When dedicated Runs on single-tenant hardware (any dedicated instance hardware).
	// When host and the host object is not provided: Runs on Dedicated Host; best-effort restart on same host.
	// When `host` and `host` object is provided with affinity `dedicatedHost` defined: Runs on specified Dedicated Host.
	// +optional
	Tenancy InstanceTenancy `json:"tenancy,omitempty"`
	// host configures placement on AWS Dedicated Hosts. This allows admins to assign instances to specific host
	// for a variety of needs including for regulatory compliance, to leverage existing per-socket or per-core software licenses (BYOL),
	// and to gain visibility and control over instance placement on a physical server.
	// When omitted, the instance is not constrained to a dedicated host.
	// +openshift:enable:FeatureGate=AWSDedicatedHosts
	// +optional
	Host *HostPlacement `json:"host,omitempty"`
}

// Filter is a filter used to identify an AWS resource
type Filter struct {
	// name of the filter. Filter names are case-sensitive.
	Name string `json:"name"`
	// values includes one or more filter values. Filter values are case-sensitive.
	// +optional
	Values []string `json:"values,omitempty"`
}

// TagSpecification is the name/value pair for a tag
type TagSpecification struct {
	// name of the tag.
	// This field is required and must be a non-empty string.
	// Must be between 1 and 128 characters in length.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +required
	Name string `json:"name"`
	// value of the tag.
	// When omitted, this creates a tag with an empty string as the value.
	// +optional
	Value string `json:"value"`
}

// AWSMachineProviderConfigList contains a list of AWSMachineProviderConfig
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type AWSMachineProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AWSMachineProviderConfig `json:"items"`
}

// LoadBalancerReference is a reference to a load balancer on AWS.
type LoadBalancerReference struct {
	Name string              `json:"name"`
	Type AWSLoadBalancerType `json:"type"`
}

// AWSLoadBalancerType is the type of LoadBalancer to use when registering
// an instance with load balancers specified in LoadBalancerNames
type AWSLoadBalancerType string

// InstanceTenancy indicates if instance should run on shared or single-tenant hardware.
type InstanceTenancy string

const (
	// DefaultTenancy instance runs on shared hardware
	DefaultTenancy InstanceTenancy = "default"
	// DedicatedTenancy instance runs on single-tenant hardware
	DedicatedTenancy InstanceTenancy = "dedicated"
	// HostTenancy instance runs on a Dedicated Host, which is an isolated server with configurations that you can control.
	HostTenancy InstanceTenancy = "host"
)

// Possible values for AWSLoadBalancerType. Add to this list as other types
// of load balancer are supported by the actuator.
const (
	ClassicLoadBalancerType AWSLoadBalancerType = "classic" // AWS classic ELB
	NetworkLoadBalancerType AWSLoadBalancerType = "network" // AWS Network Load Balancer (NLB)
)

// AWSNetworkInterfaceType defines the network interface type of the the
// AWS EC2 network interface.
type AWSNetworkInterfaceType string

const (
	// AWSENANetworkInterfaceType is the default network interface type,
	// the EC2 Elastic Network Adapter commonly used with EC2 instances.
	// This should be used for standard network operations.
	AWSENANetworkInterfaceType AWSNetworkInterfaceType = "ENA"
	// AWSEFANetworkInterfaceType is the Elastic Fabric Adapter network interface type.
	AWSEFANetworkInterfaceType AWSNetworkInterfaceType = "EFA"
)

// AWSMachineProviderStatus is the type that will be embedded in a Machine.Status.ProviderStatus field.
// It contains AWS-specific status information.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type AWSMachineProviderStatus struct {
	metav1.TypeMeta `json:",inline"`
	// instanceId is the instance ID of the machine created in AWS
	// +optional
	InstanceID *string `json:"instanceId,omitempty"`
	// instanceState is the state of the AWS instance for this machine
	// +optional
	InstanceState *string `json:"instanceState,omitempty"`
	// conditions is a set of conditions associated with the Machine to indicate
	// errors or other status
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// dedicatedHost tracks the dynamically allocated dedicated host.
	// This field is populated when allocationStrategy is Dynamic (with or without DynamicHostAllocation).
	// When omitted, this indicates that the dedicated host has not yet been allocated, or allocation is in progress.
	// +optional
	DedicatedHost *DedicatedHostStatus `json:"dedicatedHost,omitempty"`
}

// DedicatedHostStatus defines the observed state of a dynamically allocated dedicated host
// associated with an AWSMachine. This struct is used to track the ID of the dedicated host.
type DedicatedHostStatus struct {
	// id tracks the dynamically allocated dedicated host ID.
	// This field is populated when allocationStrategy is Dynamic (with or without DynamicHostAllocation).
	// The value must start with "h-" followed by either 8 or 17 lowercase hexadecimal characters (0-9 and a-f).
	// The use of 8 lowercase hexadecimal characters is for older legacy hosts that may not have been migrated to newer format.
	// Must be either 10 or 19 characters in length.
	// +kubebuilder:validation:XValidation:rule="self.matches('^h-([0-9a-f]{8}|[0-9a-f]{17})$')",message="id must start with 'h-' followed by either 8 or 17 lowercase hexadecimal characters (0-9 and a-f)"
	// +kubebuilder:validation:MinLength=10
	// +kubebuilder:validation:MaxLength=19
	// +required
	ID string `json:"id,omitempty"`
}

// MarketType describes the market type of an EC2 Instance
// +kubebuilder:validation:Enum:=OnDemand;Spot;CapacityBlock
type MarketType string

const (

	// MarketTypeOnDemand is a MarketType enum value
	// When set to OnDemand the instance runs as a standard OnDemand instance.
	MarketTypeOnDemand MarketType = "OnDemand"

	// MarketTypeSpot is a MarketType enum value
	// When set to Spot the instance runs as a Spot instance.
	MarketTypeSpot MarketType = "Spot"

	// MarketTypeCapacityBlock is a MarketType enum value
	// When set to CapacityBlock the instance utilizes pre-purchased compute capacity (capacity blocks) with AWS Capacity Reservations.
	MarketTypeCapacityBlock MarketType = "CapacityBlock"
)

// HostPlacement is the type that will be used to configure the placement of AWS instances.
// +kubebuilder:validation:XValidation:rule="has(self.affinity) && self.affinity == 'DedicatedHost' ? has(self.dedicatedHost) : true",message="dedicatedHost is required when affinity is DedicatedHost, and optional otherwise"
// +union
type HostPlacement struct {
	// affinity specifies the affinity setting for the instance.
	// Allowed values are AnyAvailable and DedicatedHost.
	// When Affinity is set to DedicatedHost, an instance started onto a specific host always restarts on the same host if stopped. In this scenario, the `dedicatedHost` field must be set.
	// When Affinity is set to AnyAvailable, and you stop and restart the instance, it can be restarted on any available host.
	// When Affinity is set to AnyAvailable and the `dedicatedHost` field is defined, it runs on specified Dedicated Host, but may move if stopped.
	// +required
	// +unionDiscriminator
	Affinity *HostAffinity `json:"affinity,omitempty"`

	// dedicatedHost specifies the exact host that an instance should be restarted on if stopped.
	// dedicatedHost is required when 'affinity' is set to DedicatedHost, and optional otherwise.
	// +optional
	// +unionMember
	DedicatedHost *DedicatedHost `json:"dedicatedHost,omitempty"`
}

// HostAffinity selects how an instance should be placed on AWS Dedicated Hosts.
// +kubebuilder:validation:Enum:=DedicatedHost;AnyAvailable
type HostAffinity string

const (
	// HostAffinityAnyAvailable lets the platform select any available dedicated host.

	HostAffinityAnyAvailable HostAffinity = "AnyAvailable"

	// HostAffinityDedicatedHost requires specifying a particular host via dedicatedHost.host.hostID.
	HostAffinityDedicatedHost HostAffinity = "DedicatedHost"
)

// AllocationStrategy selects how a dedicated host is provided to the system for assigning to the instance.
// +kubebuilder:validation:Enum:=UserProvided;Dynamic
// +enum
type AllocationStrategy string

const (
	// AllocationStrategyUserProvided specifies that the system should assign instances to a user-provided dedicated host.
	AllocationStrategyUserProvided AllocationStrategy = "UserProvided"

	// AllocationStrategyDynamic specifies that the system should dynamically allocate a dedicated host for instances.
	AllocationStrategyDynamic AllocationStrategy = "Dynamic"
)

// DedicatedHost represents the configuration for the usage of dedicated host.
// +kubebuilder:validation:XValidation:rule="self.allocationStrategy == 'UserProvided' ? has(self.id) : !has(self.id)",message="id is required when allocationStrategy is UserProvided, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="has(self.dynamicHostAllocation) ? self.allocationStrategy == 'Dynamic' : true",message="dynamicHostAllocation is only allowed when allocationStrategy is Dynamic"
// +union
type DedicatedHost struct {
	// allocationStrategy specifies if the dedicated host will be provided by the admin through the id field or if the host will be dynamically allocated.
	// Valid values are UserProvided and Dynamic.
	// When omitted, the value defaults to "UserProvided", which requires the id field to be set.
	// When allocationStrategy is set to UserProvided, an ID of the dedicated host to assign must be provided.
	// When allocationStrategy is set to Dynamic, a dedicated host will be allocated and used to assign instances.
	// When allocationStrategy is set to Dynamic, and dynamicHostAllocation is configured, a dedicated host will be allocated and the tags in dynamicHostAllocation will be assigned to that host.
	// +optional
	// +unionDiscriminator
	// +default="UserProvided"
	AllocationStrategy *AllocationStrategy `json:"allocationStrategy,omitempty"`

	// id identifies the AWS Dedicated Host on which the instance must run.
	// The value must start with "h-" followed by either 8 or 17 lowercase hexadecimal characters (0-9 and a-f).
	// The use of 8 lowercase hexadecimal characters is for older legacy hosts that may not have been migrated to newer format.
	// Must be either 10 or 19 characters in length.
	// This field is required when allocationStrategy is UserProvided, and forbidden otherwise.
	// When omitted with allocationStrategy set to Dynamic, the platform will dynamically allocate a dedicated host.
	// +kubebuilder:validation:XValidation:rule="self.matches('^h-([0-9a-f]{8}|[0-9a-f]{17})$')",message="id must start with 'h-' followed by either 8 or 17 lowercase hexadecimal characters (0-9 and a-f)"
	// +kubebuilder:validation:MinLength=10
	// +kubebuilder:validation:MaxLength=19
	// +optional
	// +unionMember=UserProvided
	ID string `json:"id,omitempty"`

	// dynamicHostAllocation specifies tags to apply to a dynamically allocated dedicated host.
	// This field is only allowed when allocationStrategy is Dynamic, and is mutually exclusive with id.
	// When specified, a dedicated host will be allocated with the provided tags applied.
	// When omitted (and allocationStrategy is Dynamic), a dedicated host will be allocated without any additional tags.
	// +optional
	// +unionMember=Dynamic
	DynamicHostAllocation *DynamicHostAllocationSpec `json:"dynamicHostAllocation,omitempty"`
}

// DynamicHostAllocationSpec defines the configuration for dynamic dedicated host allocation.
// This specification always allocates exactly one dedicated host per machine.
// At least one property must be specified when this struct is used.
// Currently only Tags are available for configuring, but in the future more configs may become available.
// +kubebuilder:validation:MinProperties=1
type DynamicHostAllocationSpec struct {
	// tags specifies a set of key-value pairs to apply to the allocated dedicated host.
	// When omitted, no additional user-defined tags will be applied to the allocated host.
	// A maximum of 50 tags can be specified.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	// +listType=map
	// +listMapKey=name
	// +optional
	Tags *[]TagSpecification `json:"tags,omitempty"`
}
