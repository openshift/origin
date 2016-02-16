package origin

import (
	oapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
)

// AssetConfig defines the required parameters for starting the OpenShift master
type AssetConfig struct {
	Options               oapi.AssetConfig
	LimitRequestOverrides *api.ClusterResourceOverrideConfig
}

// NewAssetConfig returns a new AssetConfig
func NewAssetConfig(options oapi.AssetConfig, limitRequestOverrides *api.ClusterResourceOverrideConfig) (*AssetConfig, error) {
	return &AssetConfig{options, limitRequestOverrides}, nil
}
