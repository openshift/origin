package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Healthy;Unhealthy;Error
type KMSPluginHealthStatus string

const (
	KMSPluginHealthStatusHealthy KMSPluginHealthStatus = "Healthy"

	KMSPluginHealthStatusUnhealthy KMSPluginHealthStatus = "Unhealthy"

	KMSPluginHealthStatusError KMSPluginHealthStatus = "Error"
)

type KMSPluginHealthReport struct {

	// nodeName is the name of the node this instance of the plugin runs on.
	// The combination of nodeName and keyID makes this health report unique.
	// The value must be a valid Kubernetes node name: a lowercase RFC 1123 subdomain
	// consisting of lowercase alphanumeric characters, '-' or '.', starting and ending with
	// an alphanumeric character, and be at most 253 characters in length.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="nodeName must be a lowercase RFC 1123 subdomain consisting of lowercase alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
	// +required
	NodeName string `json:"nodeName,omitempty"`

	// keyID is the encryption-key-secret id (kms-{keyID}.sock), a unique identifier of the plugin on that node.
	// This is not a cryptographic key used to encrypt/decrypt any resources.
	// The value must be between 1 and 512 characters.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	// +required
	KeyID string `json:"keyID,omitempty"`

	// --- TOMBSTONE ---
	// keyId is the encryption-key-secret id (kms-{keyId}.sock), a unique identifier of the plugin on that node.
	// This is not a cryptographic key used to encrypt/decrypt any resources.
	// The value must be between 1 and 512 characters.
	// It has been renamed to keyID.
	// The field name is reserved to prevent reuse.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	// +required
	// KeyId string `json:"keyId,omitempty"`

	// status contains a health indicator for the respective KMS plugin
	// The field can have three states: healthy, unhealthy, error.
	// With error and unhealthy containing additional information in Detail.
	// +required
	Status KMSPluginHealthStatus `json:"status,omitempty"`

	// lastCheckedTime is a timestamp of when the probe was last checked.
	// +required
	LastCheckedTime metav1.Time `json:"lastCheckedTime,omitempty"`

	// remoteKeyID refers to the remote key identifier from KMS v2 StatusResponse.key_id.
	// This is not a cryptographic key, but a unique representation of the KEK.
	// The value must be between 1 and 1024 characters.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	// +required
	RemoteKeyID string `json:"remoteKeyID,omitempty"`

	// --- TOMBSTONE ---
	// kekId refers to the remote KEK id from KMS v2 StatusResponse.key_id.
	// This is not a cryptographic key, but a unique representation of the KEK.
	// The value must be between 1 and 1024 characters.
	// It has been renamed to remoteKeyID.
	// The field name is reserved to prevent reuse.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	// +required
	// KEKId string `json:"kekId,omitempty"`

	// detail contains additional error/health information for the respective KMS plugin.
	// When omitted, no additional error or health information is provided.
	// When set, the value must be between 1 and 1024 characters.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	// +optional
	Detail string `json:"detail,omitempty"`
}

// +openshift:compatibility-gen:level=1
// +kubebuilder:validation:MinProperties=1
type KMSEncryptionStatus struct {
	// healthReports contains all KMS plugin health reports.
	// When omitted, no health reports are available.
	// Each entry must have a unique combination of nodeName and keyID.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=200
	// +listType=map
	// +listMapKey=nodeName
	// +listMapKey=keyID
	HealthReports []KMSPluginHealthReport `json:"healthReports,omitempty"`

	// preflight contains the state of KMS preflight validation for this operator.
	// The preflight validates the KMS provider configuration before it is used
	// to create a new encryption key, catching configuration issues early such
	// as incorrect login credentials or an unreachable Vault service.
	// When omitted, no preflight validation is in progress.
	// +optional
	Preflight KMSPreflightCheck `json:"preflight,omitzero"`
}

// KMSPreflightCheck describes a preflight validation request and its result.
//
// +kubebuilder:validation:MinProperties=1
type KMSPreflightCheck struct {
	// observedConfigHash is a hash of the KMS provider configuration and
	// its referenced resources that has been observed and requires preflight
	// validation before a new encryption key can be created.
	// The value must be exactly 8 characters.
	// +kubebuilder:validation:MinLength=8
	// +kubebuilder:validation:MaxLength=8
	// +kubebuilder:validation:XValidation:rule="self.matches('^[A-Za-z0-9_-]*={0,2}$')",message="must be a valid base64url encoded value"
	// +required
	ObservedConfigHash string `json:"observedConfigHash,omitempty"`

	// result contains the outcome of the most recent preflight check.
	// Preflight is considered passed when result.status is Succeeded and
	// result.configHash matches observedConfigHash.
	// When omitted, no preflight check result has been reported yet.
	// +optional
	Result KMSPreflightResult `json:"result,omitzero"`
}

// +kubebuilder:validation:Enum=Succeeded;Failed
type KMSPreflightResultStatus string

const (
	KMSPreflightResultSucceeded KMSPreflightResultStatus = "Succeeded"

	KMSPreflightResultFailed KMSPreflightResultStatus = "Failed"
)

// KMSPreflightResult contains the outcome of a preflight validation.
//
// +openshift:compatibility-gen:level=1
type KMSPreflightResult struct {
	// status indicates the outcome of the preflight check.
	// Succeeded means the KMS plugin responded to Status, Encrypt, and
	// Decrypt calls successfully.
	// Failed means the validation did not pass.
	// +required
	Status KMSPreflightResultStatus `json:"status,omitempty"`

	// configHash is the hash of the configuration that was validated.
	// This is compared against observedConfigHash to confirm the result
	// corresponds to the current configuration.
	// The value must be exactly 8 characters.
	// +kubebuilder:validation:MinLength=8
	// +kubebuilder:validation:MaxLength=8
	// +kubebuilder:validation:XValidation:rule="self.matches('^[A-Za-z0-9_-]*={0,2}$')",message="must be a valid base64url encoded value"
	// +required
	ConfigHash string `json:"configHash,omitempty"`

	// remoteKeyID is the remote key encryption key identifier from KMS v2
	// StatusResponse.key_id. This is not a cryptographic key, but a unique
	// representation of the remote key used to encrypt data.
	// The value must be between 1 and 1024 characters.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	// +required
	RemoteKeyID string `json:"remoteKeyID,omitempty"`
}
