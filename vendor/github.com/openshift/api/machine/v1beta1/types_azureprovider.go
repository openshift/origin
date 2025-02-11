package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecurityEncryptionTypes represents the Encryption Type when the Azure Virtual Machine is a
// Confidential VM.
type SecurityEncryptionTypes string

const (
	// SecurityEncryptionTypesVMGuestStateOnly disables OS disk confidential encryption.
	SecurityEncryptionTypesVMGuestStateOnly SecurityEncryptionTypes = "VMGuestStateOnly"
	// SecurityEncryptionTypesDiskWithVMGuestState enables OS disk confidential encryption with a
	// platform-managed key (PMK) or a customer-managed key (CMK).
	SecurityEncryptionTypesDiskWithVMGuestState SecurityEncryptionTypes = "DiskWithVMGuestState"
)

// SecurityTypes represents the SecurityType of the virtual machine.
type SecurityTypes string

const (
	// SecurityTypesConfidentialVM defines the SecurityType of the virtual machine as a Confidential VM.
	SecurityTypesConfidentialVM SecurityTypes = "ConfidentialVM"
	// SecurityTypesTrustedLaunch defines the SecurityType of the virtual machine as a Trusted Launch VM.
	SecurityTypesTrustedLaunch SecurityTypes = "TrustedLaunch"
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
	// userDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance
	// +optional
	UserDataSecret *corev1.SecretReference `json:"userDataSecret,omitempty"`
	// credentialsSecret is a reference to the secret with Azure credentials.
	// +optional
	CredentialsSecret *corev1.SecretReference `json:"credentialsSecret,omitempty"`
	// location is the region to use to create the instance
	// +optional
	Location string `json:"location,omitempty"`
	// vmSize is the size of the VM to create.
	// +optional
	VMSize string `json:"vmSize,omitempty"`
	// image is the OS image to use to create the instance.
	Image Image `json:"image"`
	// osDisk represents the parameters for creating the OS disk.
	OSDisk OSDisk `json:"osDisk"`
	// DataDisk specifies the parameters that are used to add one or more data disks to the machine.
	// +optional
	DataDisks []DataDisk `json:"dataDisks,omitempty"`
	// sshPublicKey is the public key to use to SSH to the virtual machine.
	// +optional
	SSHPublicKey string `json:"sshPublicKey,omitempty"`
	// publicIP if true a public IP will be used
	PublicIP bool `json:"publicIP"`
	// tags is a list of tags to apply to the machine.
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
	// subnet to use for this instance
	Subnet string `json:"subnet"`
	// publicLoadBalancer to use for this instance
	// +optional
	PublicLoadBalancer string `json:"publicLoadBalancer,omitempty"`
	// InternalLoadBalancerName to use for this instance
	// +optional
	InternalLoadBalancer string `json:"internalLoadBalancer,omitempty"`
	// natRule to set inbound NAT rule of the load balancer
	// +optional
	NatRule *int64 `json:"natRule,omitempty"`
	// managedIdentity to set managed identity name
	// +optional
	ManagedIdentity string `json:"managedIdentity,omitempty"`
	// vnet to set virtual network name
	// +optional
	Vnet string `json:"vnet,omitempty"`
	// Availability Zone for the virtual machine.
	// If nil, the virtual machine should be deployed to no zone
	// +optional
	Zone string `json:"zone,omitempty"`
	// networkResourceGroup is the resource group for the virtual machine's network
	// +optional
	NetworkResourceGroup string `json:"networkResourceGroup,omitempty"`
	// resourceGroup is the resource group for the virtual machine
	// +optional
	ResourceGroup string `json:"resourceGroup,omitempty"`
	// spotVMOptions allows the ability to specify the Machine should use a Spot VM
	// +optional
	SpotVMOptions *SpotVMOptions `json:"spotVMOptions,omitempty"`
	// securityProfile specifies the Security profile settings for a virtual machine.
	// +optional
	SecurityProfile *SecurityProfile `json:"securityProfile,omitempty"`
	// ultraSSDCapability enables or disables Azure UltraSSD capability for a virtual machine.
	// This can be used to allow/disallow binding of Azure UltraSSD to the Machine both as Data Disks or via Persistent Volumes.
	// This Azure feature is subject to a specific scope and certain limitations.
	// More informations on this can be found in the official Azure documentation for Ultra Disks:
	// (https://docs.microsoft.com/en-us/azure/virtual-machines/disks-enable-ultra-ssd?tabs=azure-portal#ga-scope-and-limitations).
	//
	// When omitted, if at least one Data Disk of type UltraSSD is specified, the platform will automatically enable the capability.
	// If a Perisistent Volume backed by an UltraSSD is bound to a Pod on the Machine, when this field is ommitted, the platform will *not* automatically enable the capability (unless already enabled by the presence of an UltraSSD as Data Disk).
	// This may manifest in the Pod being stuck in `ContainerCreating` phase.
	// This defaulting behaviour may be subject to change in future.
	//
	// When set to "Enabled", if the capability is available for the Machine based on the scope and limitations described above, the capability will be set on the Machine.
	// This will thus allow UltraSSD both as Data Disks and Persistent Volumes.
	// If set to "Enabled" when the capability can't be available due to scope and limitations, the Machine will go into "Failed" state.
	//
	// When set to "Disabled", UltraSSDs will not be allowed either as Data Disks nor as Persistent Volumes.
	// In this case if any UltraSSDs are specified as Data Disks on a Machine, the Machine will go into a "Failed" state.
	// If instead any UltraSSDs are backing the volumes (via Persistent Volumes) of any Pods scheduled on a Node which is backed by the Machine, the Pod may get stuck in `ContainerCreating` phase.
	//
	// +kubebuilder:validation:Enum:="Enabled";"Disabled"
	// +optional
	UltraSSDCapability AzureUltraSSDCapabilityState `json:"ultraSSDCapability,omitempty"`
	// acceleratedNetworking enables or disables Azure accelerated networking feature.
	// Set to false by default. If true, then this will depend on whether the requested
	// VMSize is supported. If set to true with an unsupported VMSize, Azure will return an error.
	// +optional
	AcceleratedNetworking bool `json:"acceleratedNetworking,omitempty"`
	// availabilitySet specifies the availability set to use for this instance.
	// Availability set should be precreated, before using this field.
	// +optional
	AvailabilitySet string `json:"availabilitySet,omitempty"`
	// diagnostics configures the diagnostics settings for the virtual machine.
	// This allows you to configure boot diagnostics such as capturing serial output from
	// the virtual machine on boot.
	// This is useful for debugging software based launch issues.
	// +optional
	Diagnostics AzureDiagnostics `json:"diagnostics,omitempty"`
	// capacityReservationGroupID specifies the capacity reservation group resource id that should be
	// used for allocating the virtual machine.
	// The field size should be greater than 0 and the field input must start with '/'.
	// The input for capacityReservationGroupID must be similar to '/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/capacityReservationGroups/{capacityReservationGroupName}'.
	// The keys which are used should be among 'subscriptions', 'providers' and 'resourcegroups' followed by valid ID or names respectively.
	// +optional
	CapacityReservationGroupID string `json:"capacityReservationGroupID,omitempty"`
}

// SpotVMOptions defines the options relevant to running the Machine on Spot VMs
type SpotVMOptions struct {
	// maxPrice defines the maximum price the user is willing to pay for Spot VM instances
	// +optional
	MaxPrice *resource.Quantity `json:"maxPrice,omitempty"`
}

// AzureDiagnostics is used to configure the diagnostic settings of the virtual machine.
type AzureDiagnostics struct {
	// AzureBootDiagnostics configures the boot diagnostics settings for the virtual machine.
	// This allows you to configure capturing serial output from the virtual machine on boot.
	// This is useful for debugging software based launch issues.
	// + This is a pointer so that we can validate required fields only when the structure is
	// + configured by the user.
	// +optional
	Boot *AzureBootDiagnostics `json:"boot,omitempty"`
}

// AzureBootDiagnostics configures the boot diagnostics settings for the virtual machine.
// This allows you to configure capturing serial output from the virtual machine on boot.
// This is useful for debugging software based launch issues.
// +union
type AzureBootDiagnostics struct {
	// storageAccountType determines if the storage account for storing the diagnostics data
	// should be provisioned by Azure (AzureManaged) or by the customer (CustomerManaged).
	// +required
	// +unionDiscriminator
	StorageAccountType AzureBootDiagnosticsStorageAccountType `json:"storageAccountType"`

	// customerManaged provides reference to the customer manager storage account.
	// +optional
	CustomerManaged *AzureCustomerManagedBootDiagnostics `json:"customerManaged,omitempty"`
}

// AzureCustomerManagedBootDiagnostics provides reference to a customer managed
// storage account.
type AzureCustomerManagedBootDiagnostics struct {
	// storageAccountURI is the URI of the customer managed storage account.
	// The URI typically will be `https://<mystorageaccountname>.blob.core.windows.net/`
	// but may differ if you are using Azure DNS zone endpoints.
	// You can find the correct endpoint by looking for the Blob Primary Endpoint in the
	// endpoints tab in the Azure console.
	// +required
	// +kubebuilder:validation:Pattern=`^https://`
	// +kubebuilder:validation:MaxLength=1024
	StorageAccountURI string `json:"storageAccountURI"`
}

// AzureBootDiagnosticsStorageAccountType defines the list of valid storage account types
// for the boot diagnostics.
// +kubebuilder:validation:Enum:="AzureManaged";"CustomerManaged"
type AzureBootDiagnosticsStorageAccountType string

const (
	// AzureManagedAzureDiagnosticsStorage is used to determine that the diagnostics storage account
	// should be provisioned by Azure.
	AzureManagedAzureDiagnosticsStorage AzureBootDiagnosticsStorageAccountType = "AzureManaged"

	// CustomerManagedAzureDiagnosticsStorage is used to determine that the diagnostics storage account
	// should be provisioned by the Customer.
	CustomerManagedAzureDiagnosticsStorage AzureBootDiagnosticsStorageAccountType = "CustomerManaged"
)

// AzureMachineProviderStatus is the type that will be embedded in a Machine.Status.ProviderStatus field.
// It contains Azure-specific status information.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type AzureMachineProviderStatus struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// vmId is the ID of the virtual machine created in Azure.
	// +optional
	VMID *string `json:"vmId,omitempty"`
	// vmState is the provisioning state of the Azure virtual machine.
	// +optional
	VMState *AzureVMState `json:"vmState,omitempty"`
	// conditions is a set of conditions associated with the Machine to indicate
	// errors or other status.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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
	// publisher is the name of the organization that created the image
	Publisher string `json:"publisher"`
	// offer specifies the name of a group of related images created by the publisher.
	// For example, UbuntuServer, WindowsServer
	Offer string `json:"offer"`
	// sku specifies an instance of an offer, such as a major release of a distribution.
	// For example, 18.04-LTS, 2019-Datacenter
	SKU string `json:"sku"`
	// version specifies the version of an image sku. The allowed formats
	// are Major.Minor.Build or 'latest'. Major, Minor, and Build are decimal numbers.
	// Specify 'latest' to use the latest version of an image available at deploy time.
	// Even if you use 'latest', the VM image will not automatically update after deploy
	// time even if a new version becomes available.
	Version string `json:"version"`
	// resourceID specifies an image to use by ID
	ResourceID string `json:"resourceID"`
	// type identifies the source of the image and related information, such as purchase plans.
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
	// osType is the operating system type of the OS disk. Possible values include "Linux" and "Windows".
	OSType string `json:"osType"`
	// managedDisk specifies the Managed Disk parameters for the OS disk.
	ManagedDisk OSDiskManagedDiskParameters `json:"managedDisk"`
	// diskSizeGB is the size in GB to assign to the data disk.
	DiskSizeGB int32 `json:"diskSizeGB"`
	// diskSettings describe ephemeral disk settings for the os disk.
	// +optional
	DiskSettings DiskSettings `json:"diskSettings,omitempty"`
	// cachingType specifies the caching requirements.
	// Possible values include: 'None', 'ReadOnly', 'ReadWrite'.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over
	// time. Currently the default is `None`.
	// +optional
	// +kubebuilder:validation:Enum=None;ReadOnly;ReadWrite
	CachingType string `json:"cachingType,omitempty"`
}

// DataDisk specifies the parameters that are used to add one or more data disks to the machine.
// A Data Disk is a managed disk that's attached to a virtual machine to store application data.
// It differs from an OS Disk as it doesn't come with a pre-installed OS, and it cannot contain the boot volume.
// It is registered as SCSI drive and labeled with the chosen `lun`. e.g. for `lun: 0` the raw disk device will be available at `/dev/disk/azure/scsi1/lun0`.
//
// As the Data Disk disk device is attached raw to the virtual machine, it will need to be partitioned, formatted with a filesystem and mounted, in order for it to be usable.
// This can be done by creating a custom userdata Secret with custom Ignition configuration to achieve the desired initialization.
// At this stage the previously defined `lun` is to be used as the "device" key for referencing the raw disk device to be initialized.
// Once the custom userdata Secret has been created, it can be referenced in the Machine's `.providerSpec.userDataSecret`.
// For further guidance and examples, please refer to the official OpenShift docs.
type DataDisk struct {
	// nameSuffix is the suffix to be appended to the machine name to generate the disk name.
	// Each disk name will be in format <machineName>_<nameSuffix>.
	// NameSuffix name must start and finish with an alphanumeric character and can only contain letters, numbers, underscores, periods or hyphens.
	// The overall disk name must not exceed 80 chars in length.
	// +kubebuilder:validation:Pattern:=`^[a-zA-Z0-9](?:[\w\.-]*[a-zA-Z0-9])?$`
	// +kubebuilder:validation:MaxLength:=78
	// +required
	NameSuffix string `json:"nameSuffix"`
	// diskSizeGB is the size in GB to assign to the data disk.
	// +kubebuilder:validation:Minimum=4
	// +required
	DiskSizeGB int32 `json:"diskSizeGB"`
	// managedDisk specifies the Managed Disk parameters for the data disk.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over time.
	// Currently the default is a ManagedDisk with with storageAccountType: "Premium_LRS" and diskEncryptionSet.id: "Default".
	// +optional
	ManagedDisk DataDiskManagedDiskParameters `json:"managedDisk,omitempty"`
	// lun Specifies the logical unit number of the data disk.
	// This value is used to identify data disks within the VM and therefore must be unique for each data disk attached to a VM.
	// This value is also needed for referencing the data disks devices within userdata to perform disk initialization through Ignition (e.g. partition/format/mount).
	// The value must be between 0 and 63.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=63
	// +required
	Lun int32 `json:"lun,omitempty"`
	// cachingType specifies the caching requirements.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over time.
	// Currently the default is CachingTypeNone.
	// +optional
	// +kubebuilder:validation:Enum=None;ReadOnly;ReadWrite
	CachingType CachingTypeOption `json:"cachingType,omitempty"`
	// deletionPolicy specifies the data disk deletion policy upon Machine deletion.
	// Possible values are "Delete","Detach".
	// When "Delete" is used the data disk is deleted when the Machine is deleted.
	// When "Detach" is used the data disk is detached from the Machine and retained when the Machine is deleted.
	// +kubebuilder:validation:Enum=Delete;Detach
	// +required
	DeletionPolicy DiskDeletionPolicyType `json:"deletionPolicy"`
}

// DiskDeletionPolicyType defines the possible values for DeletionPolicy.
type DiskDeletionPolicyType string

// These are the valid DiskDeletionPolicyType values.
const (
	// DiskDeletionPolicyTypeDelete means the DiskDeletionPolicyType is "Delete".
	DiskDeletionPolicyTypeDelete DiskDeletionPolicyType = "Delete"
	// DiskDeletionPolicyTypeDetach means the DiskDeletionPolicyType is "Detach".
	DiskDeletionPolicyTypeDetach DiskDeletionPolicyType = "Detach"
)

// CachingTypeOption defines the different values for a CachingType.
type CachingTypeOption string

// These are the valid CachingTypeOption values.
const (
	// CachingTypeReadOnly means the CachingType is "ReadOnly".
	CachingTypeReadOnly CachingTypeOption = "ReadOnly"
	// CachingTypeReadWrite means the CachingType is "ReadWrite".
	CachingTypeReadWrite CachingTypeOption = "ReadWrite"
	// CachingTypeNone means the CachingType is "None".
	CachingTypeNone CachingTypeOption = "None"
)

// DiskSettings describe ephemeral disk settings for the os disk.
type DiskSettings struct {
	// ephemeralStorageLocation enables ephemeral OS when set to 'Local'.
	// Possible values include: 'Local'.
	// See https://docs.microsoft.com/en-us/azure/virtual-machines/ephemeral-os-disks for full details.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over
	// time. Currently the default is that disks are saved to remote Azure storage.
	// +optional
	// +kubebuilder:validation:Enum=Local
	EphemeralStorageLocation string `json:"ephemeralStorageLocation,omitempty"`
}

// OSDiskManagedDiskParameters is the parameters of a OSDisk managed disk.
type OSDiskManagedDiskParameters struct {
	// storageAccountType is the storage account type to use.
	// Possible values include "Standard_LRS", "Premium_LRS".
	StorageAccountType string `json:"storageAccountType"`
	// diskEncryptionSet is the disk encryption set properties
	// +optional
	DiskEncryptionSet *DiskEncryptionSetParameters `json:"diskEncryptionSet,omitempty"`
	// securityProfile specifies the security profile for the managed disk.
	// +optional
	SecurityProfile VMDiskSecurityProfile `json:"securityProfile,omitempty"`
}

// VMDiskSecurityProfile specifies the security profile settings for the managed disk.
// It can be set only for Confidential VMs.
type VMDiskSecurityProfile struct {
	// diskEncryptionSet specifies the customer managed disk encryption set resource id for the
	// managed disk that is used for Customer Managed Key encrypted ConfidentialVM OS Disk and
	// VMGuest blob.
	// +optional
	DiskEncryptionSet DiskEncryptionSetParameters `json:"diskEncryptionSet,omitempty"`
	// securityEncryptionType specifies the encryption type of the managed disk.
	// It is set to DiskWithVMGuestState to encrypt the managed disk along with the VMGuestState
	// blob, and to VMGuestStateOnly to encrypt the VMGuestState blob only.
	// When set to VMGuestStateOnly, the vTPM should be enabled.
	// When set to DiskWithVMGuestState, both SecureBoot and vTPM should be enabled.
	// If the above conditions are not fulfilled, the VM will not be created and the respective error
	// will be returned.
	// It can be set only for Confidential VMs. Confidential VMs are defined by their
	// SecurityProfile.SecurityType being set to ConfidentialVM, the SecurityEncryptionType of their
	// OS disk being set to one of the allowed values and by enabling the respective
	// SecurityProfile.UEFISettings of the VM (i.e. vTPM and SecureBoot), depending on the selected
	// SecurityEncryptionType.
	// For further details on Azure Confidential VMs, please refer to the respective documentation:
	// https://learn.microsoft.com/azure/confidential-computing/confidential-vm-overview
	// +kubebuilder:validation:Enum=VMGuestStateOnly;DiskWithVMGuestState
	// +optional
	SecurityEncryptionType SecurityEncryptionTypes `json:"securityEncryptionType,omitempty"`
}

// DataDiskManagedDiskParameters is the parameters of a DataDisk managed disk.
type DataDiskManagedDiskParameters struct {
	// storageAccountType is the storage account type to use.
	// Possible values include "Standard_LRS", "Premium_LRS" and "UltraSSD_LRS".
	// +kubebuilder:validation:Enum=Standard_LRS;Premium_LRS;UltraSSD_LRS
	StorageAccountType StorageAccountType `json:"storageAccountType"`
	// diskEncryptionSet is the disk encryption set properties.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over time.
	// Currently the default is a DiskEncryptionSet with id: "Default".
	// +optional
	DiskEncryptionSet *DiskEncryptionSetParameters `json:"diskEncryptionSet,omitempty"`
}

// StorageAccountType defines the different storage types to use for a ManagedDisk.
type StorageAccountType string

// These are the valid StorageAccountType types.
const (
	// "StorageAccountStandardLRS" means the Standard_LRS storage type.
	StorageAccountStandardLRS StorageAccountType = "Standard_LRS"
	// "StorageAccountPremiumLRS" means the Premium_LRS storage type.
	StorageAccountPremiumLRS StorageAccountType = "Premium_LRS"
	// "StorageAccountUltraSSDLRS" means the UltraSSD_LRS storage type.
	StorageAccountUltraSSDLRS StorageAccountType = "UltraSSD_LRS"
)

// DiskEncryptionSetParameters is the disk encryption set properties
type DiskEncryptionSetParameters struct {
	// id is the disk encryption set ID
	// Empty value means no opinion and the platform chooses a default, which is subject to change over time.
	// Currently the default is: "Default".
	// +optional
	ID string `json:"id,omitempty"`
}

// SecurityProfile specifies the Security profile settings for a
// virtual machine or virtual machine scale set.
type SecurityProfile struct {
	// encryptionAtHost indicates whether Host Encryption should be enabled or disabled for a virtual
	// machine or virtual machine scale set.
	// This should be disabled when SecurityEncryptionType is set to DiskWithVMGuestState.
	// Default is disabled.
	// +optional
	EncryptionAtHost *bool `json:"encryptionAtHost,omitempty"`
	// settings specify the security type and the UEFI settings of the virtual machine. This field can
	// be set for Confidential VMs and Trusted Launch for VMs.
	// +optional
	Settings SecuritySettings `json:"settings,omitempty"`
}

// SecuritySettings define the security type and the UEFI settings of the virtual machine.
// +union
type SecuritySettings struct {
	// securityType specifies the SecurityType of the virtual machine. It has to be set to any specified value to
	// enable UEFISettings. The default behavior is: UEFISettings will not be enabled unless this property is set.
	// +kubebuilder:validation:Enum=ConfidentialVM;TrustedLaunch
	// +required
	// +unionDiscriminator
	SecurityType SecurityTypes `json:"securityType,omitempty"`
	// confidentialVM specifies the security configuration of the virtual machine.
	// For more information regarding Confidential VMs, please refer to:
	// https://learn.microsoft.com/azure/confidential-computing/confidential-vm-overview
	// +optional
	ConfidentialVM *ConfidentialVM `json:"confidentialVM,omitempty"`
	// trustedLaunch specifies the security configuration of the virtual machine.
	// For more information regarding TrustedLaunch for VMs, please refer to:
	// https://learn.microsoft.com/azure/virtual-machines/trusted-launch
	// +optional
	TrustedLaunch *TrustedLaunch `json:"trustedLaunch,omitempty"`
}

// ConfidentialVM defines the UEFI settings for the virtual machine.
type ConfidentialVM struct {
	// uefiSettings specifies the security settings like secure boot and vTPM used while creating the virtual machine.
	// +required
	UEFISettings UEFISettings `json:"uefiSettings,omitempty"`
}

// TrustedLaunch defines the UEFI settings for the virtual machine.
type TrustedLaunch struct {
	// uefiSettings specifies the security settings like secure boot and vTPM used while creating the virtual machine.
	// +required
	UEFISettings UEFISettings `json:"uefiSettings,omitempty"`
}

// UEFISettings specifies the security settings like secure boot and vTPM used while creating the
// virtual machine.
type UEFISettings struct {
	// secureBoot specifies whether secure boot should be enabled on the virtual machine.
	// Secure Boot verifies the digital signature of all boot components and halts the boot process if
	// signature verification fails.
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is disabled.
	// +kubebuilder:validation:Enum=Enabled;Disabled
	// +optional
	SecureBoot SecureBootPolicy `json:"secureBoot,omitempty"`
	// virtualizedTrustedPlatformModule specifies whether vTPM should be enabled on the virtual machine.
	// When enabled the virtualized trusted platform module measurements are used to create a known good boot integrity policy baseline.
	// The integrity policy baseline is used for comparison with measurements from subsequent VM boots to determine if anything has changed.
	// This is required to be enabled if SecurityEncryptionType is defined.
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is disabled.
	// +kubebuilder:validation:Enum=Enabled;Disabled
	// +optional
	VirtualizedTrustedPlatformModule VirtualizedTrustedPlatformModulePolicy `json:"virtualizedTrustedPlatformModule,omitempty"`
}

// AzureUltraSSDCapabilityState defines the different states of an UltraSSDCapability
type AzureUltraSSDCapabilityState string

// These are the valid AzureUltraSSDCapabilityState states.
const (
	// "AzureUltraSSDCapabilityEnabled" means the Azure UltraSSDCapability is Enabled
	AzureUltraSSDCapabilityEnabled AzureUltraSSDCapabilityState = "Enabled"
	// "AzureUltraSSDCapabilityDisabled" means the Azure UltraSSDCapability is Disabled
	AzureUltraSSDCapabilityDisabled AzureUltraSSDCapabilityState = "Disabled"
)
