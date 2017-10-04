package origin

import (
	assetapiserver "github.com/openshift/origin/pkg/assets/apiserver"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
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
	if err != nil {
		return nil, err
	}

	return ret, nil
}
