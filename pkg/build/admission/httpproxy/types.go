package httpproxy

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// ProxyConfig controls the proxy information for BuildConfigs
type ProxyConfig struct {
	unversioned.TypeMeta

	// HTTPProxy is the location of the HTTPProxy
	HTTPProxy string

	// HTTPSProxy is the location of the HTTPSProxy
	HTTPSProxy string
}
