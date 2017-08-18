package controller

import (
	"fmt"
	"net"
	"time"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/service/controller/ingressip"
)

type IngressIPControllerConfig struct {
	IngressIPNetworkCIDR string
	IngressIPSyncPeriod  time.Duration
}

func (c *IngressIPControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	if len(c.IngressIPNetworkCIDR) == 0 {
		return true, nil
	}

	_, ipNet, err := net.ParseCIDR(c.IngressIPNetworkCIDR)
	if err != nil {
		return false, fmt.Errorf("unable to start ingress IP controller: %v", err)
	}

	if ipNet.IP.IsUnspecified() {
		// TODO: Is this an error?
		return true, nil
	}

	ingressIPController := ingressip.NewIngressIPController(
		ctx.ExternalKubeInformers.Core().V1().Services().Informer(),
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraServiceIngressIPControllerServiceAccountName),
		ipNet,
		c.IngressIPSyncPeriod,
	)
	go ingressIPController.Run(ctx.Stop)

	return true, nil
}
