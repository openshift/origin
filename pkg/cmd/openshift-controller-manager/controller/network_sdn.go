package controller

import (
	"fmt"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/network"
	sdnmaster "github.com/openshift/origin/pkg/network/master"
)

func RunSDNController(ctx *ControllerContext) (bool, error) {
	if !network.IsOpenShiftNetworkPlugin(ctx.OpenshiftControllerConfig.Network.NetworkPluginName) {
		return false, nil
	}

	if err := sdnmaster.Start(
		ctx.OpenshiftControllerConfig.Network,
		ctx.ClientBuilder.OpenshiftNetworkClientOrDie(bootstrappolicy.InfraSDNControllerServiceAccountName),
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraSDNControllerServiceAccountName),
		ctx.KubernetesInformers,
		ctx.NetworkInformers,
	); err != nil {
		return false, fmt.Errorf("failed to start SDN plugin controller: %v", err)
	}

	return true, nil
}
