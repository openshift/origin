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
	// UserDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance
	// +optional
	UserDataSecret *corev1.LocalObjectReference `json:"userDataSecret,omitempty"`
	// CredentialsSecret is a reference to the secret with vSphere credentials.
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
	// Template is the name, inventory path, or instance UUID of the template
	// used to clone new machines.
	Template string `json:"template"`
	// Workspace describes the workspace to use for the machine.
	// +optional
	Workspace *Workspace `json:"workspace,omitempty"`
	// Network is the network configuration for this machine's VM.
	Network NetworkSpec `json:"network"`
	// NumCPUs is the number of virtual processors in a virtual machine.
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
	// MemoryMiB is the size of a virtual machine's memory, in MiB.
	// Defaults to the analogue property value in the template from which this
	// machine is cloned.
	// +optional
	MemoryMiB int64 `json:"memoryMiB,omitempty"`
	// DiskGiB is the size of a virtual machine's disk, in GiB.
	// Defaults to the analogue property value in the template from which this
	// machine is cloned.
	// This parameter will be ignored if 'LinkedClone' CloneMode is set.
	// +optional
	DiskGiB int32 `json:"diskGiB,omitempty"`
	// Snapshot is the name of the snapshot from which the VM was cloned
	// +optional
	Snapshot string `json:"snapshot"`
	// CloneMode specifies the type of clone operation.
	// The LinkedClone mode is only support for templates that have at least
	// one snapshot. If the template has no snapshots, then CloneMode defaults
	// to FullClone.
	// When LinkedClone mode is enabled the DiskGiB field is ignored as it is
	// not possible to expand disks of linked clones.
	// Defaults to FullClone.
	// When using LinkedClone, if no snapshots exist for the source template, falls back to FullClone.
	// +optional
	CloneMode CloneMode `json:"cloneMode,omitempty"`
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
	// Devices defines the virtual machine's network interfaces.
	Devices []NetworkDeviceSpec `json:"devices"`
}

// AddressesFromPool is an IPAddressPool that will be used to create
// IPAddressClaims for fulfillment by an external controller.
type AddressesFromPool struct {
	// group of the IP address pool type known to an external IPAM controller.
	// This should be a fully qualified domain name, for example, externalipam.controller.io.
	// +kubebuilder:example=externalipam.controller.io
	// +kubebuilder:validation:Pattern:="^$|^[a-z0-9]([-a-z0-9]*[a-z0-9])?(.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
	// +kubebuilder:validation:Required
	Group string `json:"group"`
	// resource of the IP address pool type known to an external IPAM controller.
	// It is normally the plural form of the resource kind in lowercase, for example,
	// ippools.
	// +kubebuilder:example=ippools
	// +kubebuilder:validation:Pattern:="^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
	// +kubebuilder:validation:Required
	Resource string `json:"resource"`
	// name of an IP address pool, for example, pool-config-1.
	// +kubebuilder:example=pool-config-1
	// +kubebuilder:validation:Pattern:="^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
	// +kubebuilder:validation:Required
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
	// +kubebuilder:example=192.168.1.1
	// +kubebuilder:example=2001:DB8:0000:0000:244:17FF:FEB6:D37D
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
	// +kubebuilder:example=192.168.1.100/24
	// +kubebuilder:example=2001:DB8:0000:0000:244:17FF:FEB6:D37D/64
	// +optional
	IPAddrs []string `json:"ipAddrs,omitempty"`

	// nameservers is a list of IPv4 and/or IPv6 addresses used as DNS nameservers, for example,
	// 8.8.8.8. a nameserver is not provided by a fulfilled IPAddressClaim. If DHCP is not the
	// source of IP addresses for this network device, nameservers should include a valid nameserver.
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:Format=ipv6
	// +kubebuilder:example=8.8.8.8
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

// WorkspaceConfig defines a workspace configuration for the vSphere cloud
// provider.
type Workspace struct {
	// Server is the IP address or FQDN of the vSphere endpoint.
	// +optional
	Server string `gcfg:"server,omitempty" json:"server,omitempty"`
	// Datacenter is the datacenter in which VMs are created/located.
	// +optional
	Datacenter string `gcfg:"datacenter,omitempty" json:"datacenter,omitempty"`
	// Folder is the folder in which VMs are created/located.
	// +optional
	Folder string `gcfg:"folder,omitempty" json:"folder,omitempty"`
	// Datastore is the datastore in which VMs are created/located.
	// +optional
	Datastore string `gcfg:"default-datastore,omitempty" json:"datastore,omitempty"`
	// ResourcePool is the resource pool in which VMs are created/located.
	// +optional
	ResourcePool string `gcfg:"resourcepool-path,omitempty" json:"resourcePool,omitempty"`
}

// VSphereMachineProviderStatus is the type that will be embedded in a Machine.Status.ProviderStatus field.
// It contains VSphere-specific status information.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type VSphereMachineProviderStatus struct {
	metav1.TypeMeta `json:",inline"`

	// InstanceID is the ID of the instance in VSphere
	// +optional
	InstanceID *string `json:"instanceId,omitempty"`
	// InstanceState is the provisioning state of the VSphere Instance.
	// +optional
	InstanceState *string `json:"instanceState,omitempty"`
	// Conditions is a set of conditions associated with the Machine to indicate
	// errors or other status
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// TaskRef is a managed object reference to a Task related to the machine.
	// This value is set automatically at runtime and should not be set or
	// modified by users.
	// +optional
	TaskRef string `json:"taskRef,omitempty"`
}
