package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AWSMachineProviderConfig is the Schema for the awsmachineproviderconfigs API
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type AWSMachineProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// AMI is the reference to the AMI from which to create the machine instance.
	AMI AWSResourceReference `json:"ami" protobuf:"bytes,2,opt,name=ami"`

	// InstanceType is the type of instance to create. Example: m4.xlarge
	InstanceType string `json:"instanceType" protobuf:"bytes,3,opt,name=instanceType"`

	// Tags is the set of tags to add to apply to an instance, in addition to the ones
	// added by default by the actuator. These tags are additive. The actuator will ensure
	// these tags are present, but will not remove any other tags that may exist on the
	// instance.
	Tags []TagSpecification `json:"tags,omitempty" protobuf:"bytes,4,rep,name=tags"`

	// IAMInstanceProfile is a reference to an IAM role to assign to the instance
	IAMInstanceProfile *AWSResourceReference `json:"iamInstanceProfile,omitempty" protobuf:"bytes,5,opt,name=iamInstanceProfile"`

	// UserDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance
	UserDataSecret *corev1.LocalObjectReference `json:"userDataSecret,omitempty" protobuf:"bytes,6,opt,name=userDataSecret"`

	// CredentialsSecret is a reference to the secret with AWS credentials. Otherwise, defaults to permissions
	// provided by attached IAM role where the actuator is running.
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty" protobuf:"bytes,7,opt,name=credentialsSecret"`

	// KeyName is the name of the KeyPair to use for SSH
	KeyName *string `json:"keyName,omitempty" protobuf:"bytes,8,opt,name=keyName"`

	// DeviceIndex is the index of the device on the instance for the network interface attachment.
	// Defaults to 0.
	DeviceIndex int64 `json:"deviceIndex" protobuf:"varint,9,opt,name=deviceIndex"`

	// PublicIP specifies whether the instance should get a public IP. If not present,
	// it should use the default of its subnet.
	PublicIP *bool `json:"publicIp,omitempty" protobuf:"varint,10,opt,name=publicIp"`

	// SecurityGroups is an array of references to security groups that should be applied to the
	// instance.
	SecurityGroups []AWSResourceReference `json:"securityGroups,omitempty" protobuf:"bytes,11,rep,name=securityGroups"`

	// Subnet is a reference to the subnet to use for this instance
	Subnet AWSResourceReference `json:"subnet" protobuf:"bytes,12,opt,name=subnet"`

	// Placement specifies where to create the instance in AWS
	Placement Placement `json:"placement" protobuf:"bytes,13,opt,name=placement"`

	// LoadBalancers is the set of load balancers to which the new instance
	// should be added once it is created.
	LoadBalancers []LoadBalancerReference `json:"loadBalancers,omitempty" protobuf:"bytes,14,rep,name=loadBalancers"`

	// BlockDevices is the set of block device mapping associated to this instance,
	// block device without a name will be used as a root device and only one device without a name is allowed
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/block-device-mapping-concepts.html
	BlockDevices []BlockDeviceMappingSpec `json:"blockDevices,omitempty" protobuf:"bytes,15,rep,name=blockDevices"`

	// SpotMarketOptions allows users to configure instances to be run using AWS Spot instances.
	SpotMarketOptions *SpotMarketOptions `json:"spotMarketOptions,omitempty" protobuf:"bytes,16,opt,name=spotMarketOptions"`
}

// BlockDeviceMappingSpec describes a block device mapping
type BlockDeviceMappingSpec struct {

	// The device name exposed to the machine (for example, /dev/sdh or xvdh).
	DeviceName *string `json:"deviceName,omitempty" protobuf:"bytes,1,opt,name=deviceName"`

	// Parameters used to automatically set up EBS volumes when the machine is
	// launched.
	EBS *EBSBlockDeviceSpec `json:"ebs,omitempty" protobuf:"bytes,2,opt,name=ebs"`

	// Suppresses the specified device included in the block device mapping of the
	// AMI.
	NoDevice *string `json:"noDevice,omitempty" protobuf:"bytes,3,opt,name=noDevice"`

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
	VirtualName *string `json:"virtualName,omitempty" protobuf:"bytes,4,opt,name=virtualName"`
}

// EBSBlockDeviceSpec describes a block device for an EBS volume.
// https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/EbsBlockDevice
type EBSBlockDeviceSpec struct {

	// Indicates whether the EBS volume is deleted on machine termination.
	DeleteOnTermination *bool `json:"deleteOnTermination,omitempty" protobuf:"varint,1,opt,name=deleteOnTermination"`

	// Indicates whether the EBS volume is encrypted. Encrypted Amazon EBS volumes
	// may only be attached to machines that support Amazon EBS encryption.
	Encrypted *bool `json:"encrypted,omitempty" protobuf:"varint,2,opt,name=encrypted"`

	// Indicates the KMS key that should be used to encrypt the Amazon EBS volume.
	KMSKey AWSResourceReference `json:"kmsKey,omitempty" protobuf:"bytes,3,opt,name=kmsKey"`

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
	Iops *int64 `json:"iops,omitempty" protobuf:"varint,4,opt,name=iops"`

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
	VolumeSize *int64 `json:"volumeSize,omitempty" protobuf:"varint,5,opt,name=volumeSize"`

	// The volume type: gp2, io1, st1, sc1, or standard.
	// Default: standard
	VolumeType *string `json:"volumeType,omitempty" protobuf:"bytes,6,opt,name=volumeType"`
}

// SpotMarketOptions defines the options available to a user when configuring
// Machines to run on Spot instances.
// Most users should provide an empty struct.
type SpotMarketOptions struct {
	// The maximum price the user is willing to pay for their instances
	// Default: On-Demand price
	MaxPrice *string `json:"maxPrice,omitempty" protobuf:"bytes,1,opt,name=maxPrice"`
}

// AWSResourceReference is a reference to a specific AWS resource by ID, ARN, or filters.
// Only one of ID, ARN or Filters may be specified. Specifying more than one will result in
// a validation error.
type AWSResourceReference struct {
	// ID of resource
	// +optional
	ID *string `json:"id,omitempty" protobuf:"bytes,1,opt,name=id"`

	// ARN of resource
	// +optional
	ARN *string `json:"arn,omitempty" protobuf:"bytes,2,opt,name=arn"`

	// Filters is a set of filters used to identify a resource
	Filters []Filter `json:"filters,omitempty" protobuf:"bytes,3,rep,name=filters"`
}

// Placement indicates where to create the instance in AWS
type Placement struct {
	// Region is the region to use to create the instance
	Region string `json:"region,omitempty" protobuf:"bytes,1,opt,name=region"`

	// AvailabilityZone is the availability zone of the instance
	AvailabilityZone string `json:"availabilityZone,omitempty" protobuf:"bytes,2,opt,name=availabilityZone"`

	// Tenancy indicates if instance should run on shared or single-tenant hardware. There are
	// supported 3 options: default, dedicated and host.
	Tenancy InstanceTenancy `json:"tenancy,omitempty" protobuf:"bytes,3,opt,name=tenancy,casttype=InstanceTenancy"`
}

// Filter is a filter used to identify an AWS resource
type Filter struct {
	// Name of the filter. Filter names are case-sensitive.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Values includes one or more filter values. Filter values are case-sensitive.
	Values []string `json:"values,omitempty" protobuf:"bytes,2,rep,name=values"`
}

// TagSpecification is the name/value pair for a tag
type TagSpecification struct {
	// Name of the tag
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Value of the tag
	Value string `json:"value" protobuf:"bytes,2,opt,name=value"`
}

// AWSMachineProviderConfigList contains a list of AWSMachineProviderConfig
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type AWSMachineProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []AWSMachineProviderConfig `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// LoadBalancerReference is a reference to a load balancer on AWS.
type LoadBalancerReference struct {
	Name string              `json:"name" protobuf:"bytes,1,opt,name=name"`
	Type AWSLoadBalancerType `json:"type" protobuf:"bytes,2,opt,name=type,casttype=AWSLoadBalancerType"`
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

// AWSMachineProviderStatus is the type that will be embedded in a Machine.Status.ProviderStatus field.
// It contains AWS-specific status information.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type AWSMachineProviderStatus struct {
	metav1.TypeMeta `json:",inline"`

	// InstanceID is the instance ID of the machine created in AWS
	// +optional
	InstanceID *string `json:"instanceId,omitempty" protobuf:"bytes,1,opt,name=instanceId"`

	// InstanceState is the state of the AWS instance for this machine
	// +optional
	InstanceState *string `json:"instanceState,omitempty" protobuf:"bytes,2,opt,name=instanceState"`

	// Conditions is a set of conditions associated with the Machine to indicate
	// errors or other status
	Conditions []AWSMachineProviderCondition `json:"conditions,omitempty" protobuf:"bytes,3,rep,name=conditions"`
}

// AWSMachineProviderConditionType is a valid value for AWSMachineProviderCondition.Type
type AWSMachineProviderConditionType string

// Valid conditions for an AWS machine instance.
const (
	// MachineCreation indicates whether the machine has been created or not. If not,
	// it should include a reason and message for the failure.
	MachineCreation AWSMachineProviderConditionType = "MachineCreation"
)

// AWSMachineProviderConditionReason is reason for the condition's last transition.
type AWSMachineProviderConditionReason string

const (
	// MachineCreationSucceeded indicates machine creation success.
	MachineCreationSucceeded AWSMachineProviderConditionReason = "MachineCreationSucceeded"
	// MachineCreationFailed indicates machine creation failure.
	MachineCreationFailed AWSMachineProviderConditionReason = "MachineCreationFailed"
)

// AWSMachineProviderCondition is a condition in a AWSMachineProviderStatus.
type AWSMachineProviderCondition struct {
	// Type is the type of the condition.
	Type AWSMachineProviderConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=AWSMachineProviderConditionType"`
	// Status is the status of the condition.
	Status corev1.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/api/core/v1.ConditionStatus"`
	// LastProbeTime is the last time we probed the condition.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty" protobuf:"bytes,3,opt,name=lastProbeTime"`
	// LastTransitionTime is the last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,opt,name=lastTransitionTime"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason AWSMachineProviderConditionReason `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason,casttype=AWSMachineProviderConditionReason"`
	// Message is a human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}
