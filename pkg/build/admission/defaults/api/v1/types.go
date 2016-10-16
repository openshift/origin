package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	buildapi "github.com/openshift/origin/pkg/build/api/v1"
)

// BuildDefaultsConfig controls the default information for Builds
type BuildDefaultsConfig struct {
	unversioned.TypeMeta `json:",inline"`

	// GitHTTPProxy is the location of the HTTPProxy for Git source
	GitHTTPProxy string `json:"gitHTTPProxy,omitempty"`

	// GitHTTPSProxy is the location of the HTTPSProxy for Git source
	GitHTTPSProxy string `json:"gitHTTPSProxy,omitempty"`

	// GitNoProxy is the list of domains for which the proxy should not be used
	GitNoProxy string `json:"gitNoProxy,omitempty"`

	//SourceSecret sets default build source secret if none is set. For corporate env where everything is secure
	SourceSecret string `json:"sourceSecret,omitempty"`

	// Env is a set of default environment variables that will be applied to the
	// build if the specified variables do not exist on the build
	Env []kapi.EnvVar `json:"env,omitempty"`

	// SourceStrategyDefaults are default values that apply to builds using the
	// source strategy.
	SourceStrategyDefaults *SourceStrategyDefaultsConfig `json:"sourceStrategyDefaults,omitempty"`

	// ImageLabels is a list of docker labels that are applied to the resulting image.
	// User can override a default label by providing a label with the same name in their
	// Build/BuildConfig.
	ImageLabels []buildapi.ImageLabel `json:"imageLabels,omitempty"`
}

// SourceStrategyDefaultsConfig contains values that apply to builds using the
// source strategy.
type SourceStrategyDefaultsConfig struct {

	// Incremental indicates if s2i build strategies should perform an incremental
	// build or not
	Incremental *bool `json:"incremental,omitempty"`
}
