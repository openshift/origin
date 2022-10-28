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
	// Defaults to LinkedClone, but fails gracefully to FullClone if the source
	// of the clone operation has no snapshots.
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

// NetworkDeviceSpec defines the network configuration for a virtual machine's
// network device.
type NetworkDeviceSpec struct {
	// NetworkName is the name of the vSphere network to which the device
	// will be connected.
	NetworkName string `json:"networkName"`
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
