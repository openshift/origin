package v1beta1

import (
	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:plural=helmchartrepositories

// HelmChartRepository holds cluster-wide configuration for proxied Helm chart repository
//
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type HelmChartRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +kubebuilder:validation:Required
	// +required
	Spec HelmChartRepositorySpec `json:"spec"`

	// Observed status of the repository within the cluster..
	// +optional
	Status HelmChartRepositoryStatus `json:"status"`
}

// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:compatibility-gen:level=2
type HelmChartRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []HelmChartRepository `json:"items"`
}

// Helm chart repository exposed within the cluster
type HelmChartRepositorySpec struct {

	// If set to true, disable the repo usage in the cluster/namespace
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	// Optional associated human readable repository name, it can be used by UI for displaying purposes
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	// +optional
	DisplayName string `json:"name,omitempty"`

	// Optional human readable repository description, it can be used by UI for displaying purposes
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +optional
	Description string `json:"description,omitempty"`

	// Required configuration for connecting to the chart repo
	ConnectionConfig ConnectionConfig `json:"connectionConfig"`
}

type ConnectionConfig struct {

	// Chart repository URL
	// +kubebuilder:validation:Pattern=`^https?:\/\/`
	// +kubebuilder:validation:MaxLength=2048
	URL string `json:"url"`

	// ca is an optional reference to a config map by name containing the PEM-encoded CA bundle.
	// It is used as a trust anchor to validate the TLS certificate presented by the remote server.
	// The key "ca-bundle.crt" is used to locate the data.
	// If empty, the default system roots are used.
	// The namespace for this config map is openshift-config.
	// +optional
	CA configv1.ConfigMapNameReference `json:"ca,omitempty"`

	// tlsClientConfig is an optional reference to a secret by name that contains the
	// PEM-encoded TLS client certificate and private key to present when connecting to the server.
	// The key "tls.crt" is used to locate the client certificate.
	// The key "tls.key" is used to locate the private key.
	// The namespace for this secret is openshift-config.
	// +optional
	TLSClientConfig configv1.SecretNameReference `json:"tlsClientConfig,omitempty"`
}

type HelmChartRepositoryStatus struct {

	// conditions is a list of conditions and their statuses
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
