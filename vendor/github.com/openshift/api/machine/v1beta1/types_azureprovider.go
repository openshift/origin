package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AzureMachineProviderSpec is the type that will be embedded in a Machine.Spec.ProviderSpec field
// for an Azure virtual machine. It is used by the Azure machine actuator to create a single Machine.
// Required parameters such as location that are not specified by this configuration, will be defaulted
// by the actuator.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AzureMachineProviderSpec struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// UserDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance
	// +optional
	UserDataSecret *corev1.SecretReference `json:"userDataSecret,omitempty"`
	// CredentialsSecret is a reference to the secret with Azure credentials.
	// +optional
	CredentialsSecret *corev1.SecretReference `json:"credentialsSecret,omitempty"`
	// Location is the region to use to create the instance
	// +optional
	Location string `json:"location,omitempty"`
	// VMSize is the size of the VM to create.
	// +optional
	VMSize string `json:"vmSize,omitempty"`
	// Image is the OS image to use to create the instance.
	Image Image `json:"image"`
	// OSDisk represents the parameters for creating the OS disk.
	OSDisk OSDisk `json:"osDisk"`
	// SSHPublicKey is the public key to use to SSH to the virtual machine.
	// +optional
	SSHPublicKey string `json:"sshPublicKey,omitempty"`
	// PublicIP if true a public IP will be used
	PublicIP bool `json:"publicIP"`
	// Tags is a list of tags to apply to the machine.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// Network Security Group that needs to be attached to the machine's interface.
	// No security group will be attached if empty.
	// +optional
	SecurityGroup string `json:"securityGroup,omitempty"`
	// Application Security Groups that need to be attached to the machine's interface.
	// No application security groups will be attached if zero-length.
	// +optional
	ApplicationSecurityGroups []string `json:"applicationSecurityGroups,omitempty"`
	// Subnet to use for this instance
	Subnet string `json:"subnet"`
	// PublicLoadBalancer to use for this instance
	// +optional
	PublicLoadBalancer string `json:"publicLoadBalancer,omitempty"`
	// InternalLoadBalancerName to use for this instance
	// +optional
	InternalLoadBalancer string `json:"internalLoadBalancer,omitempty"`
	// NatRule to set inbound NAT rule of the load balancer
	// +optional
	NatRule *int64 `json:"natRule,omitempty"`
	// ManagedIdentity to set managed identity name
	// +optional
	ManagedIdentity string `json:"managedIdentity,omitempty"`
	// Vnet to set virtual network name
	// +optional
	Vnet string `json:"vnet,omitempty"`
	// Availability Zone for the virtual machine.
	// If nil, the virtual machine should be deployed to no zone
	// +optional
	Zone *string `json:"zone,omitempty"`
	// NetworkResourceGroup is the resource group for the virtual machine's network
	// +optional
	NetworkResourceGroup string `json:"networkResourceGroup,omitempty"`
	// ResourceGroup is the resource group for the virtual machine
	// +optional
	ResourceGroup string `json:"resourceGroup,omitempty"`
	// SpotVMOptions allows the ability to specify the Machine should use a Spot VM
	// +optional
	SpotVMOptions *SpotVMOptions `json:"spotVMOptions,omitempty"`
	// SecurityProfile specifies the Security profile settings for a virtual machine.
	// +optional
	SecurityProfile *SecurityProfile `json:"securityProfile,omitempty"`
	// AcceleratedNetworking enables or disables Azure accelerated networking feature.
	// Set to false by default. If true, then this will depend on whether the requested
	// VMSize is supported. If set to true with an unsupported VMSize, Azure will return an error.
	// +optional
	AcceleratedNetworking bool `json:"acceleratedNetworking,omitempty"`
	// AvailabilitySet specifies the availability set to use for this instance.
	// Availability set should be precreated, before using this field.
	// +optional
	AvailabilitySet string `json:"availabilitySet,omitempty"`
}

// SpotVMOptions defines the options relevant to running the Machine on Spot VMs
type SpotVMOptions struct {
	// MaxPrice defines the maximum price the user is willing to pay for Spot VM instances
	// +optional
	MaxPrice *resource.Quantity `json:"maxPrice,omitempty"`
}

// AzureMachineProviderStatus is the type that will be embedded in a Machine.Status.ProviderStatus field.
// It contains Azure-specific status information.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type AzureMachineProviderStatus struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// VMID is the ID of the virtual machine created in Azure.
	// +optional
	VMID *string `json:"vmId,omitempty"`
	// VMState is the provisioning state of the Azure virtual machine.
	// +optional
	VMState *AzureVMState `json:"vmState,omitempty"`
	// Conditions is a set of conditions associated with the Machine to indicate
	// errors or other status.
	// +optional
	Conditions []AzureMachineProviderCondition `json:"conditions,omitempty"`
}

// VMState describes the state of an Azure virtual machine.
type AzureVMState string

const (
	// ProvisioningState related values
	// VMStateCreating ...
	VMStateCreating = AzureVMState("Creating")
	// VMStateDeleting ...
	VMStateDeleting = AzureVMState("Deleting")
	// VMStateFailed ...
	VMStateFailed = AzureVMState("Failed")
	// VMStateMigrating ...
	VMStateMigrating = AzureVMState("Migrating")
	// VMStateSucceeded ...
	VMStateSucceeded = AzureVMState("Succeeded")
	// VMStateUpdating ...
	VMStateUpdating = AzureVMState("Updating")

	// PowerState related values
	// VMStateStarting ...
	VMStateStarting = AzureVMState("Starting")
	// VMStateRunning ...
	VMStateRunning = AzureVMState("Running")
	// VMStateStopping ...
	VMStateStopping = AzureVMState("Stopping")
	// VMStateStopped ...
	VMStateStopped = AzureVMState("Stopped")
	// VMStateDeallocating ...
	VMStateDeallocating = AzureVMState("Deallocating")
	// VMStateDeallocated ...
	VMStateDeallocated = AzureVMState("Deallocated")
	// VMStateUnknown ...
	VMStateUnknown = AzureVMState("Unknown")
)

// Image is a mirror of azure sdk compute.ImageReference
type Image struct {
	// Publisher is the name of the organization that created the image
	Publisher string `json:"publisher"`
	// Offer specifies the name of a group of related images created by the publisher.
	// For example, UbuntuServer, WindowsServer
	Offer string `json:"offer"`
	// SKU specifies an instance of an offer, such as a major release of a distribution.
	// For example, 18.04-LTS, 2019-Datacenter
	SKU string `json:"sku"`
	// Version specifies the version of an image sku. The allowed formats
	// are Major.Minor.Build or 'latest'. Major, Minor, and Build are decimal numbers.
	// Specify 'latest' to use the latest version of an image available at deploy time.
	// Even if you use 'latest', the VM image will not automatically update after deploy
	// time even if a new version becomes available.
	Version string `json:"version"`
	// ResourceID specifies an image to use by ID
	ResourceID string `json:"resourceID"`
	// Type identifies the source of the image and related information, such as purchase plans.
	// Valid values are "ID", "MarketplaceWithPlan", "MarketplaceNoPlan", and omitted, which
	// means no opinion and the platform chooses a good default which may change over time.
	// Currently that default is "MarketplaceNoPlan" if publisher data is supplied, or "ID" if not.
	// For more information about purchase plans, see:
	// https://docs.microsoft.com/en-us/azure/virtual-machines/linux/cli-ps-findimage#check-the-purchase-plan-information
	// +optional
	Type AzureImageType `json:"type,omitempty"`
}

// AzureImageType provides an enumeration for the valid image types.
type AzureImageType string

const (
	// AzureImageTypeID specifies that the image should be referenced by its resource ID.
	AzureImageTypeID AzureImageType = "ID"
	// AzureImageTypeMarketplaceNoPlan are images available from the marketplace that do not require a purchase plan.
	AzureImageTypeMarketplaceNoPlan AzureImageType = "MarketplaceNoPlan"
	// AzureImageTypeMarketplaceWithPlan require a purchase plan. Upstream these images are referred to as "ThirdParty."
	AzureImageTypeMarketplaceWithPlan AzureImageType = "MarketplaceWithPlan"
)

type OSDisk struct {
	// OSType is the operating system type of the OS disk. Possible values include "Linux" and "Windows".
	OSType string `json:"osType"`
	// ManagedDisk specifies the Managed Disk parameters for the OS disk.
	ManagedDisk ManagedDiskParameters `json:"managedDisk"`
	// DiskSizeGB is the size in GB to assign to the data disk.
	DiskSizeGB int32 `json:"diskSizeGB"`
	// DiskSettings describe ephemeral disk settings for the os disk.
	// +optional
	DiskSettings DiskSettings `json:"diskSettings,omitempty"`
	// CachingType specifies the caching requirements.
	// Possible values include: 'None', 'ReadOnly', 'ReadWrite'.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over
	// time. Currently the default is `None`.
	// +optional
	// +kubebuilder:validation:Enum=None;ReadOnly;ReadWrite
	CachingType string `json:"cachingType,omitempty"`
}

// DiskSettings describe ephemeral disk settings for the os disk.
type DiskSettings struct {
	// EphemeralStorageLocation enables ephemeral OS when set to 'Local'.
	// Possible values include: 'Local'.
	// See https://docs.microsoft.com/en-us/azure/virtual-machines/ephemeral-os-disks for full details.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over
	// time. Currently the default is that disks are saved to remote Azure storage.
	// +optional
	// +kubebuilder:validation:Enum=Local
	EphemeralStorageLocation string `json:"ephemeralStorageLocation,omitempty"`
}

// ManagedDiskParameters is the parameters of a managed disk.
type ManagedDiskParameters struct {
	// StorageAccountType is the storage account type to use. Possible values include "Standard_LRS" and "Premium_LRS".
	StorageAccountType string `json:"storageAccountType"`
	// DiskEncryptionSet is the disk encryption set properties
	// +optional
	DiskEncryptionSet *DiskEncryptionSetParameters `json:"diskEncryptionSet,omitempty"`
}

// DiskEncryptionSetParameters is the disk encryption set properties
type DiskEncryptionSetParameters struct {
	// ID is the disk encryption set ID
	// +optional
	ID string `json:"id,omitempty"`
}

// SecurityProfile specifies the Security profile settings for a
// virtual machine or virtual machine scale set.
type SecurityProfile struct {
	// This field indicates whether Host Encryption should be enabled
	// or disabled for a virtual machine or virtual machine scale
	// set. Default is disabled.
	// +optional
	EncryptionAtHost *bool `json:"encryptionAtHost,omitempty"`
}

// AzureMachineProviderCondition is a condition in a AzureMachineProviderStatus
type AzureMachineProviderCondition struct {
	// Type is the type of the condition.
	Type ConditionType `json:"type"`
	// Status is the status of the condition.
	Status corev1.ConditionStatus `json:"status"`
	// LastProbeTime is the last time we probed the condition.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime"`
	// LastTransitionTime is the last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason string `json:"reason"`
	// Message is a human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message"`
}
