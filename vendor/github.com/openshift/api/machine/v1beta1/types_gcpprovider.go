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

// SecureBootPolicy represents the secure boot configuration for the GCP machine.
type SecureBootPolicy string

const (
	// SecureBootPolicyEnabled enables the secure boot configuration for the GCP machine.
	SecureBootPolicyEnabled SecureBootPolicy = "Enabled"
	// SecureBootPolicyDisabled disables the secure boot configuration for the GCP machine.
	SecureBootPolicyDisabled SecureBootPolicy = "Disabled"
)

// VirtualizedTrustedPlatformModulePolicy represents the virtualized trusted platform module configuration for the GCP machine.
type VirtualizedTrustedPlatformModulePolicy string

const (
	// VirtualizedTrustedPlatformModulePolicyEnabled enables the virtualized trusted platform module configuration for the GCP machine.
	VirtualizedTrustedPlatformModulePolicyEnabled VirtualizedTrustedPlatformModulePolicy = "Enabled"
	// VirtualizedTrustedPlatformModulePolicyDisabled disables the virtualized trusted platform module configuration for the GCP machine.
	VirtualizedTrustedPlatformModulePolicyDisabled VirtualizedTrustedPlatformModulePolicy = "Disabled"
)

// IntegrityMonitoringPolicy represents the integrity monitoring configuration for the GCP machine.
type IntegrityMonitoringPolicy string

const (
	// IntegrityMonitoringPolicyEnabled enables integrity monitoring for the GCP machine.
	IntegrityMonitoringPolicyEnabled IntegrityMonitoringPolicy = "Enabled"
	// IntegrityMonitoringPolicyDisabled disables integrity monitoring for the GCP machine.
	IntegrityMonitoringPolicyDisabled IntegrityMonitoringPolicy = "Disabled"
)

// ConfidentialComputePolicy represents the confidential compute configuration for the GCP machine.
type ConfidentialComputePolicy string

const (
	// ConfidentialComputePolicyEnabled enables confidential compute for the GCP machine.
	ConfidentialComputePolicyEnabled ConfidentialComputePolicy = "Enabled"
	// ConfidentialComputePolicyDisabled disables confidential compute for the GCP machine.
	ConfidentialComputePolicyDisabled ConfidentialComputePolicy = "Disabled"
)

// GCPMachineProviderSpec is the type that will be embedded in a Machine.Spec.ProviderSpec field
// for an GCP virtual machine. It is used by the GCP machine actuator to create a single Machine.
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GCPMachineProviderSpec struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// userDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance
	// +optional
	UserDataSecret *corev1.LocalObjectReference `json:"userDataSecret,omitempty"`
	// credentialsSecret is a reference to the secret with GCP credentials.
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
	// canIPForward Allows this instance to send and receive packets with non-matching destination or source IPs.
	// This is required if you plan to use this instance to forward routes.
	CanIPForward bool `json:"canIPForward"`
	// deletionProtection whether the resource should be protected against deletion.
	DeletionProtection bool `json:"deletionProtection"`
	// disks is a list of disks to be attached to the VM.
	// +optional
	Disks []*GCPDisk `json:"disks,omitempty"`
	// labels list of labels to apply to the VM.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Metadata key/value pairs to apply to the VM.
	// +optional
	Metadata []*GCPMetadata `json:"gcpMetadata,omitempty"`
	// networkInterfaces is a list of network interfaces to be attached to the VM.
	// +optional
	NetworkInterfaces []*GCPNetworkInterface `json:"networkInterfaces,omitempty"`
	// serviceAccounts is a list of GCP service accounts to be used by the VM.
	ServiceAccounts []GCPServiceAccount `json:"serviceAccounts"`
	// tags list of network tags to apply to the VM.
	Tags []string `json:"tags,omitempty"`
	// targetPools are used for network TCP/UDP load balancing. A target pool references member instances,
	// an associated legacy HttpHealthCheck resource, and, optionally, a backup target pool
	// +optional
	TargetPools []string `json:"targetPools,omitempty"`
	// machineType is the machine type to use for the VM.
	MachineType string `json:"machineType"`
	// region is the region in which the GCP machine provider will create the VM.
	Region string `json:"region"`
	// zone is the zone in which the GCP machine provider will create the VM.
	Zone string `json:"zone"`
	// projectID is the project in which the GCP machine provider will create the VM.
	// +optional
	ProjectID string `json:"projectID,omitempty"`
	// gpus is a list of GPUs to be attached to the VM.
	// +optional
	GPUs []GCPGPUConfig `json:"gpus,omitempty"`
	// preemptible indicates if created instance is preemptible.
	// +optional
	Preemptible bool `json:"preemptible,omitempty"`
	// onHostMaintenance determines the behavior when a maintenance event occurs that might cause the instance to reboot.
	// This is required to be set to "Terminate" if you want to provision machine with attached GPUs.
	// Otherwise, allowed values are "Migrate" and "Terminate".
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is "Migrate".
	// +kubebuilder:validation:Enum=Migrate;Terminate;
	// +optional
	OnHostMaintenance GCPHostMaintenanceType `json:"onHostMaintenance,omitempty"`
	// restartPolicy determines the behavior when an instance crashes or the underlying infrastructure provider stops the instance as part of a maintenance event (default "Always").
	// Cannot be "Always" with preemptible instances.
	// Otherwise, allowed values are "Always" and "Never".
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is "Always".
	// RestartPolicy represents AutomaticRestart in GCP compute api
	// +kubebuilder:validation:Enum=Always;Never;
	// +optional
	RestartPolicy GCPRestartPolicyType `json:"restartPolicy,omitempty"`

	// shieldedInstanceConfig is the Shielded VM configuration for the VM
	// +optional
	ShieldedInstanceConfig GCPShieldedInstanceConfig `json:"shieldedInstanceConfig,omitempty"`

	// confidentialCompute Defines whether the instance should have confidential compute enabled.
	// If enabled OnHostMaintenance is required to be set to "Terminate".
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is false.
	// +kubebuilder:validation:Enum=Enabled;Disabled
	// +optional
	ConfidentialCompute ConfidentialComputePolicy `json:"confidentialCompute,omitempty"`

	// resourceManagerTags is an optional list of tags to apply to the GCP resources created for
	// the cluster. See https://cloud.google.com/resource-manager/docs/tags/tags-overview for
	// information on tagging GCP resources. GCP supports a maximum of 50 tags per resource.
	// +kubebuilder:validation:MaxItems=50
	// +listType=map
	// +listMapKey=key
	// +optional
	ResourceManagerTags []ResourceManagerTag `json:"resourceManagerTags,omitempty"`
}

// ResourceManagerTag is a tag to apply to GCP resources created for the cluster.
type ResourceManagerTag struct {
	// parentID is the ID of the hierarchical resource where the tags are defined
	// e.g. at the Organization or the Project level. To find the Organization or Project ID ref
	// https://cloud.google.com/resource-manager/docs/creating-managing-organization#retrieving_your_organization_id
	// https://cloud.google.com/resource-manager/docs/creating-managing-projects#identifying_projects
	// An OrganizationID can have a maximum of 32 characters and must consist of decimal numbers, and
	// cannot have leading zeroes. A ProjectID must be 6 to 30 characters in length, can only contain
	// lowercase letters, numbers, and hyphens, and must start with a letter, and cannot end with a hyphen.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern=`(^[1-9][0-9]{0,31}$)|(^[a-z][a-z0-9-]{4,28}[a-z0-9]$)`
	ParentID string `json:"parentID"`

	// key is the key part of the tag. A tag key can have a maximum of 63 characters and cannot be empty.
	// Tag key must begin and end with an alphanumeric character, and must contain only uppercase, lowercase
	// alphanumeric characters, and the following special characters `._-`.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([0-9A-Za-z_.-]{0,61}[a-zA-Z0-9])?$`
	Key string `json:"key"`

	// value is the value part of the tag. A tag value can have a maximum of 63 characters and cannot be empty.
	// Tag value must begin and end with an alphanumeric character, and must contain only uppercase, lowercase
	// alphanumeric characters, and the following special characters `_-.@%=+:,*#&(){}[]` and spaces.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([0-9A-Za-z_.@%=+:,*#&()\[\]{}\-\s]{0,61}[a-zA-Z0-9])?$`
	Value string `json:"value"`
}

// GCPDisk describes disks for GCP.
type GCPDisk struct {
	// autoDelete indicates if the disk will be auto-deleted when the instance is deleted (default false).
	AutoDelete bool `json:"autoDelete"`
	// boot indicates if this is a boot disk (default false).
	Boot bool `json:"boot"`
	// sizeGb is the size of the disk (in GB).
	SizeGB int64 `json:"sizeGb"`
	// type is the type of the disk (eg: pd-standard).
	Type string `json:"type"`
	// image is the source image to create this disk.
	Image string `json:"image"`
	// labels list of labels to apply to the disk.
	Labels map[string]string `json:"labels"`
	// encryptionKey is the customer-supplied encryption key of the disk.
	// +optional
	EncryptionKey *GCPEncryptionKeyReference `json:"encryptionKey,omitempty"`
}

// GCPMetadata describes metadata for GCP.
type GCPMetadata struct {
	// key is the metadata key.
	Key string `json:"key"`
	// value is the metadata value.
	Value *string `json:"value"`
}

// GCPNetworkInterface describes network interfaces for GCP
type GCPNetworkInterface struct {
	// publicIP indicates if true a public IP will be used
	PublicIP bool `json:"publicIP,omitempty"`
	// network is the network name.
	Network string `json:"network,omitempty"`
	// projectID is the project in which the GCP machine provider will create the VM.
	ProjectID string `json:"projectID,omitempty"`
	// subnetwork is the subnetwork name.
	Subnetwork string `json:"subnetwork,omitempty"`
}

// GCPServiceAccount describes service accounts for GCP.
type GCPServiceAccount struct {
	// email is the service account email.
	Email string `json:"email"`
	// scopes list of scopes to be assigned to the service account.
	Scopes []string `json:"scopes"`
}

// GCPEncryptionKeyReference describes the encryptionKey to use for a disk's encryption.
type GCPEncryptionKeyReference struct {
	// KMSKeyName is the reference KMS key, in the format
	// +optional
	KMSKey *GCPKMSKeyReference `json:"kmsKey,omitempty"`
	// kmsKeyServiceAccount is the service account being used for the
	// encryption request for the given KMS key. If absent, the Compute
	// Engine default service account is used.
	// See https://cloud.google.com/compute/docs/access/service-accounts#compute_engine_service_account
	// for details on the default service account.
	// +optional
	KMSKeyServiceAccount string `json:"kmsKeyServiceAccount,omitempty"`
}

// GCPKMSKeyReference gathers required fields for looking up a GCP KMS Key
type GCPKMSKeyReference struct {
	// name is the name of the customer managed encryption key to be used for the disk encryption.
	Name string `json:"name"`
	// keyRing is the name of the KMS Key Ring which the KMS Key belongs to.
	KeyRing string `json:"keyRing"`
	// projectID is the ID of the Project in which the KMS Key Ring exists.
	// Defaults to the VM ProjectID if not set.
	// +optional
	ProjectID string `json:"projectID,omitempty"`
	// location is the GCP location in which the Key Ring exists.
	Location string `json:"location"`
}

// GCPGPUConfig describes type and count of GPUs attached to the instance on GCP.
type GCPGPUConfig struct {
	// count is the number of GPUs to be attached to an instance.
	Count int32 `json:"count"`
	// type is the type of GPU to be attached to an instance.
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
	// instanceId is the ID of the instance in GCP
	// +optional
	InstanceID *string `json:"instanceId,omitempty"`
	// instanceState is the provisioning state of the GCP Instance.
	// +optional
	InstanceState *string `json:"instanceState,omitempty"`
	// conditions is a set of conditions associated with the Machine to indicate
	// errors or other status
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// GCPShieldedInstanceConfig describes the shielded VM configuration of the instance on GCP.
// Shielded VM configuration allow users to enable and disable Secure Boot, vTPM, and Integrity Monitoring.
type GCPShieldedInstanceConfig struct {
	// secureBoot Defines whether the instance should have secure boot enabled.
	// Secure Boot verify the digital signature of all boot components, and halting the boot process if signature verification fails.
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is Disabled.
	// +kubebuilder:validation:Enum=Enabled;Disabled
	//+optional
	SecureBoot SecureBootPolicy `json:"secureBoot,omitempty"`

	// virtualizedTrustedPlatformModule enable virtualized trusted platform module measurements to create a known good boot integrity policy baseline.
	// The integrity policy baseline is used for comparison with measurements from subsequent VM boots to determine if anything has changed.
	// This is required to be set to "Enabled" if IntegrityMonitoring is enabled.
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is Enabled.
	// +kubebuilder:validation:Enum=Enabled;Disabled
	// +optional
	VirtualizedTrustedPlatformModule VirtualizedTrustedPlatformModulePolicy `json:"virtualizedTrustedPlatformModule,omitempty"`

	// integrityMonitoring determines whether the instance should have integrity monitoring that verify the runtime boot integrity.
	// Compares the most recent boot measurements to the integrity policy baseline and return
	// a pair of pass/fail results depending on whether they match or not.
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is Enabled.
	// +kubebuilder:validation:Enum=Enabled;Disabled
	// +optional
	IntegrityMonitoring IntegrityMonitoringPolicy `json:"integrityMonitoring,omitempty"`
}
