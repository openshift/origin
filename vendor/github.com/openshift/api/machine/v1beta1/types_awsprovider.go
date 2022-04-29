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
	// AMI is the reference to the AMI from which to create the machine instance.
	AMI AWSResourceReference `json:"ami"`
	// InstanceType is the type of instance to create. Example: m4.xlarge
	InstanceType string `json:"instanceType"`
	// Tags is the set of tags to add to apply to an instance, in addition to the ones
	// added by default by the actuator. These tags are additive. The actuator will ensure
	// these tags are present, but will not remove any other tags that may exist on the
	// instance.
	// +optional
	Tags []TagSpecification `json:"tags,omitempty"`
	// IAMInstanceProfile is a reference to an IAM role to assign to the instance
	// +optional
	IAMInstanceProfile *AWSResourceReference `json:"iamInstanceProfile,omitempty"`
	// UserDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance
	// +optional
	UserDataSecret *corev1.LocalObjectReference `json:"userDataSecret,omitempty"`
	// CredentialsSecret is a reference to the secret with AWS credentials. Otherwise, defaults to permissions
	// provided by attached IAM role where the actuator is running.
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
	// KeyName is the name of the KeyPair to use for SSH
	// +optional
	KeyName *string `json:"keyName,omitempty"`
	// DeviceIndex is the index of the device on the instance for the network interface attachment.
	// Defaults to 0.
	DeviceIndex int64 `json:"deviceIndex"`
	// PublicIP specifies whether the instance should get a public IP. If not present,
	// it should use the default of its subnet.
	// +optional
	PublicIP *bool `json:"publicIp,omitempty"`
	// NetworkInterfaceType specifies the type of network interface to be used for the primary
	// network interface.
	// Valid values are "ENA", "EFA", and omitted, which means no opinion and the platform
	// chooses a good default which may change over time.
	// The current default value is "ENA".
	// Please visit https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/efa.html to learn more
	// about the AWS Elastic Fabric Adapter interface option.
	// +kubebuilder:validation:Enum:="ENA";"EFA"
	// +optional
	NetworkInterfaceType AWSNetworkInterfaceType `json:"networkInterfaceType,omitempty"`
	// SecurityGroups is an array of references to security groups that should be applied to the
	// instance.
	// +optional
	SecurityGroups []AWSResourceReference `json:"securityGroups,omitempty"`
	// Subnet is a reference to the subnet to use for this instance
	Subnet AWSResourceReference `json:"subnet"`
	// Placement specifies where to create the instance in AWS
	Placement Placement `json:"placement"`
	// LoadBalancers is the set of load balancers to which the new instance
	// should be added once it is created.
	// +optional
	LoadBalancers []LoadBalancerReference `json:"loadBalancers,omitempty"`
	// BlockDevices is the set of block device mapping associated to this instance,
	// block device without a name will be used as a root device and only one device without a name is allowed
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/block-device-mapping-concepts.html
	// +optional
	BlockDevices []BlockDeviceMappingSpec `json:"blockDevices,omitempty"`
	// SpotMarketOptions allows users to configure instances to be run using AWS Spot instances.
	// +optional
	SpotMarketOptions *SpotMarketOptions `json:"spotMarketOptions,omitempty"`
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
	// +optional
	DeleteOnTermination *bool `json:"deleteOnTermination,omitempty"`
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
	// The volume type: gp2, io1, st1, sc1, or standard.
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

// AWSResourceReference is a reference to a specific AWS resource by ID, ARN, or filters.
// Only one of ID, ARN or Filters may be specified. Specifying more than one will result in
// a validation error.
type AWSResourceReference struct {
	// ID of resource
	// +optional
	ID *string `json:"id,omitempty"`
	// ARN of resource
	// +optional
	ARN *string `json:"arn,omitempty"`
	// Filters is a set of filters used to identify a resource
	// +optional
	Filters []Filter `json:"filters,omitempty"`
}

// Placement indicates where to create the instance in AWS
type Placement struct {
	// Region is the region to use to create the instance
	// +optional
	Region string `json:"region,omitempty"`
	// AvailabilityZone is the availability zone of the instance
	// +optional
	AvailabilityZone string `json:"availabilityZone,omitempty"`
	// Tenancy indicates if instance should run on shared or single-tenant hardware. There are
	// supported 3 options: default, dedicated and host.
	// +optional
	Tenancy InstanceTenancy `json:"tenancy,omitempty"`
	// Group specifies a reference to an AWSPlacementGroup resource to create the Machine within.
	// If the group specified does not exist, the Machine will not be created and will enter the failed phase.
	// +optional
	Group LocalAWSPlacementGroupReference `json:"group,omitempty"`
	// PartitionNumber specifies the numbered partition in which instances should be launched.
	// It is recommended to only use this value if multiple MachineSets share
	// a single Placement Group, in which case, each MachineSet should represent an individual partition number.
	// If unset, when a Partition placement group is used, AWS will attempt to
	// distribute instances evenly between partitions.
	// If PartitionNumber is set when used with a non Partition type Placement Group, this will be considered an error.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=7
	// +optional
	PartitionNumber int32 `json:"partitionNumber,omitempty"`
}

// LocalAWSPlacementGroupReference contains enough information to let you locate the
// referenced AWSPlacementGroup inside the same namespace.
// +structType=atomic
type LocalAWSPlacementGroupReference struct {
	// Name of the AWSPlacementGroup.
	// +kubebuilder:validation:=Required
	Name string `json:"name"`
}

// Filter is a filter used to identify an AWS resource
type Filter struct {
	// Name of the filter. Filter names are case-sensitive.
	Name string `json:"name"`
	// Values includes one or more filter values. Filter values are case-sensitive.
	// +optional
	Values []string `json:"values,omitempty"`
}

// TagSpecification is the name/value pair for a tag
type TagSpecification struct {
	// Name of the tag
	Name string `json:"name"`
	// Value of the tag
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
	// InstanceID is the instance ID of the machine created in AWS
	// +optional
	InstanceID *string `json:"instanceId,omitempty"`
	// InstanceState is the state of the AWS instance for this machine
	// +optional
	InstanceState *string `json:"instanceState,omitempty"`
	// Conditions is a set of conditions associated with the Machine to indicate
	// errors or other status
	// +optional
	Conditions []AWSMachineProviderCondition `json:"conditions,omitempty"`
}

// AWSMachineProviderCondition is a condition in a AWSMachineProviderStatus.
type AWSMachineProviderCondition struct {
	// Type is the type of the condition.
	Type ConditionType `json:"type"`
	// Status is the status of the condition.
	Status corev1.ConditionStatus `json:"status"`
	// LastProbeTime is the last time we probed the condition.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
	// LastTransitionTime is the last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// Message is a human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty"`
}
