// +build !linux

package controller

import (
	"fmt"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/network"
)

type SDNControllerConfig struct {
	NetworkConfig configapi.MasterNetworkConfig
}

func (c *SDNControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	if !network.IsOpenShiftNetworkPlugin(c.NetworkConfig.NetworkPluginName) {
		return false, nil
	}

	return false, fmt.Errorf("SDN not supported on this platform")
}
