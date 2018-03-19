package controller

import (
	"fmt"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/network"
	sdnmaster "github.com/openshift/origin/pkg/network/master"
)

type SDNControllerConfig struct {
	NetworkConfig configapi.MasterNetworkConfig
}

func (c *SDNControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	if !network.IsOpenShiftNetworkPlugin(c.NetworkConfig.NetworkPluginName) {
		return false, nil
	}

	if err := sdnmaster.Start(
		c.NetworkConfig,
		ctx.ClientBuilder.OpenshiftInternalNetworkClientOrDie(bootstrappolicy.InfraSDNControllerServiceAccountName),
		ctx.ClientBuilder.KubeInternalClientOrDie(bootstrappolicy.InfraSDNControllerServiceAccountName),
		ctx.InternalKubeInformers,
		ctx.NetworkInformers,
	); err != nil {
		return false, fmt.Errorf("failed to start SDN plugin controller: %v", err)
	}

	return true, nil
}
