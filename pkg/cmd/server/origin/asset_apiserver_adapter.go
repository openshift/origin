package origin

import (
	"os"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	override "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
)

// All this ugly was removed in 3.9
// GetResourceOverrideConfig looks in two potential places where ClusterResourceOverrideConfig can be specified
func GetResourceOverrideConfig(masterConfig configapi.MasterConfig) (*overrideapi.ClusterResourceOverrideConfig, error) {
	overrideConfig, err := checkForOverrideConfig(masterConfig.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	if overrideConfig != nil {
		return overrideConfig, nil
	}
	if masterConfig.KubernetesMasterConfig == nil { // external kube gets you a nil pointer here
		return nil, nil
	}
	overrideConfig, err = checkForOverrideConfig(masterConfig.KubernetesMasterConfig.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	return overrideConfig, nil
}

// checkForOverrideConfig looks for ClusterResourceOverrideConfig plugin cfg in the admission PluginConfig
func checkForOverrideConfig(ac configapi.AdmissionConfig) (*overrideapi.ClusterResourceOverrideConfig, error) {
	overridePluginConfigFile, err := pluginconfig.GetPluginConfigFile(ac.PluginConfig, overrideapi.PluginName, "")
	if err != nil {
		return nil, err
	}
	if overridePluginConfigFile == "" {
		return nil, nil
	}
	configFile, err := os.Open(overridePluginConfigFile)
	if err != nil {
		return nil, err
	}
	overrideConfig, err := override.ReadConfig(configFile)
	if err != nil {
		return nil, err
	}
	return overrideConfig, nil
}
