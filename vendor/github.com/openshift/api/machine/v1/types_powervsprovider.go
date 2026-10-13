package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PowerVSResourceType enum attribute to identify the type of resource reference
type PowerVSResourceType string

// PowerVSProcessorType enum attribute to identify the PowerVS instance processor type
type PowerVSProcessorType string

// IBMVPCLoadBalancerType is the type of LoadBalancer to use when registering
// an instance with load balancers specified in LoadBalancerNames
type IBMVPCLoadBalancerType string

// ApplicationLoadBalancerType is possible values for IBMVPCLoadBalancerType.
const (
	ApplicationLoadBalancerType IBMVPCLoadBalancerType = "Application" // Application Load Balancer for VPC (ALB)
)

const (
	// PowerVSResourceTypeID enum property to identify an ID type resource reference
	PowerVSResourceTypeID PowerVSResourceType = "ID"
	// PowerVSResourceTypeName enum property to identify a Name type resource reference
	PowerVSResourceTypeName PowerVSResourceType = "Name"
	// PowerVSResourceTypeRegEx enum property to identify a tags type resource reference
	PowerVSResourceTypeRegEx PowerVSResourceType = "RegEx"
	// PowerVSProcessorTypeDedicated enum property to identify a Dedicated Power VS processor type
	PowerVSProcessorTypeDedicated PowerVSProcessorType = "Dedicated"
	// PowerVSProcessorTypeShared enum property to identify a Shared Power VS processor type
	PowerVSProcessorTypeShared PowerVSProcessorType = "Shared"
	// PowerVSProcessorTypeCapped enum property to identify a Capped Power VS processor type
	PowerVSProcessorTypeCapped PowerVSProcessorType = "Capped"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PowerVSMachineProviderConfig is the type that will be embedded in a Machine.Spec.ProviderSpec field
// for a PowerVS virtual machine. It is used by the PowerVS machine actuator to create a single Machine.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +k8s:openapi-gen=true
type PowerVSMachineProviderConfig struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// userDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance.
	// +optional
	UserDataSecret *PowerVSSecretReference `json:"userDataSecret,omitempty"`

	// credentialsSecret is a reference to the secret with IBM Cloud credentials.
	// +optional
	CredentialsSecret *PowerVSSecretReference `json:"credentialsSecret,omitempty"`

	// serviceInstance is the reference to the Power VS service on which the server instance(VM) will be created.
	// Power VS service is a container for all Power VS instances at a specific geographic region.
	// serviceInstance can be created via IBM Cloud catalog or CLI.
	// supported serviceInstance identifier in PowerVSResource are Name and ID and that can be obtained from IBM Cloud UI or IBM Cloud cli.
	// More detail about Power VS service instance.
	// https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server
	// +kubebuilder:validation:=Required
	ServiceInstance PowerVSResource `json:"serviceInstance"`

	// image is to identify the rhcos image uploaded to IBM COS bucket which is used to create the instance.
	// supported image identifier in PowerVSResource are Name and ID and that can be obtained from IBM Cloud UI or IBM Cloud cli.
	// +kubebuilder:validation:=Required
	Image PowerVSResource `json:"image"`

	// network is the reference to the Network to use for this instance.
	// supported network identifier in PowerVSResource are Name, ID and RegEx and that can be obtained from IBM Cloud UI or IBM Cloud cli.
	// +kubebuilder:validation:=Required
	Network PowerVSResource `json:"network"`

	// keyPairName is the name of the KeyPair to use for SSH.
	// The key pair will be exposed to the instance via the instance metadata service.
	// On boot, the OS will copy the public keypair into the authorized keys for the core user.
	// +kubebuilder:validation:=Required
	KeyPairName string `json:"keyPairName"`

	// systemType is the System type used to host the instance.
	// systemType determines the number of cores and memory that is available.
	// Few of the supported SystemTypes are s922,e880,e980.
	// e880 systemType available only in Dallas Datacenters.
	// e980 systemType available in Datacenters except Dallas and Washington.
	// When omitted, this means that the user has no opinion and the platform is left to choose a
	// reasonable default, which is subject to change over time. The current default is s922 which is generally available.
	// + This is not an enum because we expect other values to be added later which should be supported implicitly.
	// +optional
	SystemType string `json:"systemType,omitempty"`

	// processorType is the VM instance processor type.
	// It must be set to one of the following values: Dedicated, Capped or Shared.
	// Dedicated: resources are allocated for a specific client, The hypervisor makes a 1:1 binding of a partition’s processor to a physical processor core.
	// Shared: Shared among other clients.
	// Capped: Shared, but resources do not expand beyond those that are requested, the amount of CPU time is Capped to the value specified for the entitlement.
	// if the processorType is selected as Dedicated, then processors value cannot be fractional.
	// When omitted, this means that the user has no opinion and the platform is left to choose a
	// reasonable default, which is subject to change over time. The current default is Shared.
	// +kubebuilder:validation:Enum:="Dedicated";"Shared";"Capped";""
	// +optional
	ProcessorType PowerVSProcessorType `json:"processorType,omitempty"`

	// processors is the number of virtual processors in a virtual machine.
	// when the processorType is selected as Dedicated the processors value cannot be fractional.
	// maximum value for the Processors depends on the selected SystemType.
	// when SystemType is set to e880 or e980 maximum Processors value is 143.
	// when SystemType is set to s922 maximum Processors value is 15.
	// minimum value for Processors depends on the selected ProcessorType.
	// when ProcessorType is set as Shared or Capped, The minimum processors is 0.5.
	// when ProcessorType is set as Dedicated, The minimum processors is 1.
	// When omitted, this means that the user has no opinion and the platform is left to choose a
	// reasonable default, which is subject to change over time. The default is set based on the selected ProcessorType.
	// when ProcessorType selected as Dedicated, the default is set to 1.
	// when ProcessorType selected as Shared or Capped, the default is set to 0.5.
	// +optional
	Processors intstr.IntOrString `json:"processors,omitempty"`

	// memoryGiB is the size of a virtual machine's memory, in GiB.
	// maximum value for the MemoryGiB depends on the selected SystemType.
	// when SystemType is set to e880 maximum MemoryGiB value is 7463 GiB.
	// when SystemType is set to e980 maximum MemoryGiB value is 15307 GiB.
	// when SystemType is set to s922 maximum MemoryGiB value is 942 GiB.
	// The minimum memory is 32 GiB.
	// When omitted, this means the user has no opinion and the platform is left to choose a reasonable
	// default, which is subject to change over time. The current default is 32.
	// +optional
	MemoryGiB int32 `json:"memoryGiB,omitempty"`

	// loadBalancers is the set of load balancers to which the new control plane instance
	// should be added once it is created.
	// +optional
	LoadBalancers []LoadBalancerReference `json:"loadBalancers,omitempty"`
}

// PowerVSResource is a reference to a specific PowerVS resource by ID, Name or RegEx
// Only one of ID, Name or RegEx may be specified. Specifying more than one will result in
// a validation error.
// +union
type PowerVSResource struct {
	// type identifies the resource type for this entry.
	// Valid values are ID, Name and RegEx
	// +kubebuilder:validation:Enum:=ID;Name;RegEx
	// +optional
	Type PowerVSResourceType `json:"type,omitempty"`
	// id of resource
	// +optional
	ID *string `json:"id,omitempty"`
	// name of resource
	// +optional
	Name *string `json:"name,omitempty"`
	// regex to find resource
	// Regex contains the pattern to match to find a resource
	// +optional
	RegEx *string `json:"regex,omitempty"`
}

// PowerVSMachineProviderStatus is the type that will be embedded in a Machine.Status.ProviderStatus field.
// It contains PowerVS-specific status information.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PowerVSMachineProviderStatus struct {
	metav1.TypeMeta `json:",inline"`

	// conditions is a set of conditions associated with the Machine to indicate
	// errors or other status
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// instanceId is the instance ID of the machine created in PowerVS
	// instanceId uniquely identifies a Power VS server instance(VM) under a Power VS service.
	// This will help in updating or deleting a VM in Power VS Cloud
	// +optional
	InstanceID *string `json:"instanceId,omitempty"`

	// serviceInstanceID is the reference to the Power VS ServiceInstance on which the machine instance will be created.
	// serviceInstanceID uniquely identifies the Power VS service
	// By setting serviceInstanceID it will become easy and efficient to fetch a server instance(VM) within Power VS Cloud.
	// +optional
	ServiceInstanceID *string `json:"serviceInstanceID,omitempty"`

	// instanceState is the state of the PowerVS instance for this machine
	// Possible instance states are Active, Build, ShutOff, Reboot
	// This is used to display additional information to user regarding instance current state
	// +optional
	InstanceState *string `json:"instanceState,omitempty"`
}

// PowerVSSecretReference contains enough information to locate the
// referenced secret inside the same namespace.
// +structType=atomic
type PowerVSSecretReference struct {
	// name of the secret.
	// +optional
	Name string `json:"name,omitempty"`
}

// LoadBalancerReference is a reference to a load balancer on IBM Cloud virtual private cloud(VPC).
type LoadBalancerReference struct {
	// name of the LoadBalancer in IBM Cloud VPC.
	// The name should be between 1 and 63 characters long and may consist of lowercase alphanumeric characters and hyphens only.
	// The value must not end with a hyphen.
	// It is a reference to existing LoadBalancer created by openshift installer component.
	// +required
	// +kubebuilder:validation:Pattern=`^([a-z]|[a-z][-a-z0-9]*[a-z0-9]|[0-9][-a-z0-9]*([a-z]|[-a-z][-a-z0-9]*[a-z0-9]))$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`
	// type of the LoadBalancer service supported by IBM Cloud VPC.
	// Currently, only Application LoadBalancer is supported.
	// More details about Application LoadBalancer
	// https://cloud.ibm.com/docs/vpc?topic=vpc-load-balancers-about&interface=ui
	// Supported values are Application.
	// +required
	// +kubebuilder:validation:Enum:="Application"
	Type IBMVPCLoadBalancerType `json:"type"`
}
