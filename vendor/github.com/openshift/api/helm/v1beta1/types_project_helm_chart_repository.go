package v1beta1

import (
	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:plural=projecthelmchartrepositories

// ProjectHelmChartRepository holds namespace-wide configuration for proxied Helm chart repository
//
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=projecthelmchartrepositories,scope=Namespaced
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/1084
// +openshift:file-pattern=operatorOrdering=00
type ProjectHelmChartRepository struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +required
	Spec ProjectHelmChartRepositorySpec `json:"spec"`

	// Observed status of the repository within the namespace..
	// +optional
	Status HelmChartRepositoryStatus `json:"status"`
}

// Project Helm chart repository exposed within a namespace
type ProjectHelmChartRepositorySpec struct {

	// If set to true, disable the repo usage in the namespace
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
	ProjectConnectionConfig ConnectionConfigNamespaceScoped `json:"connectionConfig"`
}

type ConnectionConfigNamespaceScoped struct {

	// Chart repository URL
	// +kubebuilder:validation:Pattern=`^https?:\/\/`
	// +kubebuilder:validation:MaxLength=2048
	URL string `json:"url"`

	// ca is an optional reference to a config map by name containing the PEM-encoded CA bundle.
	// It is used as a trust anchor to validate the TLS certificate presented by the remote server.
	// The key "ca-bundle.crt" is used to locate the data.
	// If empty, the default system roots are used.
	// The namespace for this configmap must be same as the namespace where the project helm chart repository is getting instantiated.
	// +optional
	CA configv1.ConfigMapNameReference `json:"ca,omitempty"`

	// tlsClientConfig is an optional reference to a secret by name that contains the
	// PEM-encoded TLS client certificate and private key to present when connecting to the server.
	// The key "tls.crt" is used to locate the client certificate.
	// The key "tls.key" is used to locate the private key.
	// The namespace for this secret must be same as the namespace where the project helm chart repository is getting instantiated.
	// +optional
	TLSClientConfig configv1.SecretNameReference `json:"tlsClientConfig,omitempty"`

	// basicAuthConfig is an optional reference to a secret by name that contains
	// the basic authentication credentials to present when connecting to the server.
	// The key "username" is used locate the username.
	// The key "password" is used to locate the password.
	// The namespace for this secret must be same as the namespace where the project helm chart repository is getting instantiated.
	// +optional
	BasicAuthConfig configv1.SecretNameReference `json:"basicAuthConfig,omitempty"`
}

// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:compatibility-gen:level=2
type ProjectHelmChartRepositoryList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []ProjectHelmChartRepository `json:"items"`
}
