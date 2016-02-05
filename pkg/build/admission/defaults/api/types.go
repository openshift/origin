package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// BuildDefaultsConfig controls the default information for Builds
type BuildDefaultsConfig struct {
	unversioned.TypeMeta

	// GitHTTPProxy is the location of the HTTPProxy for Git source
	GitHTTPProxy string

	// GitHTTPSProxy is the location of the HTTPSProxy for Git source
	GitHTTPSProxy string

	// Env is a set of default environment variables that will be applied to the
	// build if the specified variables do not exist on the build
	Env []kapi.EnvVar
}
