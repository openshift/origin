package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

const BuildDefaultsPlugin = "BuildDefaults"

// BuildDefaultsConfig controls the default information for Builds
type BuildDefaultsConfig struct {
	unversioned.TypeMeta

	// gitHTTPProxy is the location of the HTTPProxy for Git source
	GitHTTPProxy string

	// gitHTTPSProxy is the location of the HTTPSProxy for Git source
	GitHTTPSProxy string

	// gitNoProxy is the list of domains for which the proxy should not be used
	GitNoProxy string

	// env is a set of default environment variables that will be applied to the
	// build if the specified variables do not exist on the build
	Env []kapi.EnvVar

	// sourceStrategyDefaults are default values that apply to builds using the
	// source strategy.
	SourceStrategyDefaults *SourceStrategyDefaultsConfig

	// imageLabels is a list of docker labels that are applied to the resulting image.
	// User can override a default label by providing a label with the same name in their
	// Build/BuildConfig.
	ImageLabels []buildapi.ImageLabel

	// nodeSelector is a selector which must be true for the build pod to fit on a node
	NodeSelector map[string]string

	// annotations are annotations that will be added to the build pod
	Annotations map[string]string
}

// SourceStrategyDefaultsConfig contains values that apply to builds using the
// source strategy.
type SourceStrategyDefaultsConfig struct {

	// Incremental indicates if s2i build strategies should perform an incremental
	// build or not
	Incremental *bool
}
