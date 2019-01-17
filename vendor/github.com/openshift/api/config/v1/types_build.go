package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Build holds cluster-wide information on how to handle builds. The canonical name is `cluster`
type Build struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec holds user-settable values for the build controller configuration
	// +optional
	Spec BuildSpec `json:"spec,omitempty"`
}

type BuildSpec struct {
	// AdditionalTrustedCA is a reference to a ConfigMap containing additional CAs that
	// should be trusted for image pushes and pulls during builds.
	// The namespace for this config map is openshift-config.
	// +optional
	AdditionalTrustedCA ConfigMapNameReference `json:"additionalTrustedCA,omitempty"`
	// BuildDefaults controls the default information for Builds
	// +optional
	BuildDefaults BuildDefaults `json:"buildDefaults,omitempty"`
	// BuildOverrides controls override settings for builds
	// +optional
	BuildOverrides BuildOverrides `json:"buildOverrides,omitempty"`
}

type BuildDefaults struct {
	// DefaultProxy contains the default proxy settings for all build operations, including image pull/push
	// and source download.
	//
	// Values can be overrode by setting the `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY` environment variables
	// in the build config's strategy.
	// +optional
	DefaultProxy *ProxySpec `json:"defaultProxy,omitempty"`

	// GitProxy contains the proxy settings for git operations only. If set, this will override
	// any Proxy settings for all git commands, such as git clone.
	//
	// Values that are not set here will be inherited from DefaultProxy.
	// +optional
	GitProxy *ProxySpec `json:"gitProxy,omitempty"`

	// Env is a set of default environment variables that will be applied to the
	// build if the specified variables do not exist on the build
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// ImageLabels is a list of docker labels that are applied to the resulting image.
	// User can override a default label by providing a label with the same name in their
	// Build/BuildConfig.
	// +optional
	ImageLabels []ImageLabel `json:"imageLabels,omitempty"`

	// Resources defines resource requirements to execute the build.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// RegistriesConfig controls the registries allowed for image pull and push.
	// +optional
	RegistriesConfig RegistriesConfig `json:"registriesConfig,omitempty"`
}

type ImageLabel struct {
	// Name defines the name of the label. It must have non-zero length.
	Name string `json:"name"`

	// Value defines the literal value of the label.
	// +optional
	Value string `json:"value,omitempty"`
}

type RegistriesConfig struct {
	// SearchRegistries lists the registries to search for images if an image repository is not specified in an image pull spec.
	//
	// If this is not set, builds will search Docker Hub (docker.io) when a repository is not specified.
	// Setting this to an empty list will require all builds to fully qualify their image pull specs.
	// +optional
	SearchRegistries *[]string `json:"searchRegistries,omitempty"`
	// InsecureRegistries are registries which do not have a valid SSL certificate or only support HTTP connections.
	// +optional
	InsecureRegistries []string `json:"insecureRegistries,omitempty"`
	// BlockedRegistries are blacklisted from image pull/push. All other registries are allowed.
	//
	// Only one of BlockedRegistries or AllowedRegistries may be set.
	// +optional
	BlockedRegistries []string `json:"blockedRegistries,omitempty"`
	// AllowedRegistries are whitelisted for image pull/push. All other registries are blocked.
	//
	// Only one of BlockedRegistries or AllowedRegistries may be set.
	// +optional
	AllowedRegistries []string `json:"allowedRegistries,omitempty"`
}

type BuildOverrides struct {
	// ImageLabels is a list of docker labels that are applied to the resulting image.
	// If user provided a label in their Build/BuildConfig with the same name as one in this
	// list, the user's label will be overwritten.
	// +optional
	ImageLabels []ImageLabel `json:"imageLabels,omitempty"`

	// NodeSelector is a selector which must be true for the build pod to fit on a node
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations is a list of Tolerations that will override any existing
	// tolerations set on a build pod.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type BuildList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Build `json:"items"`
}
