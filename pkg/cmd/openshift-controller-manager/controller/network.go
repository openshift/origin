package controller

import (
	"fmt"
	"net"
	"time"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/service/controller/ingressip"
)

func RunIngressIPController(ctx *ControllerContext) (bool, error) {
	// TODO configurable?
	resyncPeriod := 10 * time.Minute

	if len(ctx.OpenshiftControllerConfig.Ingress.IngressIPNetworkCIDR) == 0 {
		return true, nil
	}

	_, ipNet, err := net.ParseCIDR(ctx.OpenshiftControllerConfig.Ingress.IngressIPNetworkCIDR)
	if err != nil {
		return false, fmt.Errorf("unable to start ingress IP controller: %v", err)
	}

	if ipNet.IP.IsUnspecified() {
		// TODO: Is this an error?
		return true, nil
	}

	ingressIPController := ingressip.NewIngressIPController(
		ctx.KubernetesInformers.Core().V1().Services().Informer(),
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraServiceIngressIPControllerServiceAccountName),
		ipNet,
		resyncPeriod,
	)
	go ingressIPController.Run(ctx.Stop)

	return true, nil
}
