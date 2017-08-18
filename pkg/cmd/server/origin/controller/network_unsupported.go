// +build !linux

package controller

import (
	"fmt"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

type SDNControllerConfig struct {
	NetworkConfig configapi.MasterNetworkConfig
}

func (c *SDNControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	return false, fmt.Errorf("SDN not supported on this platform")
}
