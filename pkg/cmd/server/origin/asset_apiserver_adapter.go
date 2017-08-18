package origin

import (
	"os"

	assetapiserver "github.com/openshift/origin/pkg/assets/apiserver"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride"
	clusterresourceoverrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
)

// TODO this is taking a very large config for a small piece of it.  The information must be broken up at some point so that
// we can run this in a pod.  This is an indication of leaky abstraction because it spent too much time in openshift start
func NewAssetServerConfigFromMasterConfig(masterConfigOptions configapi.MasterConfig) (*assetapiserver.AssetServerConfig, error) {
	ret, err := assetapiserver.NewAssetServerConfig(*masterConfigOptions.AssetConfig)
	if err != nil {
		return nil, err
	}

	// this fix up indicates that our assetConfig lacks sufficient information
	ret.GenericConfig.CorsAllowedOriginList = masterConfigOptions.CORSAllowedOrigins
	ret.LimitRequestOverrides, err = getResourceOverrideConfig(masterConfigOptions)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// getResourceOverrideConfig looks in two potential places where ClusterResourceOverrideConfig can be specified
func getResourceOverrideConfig(masterConfigOptions configapi.MasterConfig) (*clusterresourceoverrideapi.ClusterResourceOverrideConfig, error) {
	overrideConfig, err := checkForOverrideConfig(masterConfigOptions.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	if overrideConfig != nil {
		return overrideConfig, nil
	}
	if masterConfigOptions.KubernetesMasterConfig == nil { // external kube gets you a nil pointer here
		return nil, nil
	}
	overrideConfig, err = checkForOverrideConfig(masterConfigOptions.KubernetesMasterConfig.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	return overrideConfig, nil
}

// checkForOverrideConfig looks for ClusterResourceOverrideConfig plugin cfg in the admission PluginConfig
func checkForOverrideConfig(ac configapi.AdmissionConfig) (*clusterresourceoverrideapi.ClusterResourceOverrideConfig, error) {
	overridePluginConfigFile, err := pluginconfig.GetPluginConfigFile(ac.PluginConfig, clusterresourceoverrideapi.PluginName, "")
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
	overrideConfig, err := clusterresourceoverride.ReadConfig(configFile)
	if err != nil {
		return nil, err
	}
	return overrideConfig, nil
}
