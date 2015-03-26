package origin

import (
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type AssetConfig struct {
	Options configapi.AssetConfig
}

func BuildAssetConfig(options configapi.AssetConfig) (*AssetConfig, error) {
	return &AssetConfig{options}, nil
}
