package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VSphereMachineProviderSpec is the type that will be embedded in a Machine.Spec.ProviderSpec field
// for an VSphere virtual machine. It is used by the vSphere machine actuator to create a single Machine.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VSphereMachineProviderSpec struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// userDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance
	// +optional
	UserDataSecret *corev1.LocalObjectReference `json:"userDataSecret,omitempty"`
	// credentialsSecret is a reference to the secret with vSphere credentials.
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
	// template is the name, inventory path, or instance UUID of the template
	// used to clone new machines.
	Template string `json:"template"`
	// workspace describes the workspace to use for the machine.
	// +optional
	Workspace *Workspace `json:"workspace,omitempty"`
	// network is the network configuration for this machine's VM.
	Network NetworkSpec `json:"network"`
	// numCPUs is the number of virtual processors in a virtual machine.
	// Defaults to the analogue property value in the template from which this
	// machine is cloned.
	// +optional
	NumCPUs int32 `json:"numCPUs,omitempty"`
	// NumCPUs is the number of cores among which to distribute CPUs in this
	// virtual machine.
	// Defaults to the analogue property value in the template from which this
	// machine is cloned.
	// +optional
	NumCoresPerSocket int32 `json:"numCoresPerSocket,omitempty"`
	// memoryMiB is the size of a virtual machine's memory, in MiB.
	// Defaults to the analogue property value in the template from which this
	// machine is cloned.
	// +optional
	MemoryMiB int64 `json:"memoryMiB,omitempty"`
	// diskGiB is the size of a virtual machine's disk, in GiB.
	// Defaults to the analogue property value in the template from which this
	// machine is cloned.
	// This parameter will be ignored if 'LinkedClone' CloneMode is set.
	// +optional
	DiskGiB int32 `json:"diskGiB,omitempty"`
	// tagIDs is an optional set of tags to add to an instance. Specified tagIDs
	// must use URN-notation instead of display names. A maximum of 10 tag IDs may be specified.
	// +kubebuilder:validation:Pattern="^(urn):(vmomi):(InventoryServiceTag):([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}):([^:]+)$"
	// +kubebuilder:example="urn:vmomi:InventoryServiceTag:5736bf56-49f5-4667-b38c-b97e09dc9578:GLOBAL"
	// +optional
	TagIDs []string `json:"tagIDs,omitempty"`
	// snapshot is the name of the snapshot from which the VM was cloned
	// +optional
	Snapshot string `json:"snapshot"`
	// cloneMode specifies the type of clone operation.
	// The LinkedClone mode is only support for templates that have at least
	// one snapshot. If the template has no snapshots, then CloneMode defaults
	// to FullClone.
	// When LinkedClone mode is enabled the DiskGiB field is ignored as it is
	// not possible to expand disks of linked clones.
	// Defaults to FullClone.
	// When using LinkedClone, if no snapshots exist for the source template, falls back to FullClone.
	// +optional
	CloneMode CloneMode `json:"cloneMode,omitempty"`
	// dataDisks is a list of non OS disks to be created and attached to the VM.  The max number of disk allowed to be attached is
	// currently 29.  The max number of disks for any controller is 30, but VM template will always have OS disk so that will leave
	// 29 disks on any controller type.
	// +openshift:enable:FeatureGate=VSphereMultiDisk
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=29
	DataDisks []VSphereDisk `json:"dataDisks,omitempty"`
}

// CloneMode is the type of clone operation used to clone a VM from a template.
type CloneMode string

const (
	// FullClone indicates a VM will have no relationship to the source of the
	// clone operation once the operation is complete. This is the safest clone
	// mode, but it is not the fastest.
	FullClone CloneMode = "fullClone"
	// LinkedClone means resulting VMs will be dependent upon the snapshot of
	// the source VM/template from which the VM was cloned. This is the fastest
	// clone mode, but it also prevents expanding a VMs disk beyond the size of
	// the source VM/template.
	LinkedClone CloneMode = "linkedClone"
)

// NetworkSpec defines the virtual machine's network configuration.
type NetworkSpec struct {
	// devices defines the virtual machine's network interfaces.
	Devices []NetworkDeviceSpec `json:"devices"`
}

// AddressesFromPool is an IPAddressPool that will be used to create
// IPAddressClaims for fulfillment by an external controller.
type AddressesFromPool struct {
	// group of the IP address pool type known to an external IPAM controller.
	// This should be a fully qualified domain name, for example, externalipam.controller.io.
	// +kubebuilder:example=externalipam.controller.io
	// +kubebuilder:validation:Pattern="^$|^[a-z0-9]([-a-z0-9]*[a-z0-9])?(.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
	// +required
	Group string `json:"group"`
	// resource of the IP address pool type known to an external IPAM controller.
	// It is normally the plural form of the resource kind in lowercase, for example,
	// ippools.
	// +kubebuilder:example=ippools
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
	// +required
	Resource string `json:"resource"`
	// name of an IP address pool, for example, pool-config-1.
	// +kubebuilder:example=pool-config-1
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
	// +required
	Name string `json:"name"`
}

// NetworkDeviceSpec defines the network configuration for a virtual machine's
// network device.
type NetworkDeviceSpec struct {
	// networkName is the name of the vSphere network or port group to which the network
	// device will be connected, for example, port-group-1. When not provided, the vCenter
	// API will attempt to select a default network.
	// The available networks (port groups) can be listed using `govc ls 'network/*'`
	// +kubebuilder:example=port-group-1
	// +kubebuilder:validation:MaxLength=80
	// +optional
	NetworkName string `json:"networkName,omitempty"`

	// gateway is an IPv4 or IPv6 address which represents the subnet gateway,
	// for example, 192.168.1.1.
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:Format=ipv6
	// +kubebuilder:example="192.168.1.1"
	// +kubebuilder:example="2001:DB8:0000:0000:244:17FF:FEB6:D37D"
	// +optional
	Gateway string `json:"gateway,omitempty"`

	// ipAddrs is a list of one or more IPv4 and/or IPv6 addresses and CIDR to assign to
	// this device, for example, 192.168.1.100/24. IP addresses provided via ipAddrs are
	// intended to allow explicit assignment of a machine's IP address. IP pool configurations
	// provided via addressesFromPool, however, defer IP address assignment to an external controller.
	// If both addressesFromPool and ipAddrs are empty or not defined, DHCP will be used to assign
	// an IP address. If both ipAddrs and addressesFromPools are defined, the IP addresses associated with
	// ipAddrs will be applied first followed by IP addresses from addressesFromPools.
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:Format=ipv6
	// +kubebuilder:example="192.168.1.100/24"
	// +kubebuilder:example="2001:DB8:0000:0000:244:17FF:FEB6:D37D/64"
	// +optional
	IPAddrs []string `json:"ipAddrs,omitempty"`

	// nameservers is a list of IPv4 and/or IPv6 addresses used as DNS nameservers, for example,
	// 8.8.8.8. a nameserver is not provided by a fulfilled IPAddressClaim. If DHCP is not the
	// source of IP addresses for this network device, nameservers should include a valid nameserver.
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:Format=ipv6
	// +kubebuilder:example="8.8.8.8"
	// +optional
	Nameservers []string `json:"nameservers,omitempty"`

	// addressesFromPools is a list of references to IP pool types and instances which are handled
	// by an external controller. addressesFromPool configurations provided via addressesFromPools
	// defer IP address assignment to an external controller. IP addresses provided via ipAddrs,
	// however, are intended to allow explicit assignment of a machine's IP address. If both
	// addressesFromPool and ipAddrs are empty or not defined, DHCP will assign an IP address.
	// If both ipAddrs and addressesFromPools are defined, the IP addresses associated with
	// ipAddrs will be applied first followed by IP addresses from addressesFromPools.
	// +kubebuilder:validation:Format=ipv4
	// +optional
	AddressesFromPools []AddressesFromPool `json:"addressesFromPools,omitempty"`
}

// VSphereDisk describes additional disks for vSphere.
type VSphereDisk struct {
	// name is used to identify the disk definition. name is required needs to be unique so that it can be used to
	// clearly identify purpose of the disk.
	// It must be at most 80 characters in length and must consist only of alphanumeric characters, hyphens and underscores,
	// and must start and end with an alphanumeric character.
	// +kubebuilder:example=images_1
	// +kubebuilder:validation:MaxLength=80
	// +kubebuilder:validation:Pattern="^[a-zA-Z0-9]([-_a-zA-Z0-9]*[a-zA-Z0-9])?$"
	// +required
	Name string `json:"name"`
	// sizeGiB is the size of the disk in GiB.
	// The maximum supported size 16384 GiB.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=16384
	// +required
	SizeGiB int32 `json:"sizeGiB"`
	// provisioningMode is an optional field that specifies the provisioning type to be used by this vSphere data disk.
	// Allowed values are "Thin", "Thick", "EagerlyZeroed", and omitted.
	// When set to Thin, the disk will be made using thin provisioning allocating the bare minimum space.
	// When set to Thick, the full disk size will be allocated when disk is created.
	// When set to EagerlyZeroed, the disk will be created using eager zero provisioning. An eager zeroed thick disk has all space allocated and wiped clean of any previous contents on the physical media at creation time. Such disks may take longer time during creation compared to other disk formats.
	// When omitted, no setting will be applied to the data disk and the provisioning mode for the disk will be determined by the default storage policy configured for the datastore in vSphere.
	// +optional
	ProvisioningMode ProvisioningMode `json:"provisioningMode,omitempty"`
}

// provisioningMode represents the various provisioning types available to a VMs disk.
// +kubebuilder:validation:Enum=Thin;Thick;EagerlyZeroed
type ProvisioningMode string

const (
	// ProvisioningModeThin creates the disk using thin provisioning. This means a sparse (allocate on demand)
	// format with additional space optimizations.
	ProvisioningModeThin ProvisioningMode = "Thin"

	// ProvisioningModeThick creates the disk with all space allocated.
	ProvisioningModeThick ProvisioningMode = "Thick"

	// ProvisioningModeEagerlyZeroed creates the disk using eager zero provisioning. An eager zeroed thick disk
	// has all space allocated and wiped clean of any previous contents on the physical media at
	// creation time. Such disks may take longer time during creation compared to other disk formats.
	ProvisioningModeEagerlyZeroed ProvisioningMode = "EagerlyZeroed"
)

// WorkspaceConfig defines a workspace configuration for the vSphere cloud
// provider.
type Workspace struct {
	// server is the IP address or FQDN of the vSphere endpoint.
	// +optional
	Server string `gcfg:"server,omitempty" json:"server,omitempty"`
	// datacenter is the datacenter in which VMs are created/located.
	// +optional
	Datacenter string `gcfg:"datacenter,omitempty" json:"datacenter,omitempty"`
	// folder is the folder in which VMs are created/located.
	// +optional
	Folder string `gcfg:"folder,omitempty" json:"folder,omitempty"`
	// datastore is the datastore in which VMs are created/located.
	// +optional
	Datastore string `gcfg:"default-datastore,omitempty" json:"datastore,omitempty"`
	// resourcePool is the resource pool in which VMs are created/located.
	// +optional
	ResourcePool string `gcfg:"resourcepool-path,omitempty" json:"resourcePool,omitempty"`
	// vmGroup is the cluster vm group in which virtual machines will be added for vm host group based zonal.
	// +openshift:validation:featureGate=VSphereHostVMGroupZonal
	// +optional
	VMGroup string `gcfg:"vmGroup,omitempty" json:"vmGroup,omitempty"`
}

// VSphereMachineProviderStatus is the type that will be embedded in a Machine.Status.ProviderStatus field.
// It contains VSphere-specific status information.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type VSphereMachineProviderStatus struct {
	metav1.TypeMeta `json:",inline"`

	// instanceId is the ID of the instance in VSphere
	// +optional
	InstanceID *string `json:"instanceId,omitempty"`
	// instanceState is the provisioning state of the VSphere Instance.
	// +optional
	InstanceState *string `json:"instanceState,omitempty"`
	// conditions is a set of conditions associated with the Machine to indicate
	// errors or other status
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// taskRef is a managed object reference to a Task related to the machine.
	// This value is set automatically at runtime and should not be set or
	// modified by users.
	// +optional
	TaskRef string `json:"taskRef,omitempty"`
}
