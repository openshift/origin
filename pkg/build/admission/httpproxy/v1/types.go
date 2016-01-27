package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// ProxyConfig controls the proxy information for BuildConfigs
type DefaultConfig struct {
	unversioned.TypeMeta

	// HTTPProxy is the location of the HTTPProxy
	HTTPProxy string `json:"httpProxy",description:"location of the global http proxy"`

	// HTTPSProxy is the location of the HTTPSProxy
	HTTPSProxy string `json:"httpsProxy",description:"location of the global https proxy"`

	// Env is a set of default environment variables that will be applied to the
	// build if the specified variables do not exist on the build.
	Env []kapi.EnvVar `json:"env",description:"default environment variable values to add to builds"`

	/*
		admissionConfig:
		  pluginConfig:
		    "BuildDefaulter":
		      configuration:
		        kind: "DefaultConfig"
		        apiVersion: "v1"
		        httpProxy: "defaultproxy"
		        httpsProxy: "defaultsslproxy"
		        env:
		        - name: "env1"
		          value: "value1"
	*/

}
