package controller

import (
	"time"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	deployclient "github.com/openshift/origin/pkg/deploy/generated/internalclientset/typed/apps/internalversion"
	unidlingcontroller "github.com/openshift/origin/pkg/unidling/controller"
)

type UnidlingControllerConfig struct {
	ResyncPeriod time.Duration
}

func (c *UnidlingControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	scaleNamespacer := osclient.NewDelegatingScaleNamespacer(
		ctx.ClientBuilder.DeprecatedOpenshiftClientOrDie(bootstrappolicy.InfraUnidlingControllerServiceAccountName),
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraUnidlingControllerServiceAccountName).Extensions(),
	)
	coreClient := ctx.ClientBuilder.KubeInternalClientOrDie(bootstrappolicy.InfraUnidlingControllerServiceAccountName).Core()
	controller := unidlingcontroller.NewUnidlingController(
		scaleNamespacer,
		coreClient,
		coreClient,
		deployclient.NewForConfigOrDie(ctx.ClientBuilder.ConfigOrDie(bootstrappolicy.InfraUnidlingControllerServiceAccountName)),
		coreClient,
		c.ResyncPeriod,
	)

	go controller.Run(ctx.Stop)

	return true, nil
}
