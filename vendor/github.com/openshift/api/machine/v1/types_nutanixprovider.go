package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NutanixMachineProviderConfig is the Schema for the nutanixmachineproviderconfigs API
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +k8s:openapi-gen=true
type NutanixMachineProviderConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// cluster is to identify the cluster (the Prism Element under management
	// of the Prism Central), in which the Machine's VM will be created.
	// The cluster identifier (uuid or name) can be obtained from the Prism Central console
	// or using the prism_central API.
	// +kubebuilder:validation:Required
	Cluster NutanixResourceIdentifier `json:"cluster"`

	// image is to identify the rhcos image uploaded to the Prism Central (PC)
	// The image identifier (uuid or name) can be obtained from the Prism Central console
	// or using the prism_central API.
	// +kubebuilder:validation:Required
	Image NutanixResourceIdentifier `json:"image"`

	// subnets holds a list of identifiers (one or more) of the cluster's network subnets
	// for the Machine's VM to connect to. The subnet identifiers (uuid or name) can be
	// obtained from the Prism Central console or using the prism_central API.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Subnets []NutanixResourceIdentifier `json:"subnets"`

	// vcpusPerSocket is the number of vCPUs per socket of the VM
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	VCPUsPerSocket int32 `json:"vcpusPerSocket"`

	// vcpuSockets is the number of vCPU sockets of the VM
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	VCPUSockets int32 `json:"vcpuSockets"`

	// memorySize is the memory size (in Quantity format) of the VM
	// The minimum memorySize is 2Gi bytes
	// +kubebuilder:validation:Required
	MemorySize resource.Quantity `json:"memorySize"`

	// systemDiskSize is size (in Quantity format) of the system disk of the VM
	// The minimum systemDiskSize is 20Gi bytes
	// +kubebuilder:validation:Required
	SystemDiskSize resource.Quantity `json:"systemDiskSize"`

	// bootType indicates the boot type (Legacy, UEFI or SecureBoot) the Machine's VM uses to boot.
	// If this field is empty or omitted, the VM will use the default boot type "Legacy" to boot.
	// "SecureBoot" depends on "UEFI" boot, i.e., enabling "SecureBoot" means that "UEFI" boot is also enabled.
	// +kubebuilder:validation:Enum="";Legacy;UEFI;SecureBoot
	// +optional
	BootType NutanixBootType `json:"bootType"`

	// project optionally identifies a Prism project for the Machine's VM to associate with.
	// +optional
	Project NutanixResourceIdentifier `json:"project"`

	// categories optionally adds one or more prism categories (each with key and value) for
	// the Machine's VM to associate with. All the category key and value pairs specified must
	// already exist in the prism central.
	// +listType=map
	// +listMapKey=key
	// +optional
	Categories []NutanixCategory `json:"categories"`

	// userDataSecret is a local reference to a secret that contains the
	// UserData to apply to the VM
	UserDataSecret *corev1.LocalObjectReference `json:"userDataSecret,omitempty"`

	// credentialsSecret is a local reference to a secret that contains the
	// credentials data to access Nutanix PC client
	// +kubebuilder:validation:Required
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret"`

	// failureDomain refers to the name of the FailureDomain with which this Machine is associated.
	// If this is configured, the Nutanix machine controller will use the prism_central endpoint
	// and credentials defined in the referenced FailureDomain to communicate to the prism_central.
	// It will also verify that the 'cluster' and subnets' configuration in the NutanixMachineProviderConfig
	// is consistent with that in the referenced failureDomain.
	// +optional
	FailureDomain *NutanixFailureDomainReference `json:"failureDomain"`
}

// NutanixCategory identifies a pair of prism category key and value
type NutanixCategory struct {
	// key is the prism category key name
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// value is the prism category value associated with the key
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}

// NutanixBootType is an enumeration of different boot types for Nutanix VM.
type NutanixBootType string

const (
	// NutanixLegacyBoot is the legacy BIOS boot type
	NutanixLegacyBoot NutanixBootType = "Legacy"

	// NutanixUEFIBoot is the UEFI boot type
	NutanixUEFIBoot NutanixBootType = "UEFI"

	// NutanixSecureBoot is the Secure boot type
	NutanixSecureBoot NutanixBootType = "SecureBoot"
)

// NutanixIdentifierType is an enumeration of different resource identifier types.
type NutanixIdentifierType string

const (
	// NutanixIdentifierUUID is a resource identifier identifying the object by UUID.
	NutanixIdentifierUUID NutanixIdentifierType = "uuid"

	// NutanixIdentifierName is a resource identifier identifying the object by Name.
	NutanixIdentifierName NutanixIdentifierType = "name"
)

// NutanixResourceIdentifier holds the identity of a Nutanix PC resource (cluster, image, subnet, etc.)
// +union
type NutanixResourceIdentifier struct {
	// Type is the identifier type to use for this resource.
	// +unionDiscriminator
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum:=uuid;name
	Type NutanixIdentifierType `json:"type"`

	// uuid is the UUID of the resource in the PC.
	// +optional
	UUID *string `json:"uuid,omitempty"`

	// name is the resource name in the PC
	// +optional
	Name *string `json:"name,omitempty"`
}

// NutanixMachineProviderStatus is the type that will be embedded in a Machine.Status.ProviderStatus field.
// It contains nutanix-specific status information.
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NutanixMachineProviderStatus struct {
	metav1.TypeMeta `json:",inline"`

	// conditions is a set of conditions associated with the Machine to indicate
	// errors or other status
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// vmUUID is the Machine associated VM's UUID
	// The field is missing before the VM is created.
	// Once the VM is created, the field is filled with the VM's UUID and it will not change.
	// The vmUUID is used to find the VM when updating the Machine status,
	// and to delete the VM when the Machine is deleted.
	// +optional
	VmUUID *string `json:"vmUUID,omitempty"`
}
