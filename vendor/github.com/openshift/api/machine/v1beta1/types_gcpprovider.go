package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GCPHostMaintenanceType is a type representing acceptable values for OnHostMaintenance field in GCPMachineProviderSpec
type GCPHostMaintenanceType string

const (
	// MigrateHostMaintenanceType [default] - causes Compute Engine to live migrate an instance when there is a maintenance event.
	MigrateHostMaintenanceType GCPHostMaintenanceType = "Migrate"
	// TerminateHostMaintenanceType - stops an instance instead of migrating it.
	TerminateHostMaintenanceType GCPHostMaintenanceType = "Terminate"
)

// GCPHostMaintenanceType is a type representing acceptable values for RestartPolicy field in GCPMachineProviderSpec
type GCPRestartPolicyType string

const (
	// Restart an instance if an instance crashes or the underlying infrastructure provider stops the instance as part of a maintenance event.
	RestartPolicyAlways GCPRestartPolicyType = "Always"
	// Do not restart an instance if an instance crashes or the underlying infrastructure provider stops the instance as part of a maintenance event.
	RestartPolicyNever GCPRestartPolicyType = "Never"
)

// GCPMachineProviderSpec is the type that will be embedded in a Machine.Spec.ProviderSpec field
// for an GCP virtual machine. It is used by the GCP machine actuator to create a single Machine.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GCPMachineProviderSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// UserDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance
	// +optional
	UserDataSecret *corev1.LocalObjectReference `json:"userDataSecret,omitempty"`
	// CredentialsSecret is a reference to the secret with GCP credentials.
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
	// CanIPForward Allows this instance to send and receive packets with non-matching destination or source IPs.
	// This is required if you plan to use this instance to forward routes.
	CanIPForward bool `json:"canIPForward"`
	// DeletionProtection whether the resource should be protected against deletion.
	DeletionProtection bool `json:"deletionProtection"`
	// Disks is a list of disks to be attached to the VM.
	// +optional
	Disks []*GCPDisk `json:"disks,omitempty"`
	// Labels list of labels to apply to the VM.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Metadata key/value pairs to apply to the VM.
	// +optional
	Metadata []*GCPMetadata `json:"gcpMetadata,omitempty"`
	// NetworkInterfaces is a list of network interfaces to be attached to the VM.
	// +optional
	NetworkInterfaces []*GCPNetworkInterface `json:"networkInterfaces,omitempty"`
	// ServiceAccounts is a list of GCP service accounts to be used by the VM.
	ServiceAccounts []GCPServiceAccount `json:"serviceAccounts"`
	// Tags list of tags to apply to the VM.
	Tags []string `json:"tags,omitempty"`
	// TargetPools are used for network TCP/UDP load balancing. A target pool references member instances,
	// an associated legacy HttpHealthCheck resource, and, optionally, a backup target pool
	// +optional
	TargetPools []string `json:"targetPools,omitempty"`
	// MachineType is the machine type to use for the VM.
	MachineType string `json:"machineType"`
	// Region is the region in which the GCP machine provider will create the VM.
	Region string `json:"region"`
	// Zone is the zone in which the GCP machine provider will create the VM.
	Zone string `json:"zone"`
	// ProjectID is the project in which the GCP machine provider will create the VM.
	// +optional
	ProjectID string `json:"projectID,omitempty"`
	// GPUs is a list of GPUs to be attached to the VM.
	// +optional
	GPUs []GCPGPUConfig `json:"gpus,omitempty"`
	// Preemptible indicates if created instance is preemptible.
	// +optional
	Preemptible bool `json:"preemptible,omitempty"`
	// OnHostMaintenance determines the behavior when a maintenance event occurs that might cause the instance to reboot.
	// This is required to be set to "Terminate" if you want to provision machine with attached GPUs.
	// Otherwise, allowed values are "Migrate" and "Terminate".
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is "Migrate".
	// +kubebuilder:validation:Enum=Migrate;Terminate;
	// +optional
	OnHostMaintenance GCPHostMaintenanceType `json:"onHostMaintenance,omitempty"`
	// RestartPolicy determines the behavior when an instance crashes or the underlying infrastructure provider stops the instance as part of a maintenance event (default "Always").
	// Cannot be "Always" with preemptible instances.
	// Otherwise, allowed values are "Always" and "Never".
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is "Always".
	// RestartPolicy represents AutomaticRestart in GCP compute api
	// +kubebuilder:validation:Enum=Always;Never;
	// +optional
	RestartPolicy GCPRestartPolicyType `json:"restartPolicy,omitempty"`
}

// GCPDisk describes disks for GCP.
type GCPDisk struct {
	// AutoDelete indicates if the disk will be auto-deleted when the instance is deleted (default false).
	AutoDelete bool `json:"autoDelete"`
	// Boot indicates if this is a boot disk (default false).
	Boot bool `json:"boot"`
	// SizeGB is the size of the disk (in GB).
	SizeGB int64 `json:"sizeGb"`
	// Type is the type of the disk (eg: pd-standard).
	Type string `json:"type"`
	// Image is the source image to create this disk.
	Image string `json:"image"`
	// Labels list of labels to apply to the disk.
	Labels map[string]string `json:"labels"`
	// EncryptionKey is the customer-supplied encryption key of the disk.
	// +optional
	EncryptionKey *GCPEncryptionKeyReference `json:"encryptionKey,omitempty"`
}

// GCPMetadata describes metadata for GCP.
type GCPMetadata struct {
	// Key is the metadata key.
	Key string `json:"key"`
	// Value is the metadata value.
	Value *string `json:"value"`
}

// GCPNetworkInterface describes network interfaces for GCP
type GCPNetworkInterface struct {
	// PublicIP indicates if true a public IP will be used
	PublicIP bool `json:"publicIP,omitempty"`
	// Network is the network name.
	Network string `json:"network,omitempty"`
	// ProjectID is the project in which the GCP machine provider will create the VM.
	ProjectID string `json:"projectID,omitempty"`
	// Subnetwork is the subnetwork name.
	Subnetwork string `json:"subnetwork,omitempty"`
}

// GCPServiceAccount describes service accounts for GCP.
type GCPServiceAccount struct {
	// Email is the service account email.
	Email string `json:"email"`
	// Scopes list of scopes to be assigned to the service account.
	Scopes []string `json:"scopes"`
}

// GCPEncryptionKeyReference describes the encryptionKey to use for a disk's encryption.
type GCPEncryptionKeyReference struct {
	// KMSKeyName is the reference KMS key, in the format
	// +optional
	KMSKey *GCPKMSKeyReference `json:"kmsKey,omitempty"`
	// KMSKeyServiceAccount is the service account being used for the
	// encryption request for the given KMS key. If absent, the Compute
	// Engine default service account is used.
	// See https://cloud.google.com/compute/docs/access/service-accounts#compute_engine_service_account
	// for details on the default service account.
	// +optional
	KMSKeyServiceAccount string `json:"kmsKeyServiceAccount,omitempty"`
}

// GCPKMSKeyReference gathers required fields for looking up a GCP KMS Key
type GCPKMSKeyReference struct {
	// Name is the name of the customer managed encryption key to be used for the disk encryption.
	Name string `json:"name"`
	// KeyRing is the name of the KMS Key Ring which the KMS Key belongs to.
	KeyRing string `json:"keyRing"`
	// ProjectID is the ID of the Project in which the KMS Key Ring exists.
	// Defaults to the VM ProjectID if not set.
	// +optional
	ProjectID string `json:"projectID,omitempty"`
	// Location is the GCP location in which the Key Ring exists.
	Location string `json:"location"`
}

// GCPGPUConfig describes type and count of GPUs attached to the instance on GCP.
type GCPGPUConfig struct {
	// Count is the number of GPUs to be attached to an instance.
	Count int32 `json:"count"`
	// Type is the type of GPU to be attached to an instance.
	// Supported GPU types are: nvidia-tesla-k80, nvidia-tesla-p100, nvidia-tesla-v100, nvidia-tesla-p4, nvidia-tesla-t4
	// +kubebuilder:validation:Pattern=`^nvidia-tesla-(k80|p100|v100|p4|t4)$`
	Type string `json:"type"`
}

// GCPMachineProviderStatus is the type that will be embedded in a Machine.Status.ProviderStatus field.
// It contains GCP-specific status information.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type GCPMachineProviderStatus struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// InstanceID is the ID of the instance in GCP
	// +optional
	InstanceID *string `json:"instanceId,omitempty"`
	// InstanceState is the provisioning state of the GCP Instance.
	// +optional
	InstanceState *string `json:"instanceState,omitempty"`
	// Conditions is a set of conditions associated with the Machine to indicate
	// errors or other status
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
