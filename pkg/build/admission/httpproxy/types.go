package httpproxy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// DefaultConfig controls the default information for BuildConfigs
type DefaultConfig struct {
	unversioned.TypeMeta

	// HTTPProxy is the location of the HTTPProxy
	HTTPProxy string

	// HTTPSProxy is the location of the HTTPSProxy
	HTTPSProxy string

	// Env is a set of default environment variables that will be applied to the
	// build if the specified variables do not exist on the build.
	Env []kapi.EnvVar
}
