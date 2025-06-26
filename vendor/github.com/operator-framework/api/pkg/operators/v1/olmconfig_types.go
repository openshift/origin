package v1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DisabledCopiedCSVsConditionType = "DisabledCopiedCSVs"
)

// OLMConfigSpec is the spec for an OLMConfig resource.
type OLMConfigSpec struct {
	Features *Features `json:"features,omitempty"`
}

// Features contains the list of configurable OLM features.
type Features struct {

	// DisableCopiedCSVs is used to disable OLM's "Copied CSV" feature
	// for operators installed at the cluster scope, where a cluster
	// scoped operator is one that has been installed in an
	// OperatorGroup that targets all namespaces.
	// When reenabled, OLM will recreate the "Copied CSVs" for each
	// cluster scoped operator.
	DisableCopiedCSVs *bool `json:"disableCopiedCSVs,omitempty"`
	// PackageServerSyncInterval is used to define the sync interval for
	// packagerserver pods. Packageserver pods periodically check the
	// status of CatalogSources; this specifies the period using duration
	// format (e.g. "60m"). For this parameter, only hours ("h"), minutes
	// ("m"), and seconds ("s") may be specified. When not specified, the
	// period defaults to the value specified within the packageserver.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(s|m|h))+$"
	PackageServerSyncInterval *metav1.Duration `json:"packageServerSyncInterval,omitempty"`
}

// OLMConfigStatus is the status for an OLMConfig resource.
type OLMConfigStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=olm,scope=Cluster
// +kubebuilder:subresource:status

// OLMConfig is a resource responsible for configuring OLM.
type OLMConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   OLMConfigSpec   `json:"spec,omitempty"`
	Status OLMConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OLMConfigList is a list of OLMConfig resources.
type OLMConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	// +listType=set
	Items []OLMConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OLMConfig{}, &OLMConfigList{})
}

// CopiedCSVsAreEnabled returns true if and only if the olmConfigs DisableCopiedCSVs is set and true,
// otherwise false is returned
func (config *OLMConfig) CopiedCSVsAreEnabled() bool {
	if config == nil || config.Spec.Features == nil || config.Spec.Features.DisableCopiedCSVs == nil {
		return true
	}

	return !*config.Spec.Features.DisableCopiedCSVs
}

func (config *OLMConfig) PackageServerSyncInterval() *time.Duration {
	if config == nil || config.Spec.Features == nil || config.Spec.Features.PackageServerSyncInterval == nil {
		return nil
	}
	return &config.Spec.Features.PackageServerSyncInterval.Duration
}
