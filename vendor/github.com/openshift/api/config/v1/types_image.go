package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Image governs policies related to imagestream imports and runtime configuration
// for external registries. It allows cluster admins to configure which registries
// OpenShift is allowed to import images from, extra CA trust bundles for external
// registries, and policies to block or allow registry hostnames.
// When exposing OpenShift's image registry to the public, this also lets cluster
// admins specify the external hostname.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/470
// +openshift:file-pattern=cvoRunLevel=0000_10,operatorName=config-operator,operatorOrdering=01
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=images,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:metadata:annotations=release.openshift.io/bootstrap-required=true
type Image struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +kubebuilder:validation:Required
	// +required
	Spec ImageSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status ImageStatus `json:"status"`
}

// ImportModeType describes how to import an image manifest.
// +enum
// +kubebuilder:validation:Enum:="";Legacy;PreserveOriginal
type ImportModeType string

const (
	// ImportModeLegacy indicates that the legacy behaviour should be used.
	// For manifest lists, the legacy behaviour will discard the manifest list and import a single
	// sub-manifest. In this case, the platform is chosen in the following order of priority:
	// 1. tag annotations; 2. control plane arch/os; 3. linux/amd64; 4. the first manifest in the list.
	// This mode is the default.
	ImportModeLegacy ImportModeType = "Legacy"
	// ImportModePreserveOriginal indicates that the original manifest will be preserved.
	// For manifest lists, the manifest list and all its sub-manifests will be imported.
	ImportModePreserveOriginal ImportModeType = "PreserveOriginal"
)

type ImageSpec struct {
	// allowedRegistriesForImport limits the container image registries that normal users may import
	// images from. Set this list to the registries that you trust to contain valid Docker
	// images and that you want applications to be able to import from. Users with
	// permission to create Images or ImageStreamMappings via the API are not affected by
	// this policy - typically only administrators or system integrations will have those
	// permissions.
	// +optional
	// +listType=atomic
	AllowedRegistriesForImport []RegistryLocation `json:"allowedRegistriesForImport,omitempty"`

	// externalRegistryHostnames provides the hostnames for the default external image
	// registry. The external hostname should be set only when the image registry
	// is exposed externally. The first value is used in 'publicDockerImageRepository'
	// field in ImageStreams. The value must be in "hostname[:port]" format.
	// +optional
	// +listType=atomic
	ExternalRegistryHostnames []string `json:"externalRegistryHostnames,omitempty"`

	// additionalTrustedCA is a reference to a ConfigMap containing additional CAs that
	// should be trusted during imagestream import, pod image pull, build image pull, and
	// imageregistry pullthrough.
	// The namespace for this config map is openshift-config.
	// +optional
	AdditionalTrustedCA ConfigMapNameReference `json:"additionalTrustedCA"`

	// registrySources contains configuration that determines how the container runtime
	// should treat individual registries when accessing images for builds+pods. (e.g.
	// whether or not to allow insecure access).  It does not contain configuration for the
	// internal cluster registry.
	// +optional
	RegistrySources RegistrySources `json:"registrySources"`

	// imageStreamImportMode controls the import mode behaviour of imagestreams.
	// It can be set to `Legacy` or `PreserveOriginal` or the empty string. If this value
	// is specified, this setting is applied to all newly created imagestreams which do not have the
	// value set. `Legacy` indicates that the legacy behaviour should be used.
	// For manifest lists, the legacy behaviour will discard the manifest list and import a single
	// sub-manifest. In this case, the platform is chosen in the following order of priority:
	// 1. tag annotations; 2. control plane arch/os; 3. linux/amd64; 4. the first manifest in the list.
	// `PreserveOriginal` indicates that the original manifest will be preserved. For manifest lists,
	// the manifest list and all its sub-manifests will be imported. When empty, the behaviour will be
	// decided based on the payload type advertised by the ClusterVersion status, i.e single arch payload
	// implies the import mode is Legacy and multi payload implies PreserveOriginal.
	// +openshift:enable:FeatureGate=ImageStreamImportMode
	// +optional
	ImageStreamImportMode ImportModeType `json:"imageStreamImportMode"`
}

type ImageStatus struct {
	// internalRegistryHostname sets the hostname for the default internal image
	// registry. The value must be in "hostname[:port]" format.
	// This value is set by the image registry operator which controls the internal registry
	// hostname.
	// +optional
	InternalRegistryHostname string `json:"internalRegistryHostname,omitempty"`

	// externalRegistryHostnames provides the hostnames for the default external image
	// registry. The external hostname should be set only when the image registry
	// is exposed externally. The first value is used in 'publicDockerImageRepository'
	// field in ImageStreams. The value must be in "hostname[:port]" format.
	// +optional
	// +listType=atomic
	ExternalRegistryHostnames []string `json:"externalRegistryHostnames,omitempty"`

	// imageStreamImportMode controls the import mode behaviour of imagestreams. It can be
	// `Legacy` or `PreserveOriginal`. `Legacy` indicates that the legacy behaviour should be used.
	// For manifest lists, the legacy behaviour will discard the manifest list and import a single
	// sub-manifest. In this case, the platform is chosen in the following order of priority:
	// 1. tag annotations; 2. control plane arch/os; 3. linux/amd64; 4. the first manifest in the list.
	// `PreserveOriginal` indicates that the original manifest will be preserved. For manifest lists,
	// the manifest list and all its sub-manifests will be imported. This value will be reconciled based
	// on either the spec value or if no spec value is specified, the image registry operator would look
	// at the ClusterVersion status to determine the payload type and set the import mode accordingly,
	// i.e single arch payload implies the import mode is Legacy and multi payload implies PreserveOriginal.
	// +openshift:enable:FeatureGate=ImageStreamImportMode
	// +optional
	ImageStreamImportMode ImportModeType `json:"imageStreamImportMode,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type ImageList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []Image `json:"items"`
}

// RegistryLocation contains a location of the registry specified by the registry domain
// name. The domain name might include wildcards, like '*' or '??'.
type RegistryLocation struct {
	// domainName specifies a domain name for the registry
	// In case the registry use non-standard (80 or 443) port, the port should be included
	// in the domain name as well.
	DomainName string `json:"domainName"`
	// insecure indicates whether the registry is secure (https) or insecure (http)
	// By default (if not specified) the registry is assumed as secure.
	// +optional
	Insecure bool `json:"insecure,omitempty"`
}

// RegistrySources holds cluster-wide information about how to handle the registries config.
type RegistrySources struct {
	// insecureRegistries are registries which do not have a valid TLS certificates or only support HTTP connections.
	// +optional
	// +listType=atomic
	InsecureRegistries []string `json:"insecureRegistries,omitempty"`
	// blockedRegistries cannot be used for image pull and push actions. All other registries are permitted.
	//
	// Only one of BlockedRegistries or AllowedRegistries may be set.
	// +optional
	// +listType=atomic
	BlockedRegistries []string `json:"blockedRegistries,omitempty"`
	// allowedRegistries are the only registries permitted for image pull and push actions. All other registries are denied.
	//
	// Only one of BlockedRegistries or AllowedRegistries may be set.
	// +optional
	// +listType=atomic
	AllowedRegistries []string `json:"allowedRegistries,omitempty"`
	// containerRuntimeSearchRegistries are registries that will be searched when pulling images that do not have fully qualified
	// domains in their pull specs. Registries will be searched in the order provided in the list.
	// Note: this search list only works with the container runtime, i.e CRI-O. Will NOT work with builds or imagestream imports.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Format=hostname
	// +listType=set
	ContainerRuntimeSearchRegistries []string `json:"containerRuntimeSearchRegistries,omitempty"`
}
