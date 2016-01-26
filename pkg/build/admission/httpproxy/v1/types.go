package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// ProxyConfig controls the proxy information for BuildConfigs
type ProxyConfig struct {
	unversioned.TypeMeta

	// HTTPProxy is the location of the HTTPProxy
	HTTPProxy string `json:"httpProxy",description:"location of the global http proxy"`

	// HTTPSProxy is the location of the HTTPSProxy
	HTTPSProxy string `json:"httpsProxy",description:"location of the global https proxy"`
}
