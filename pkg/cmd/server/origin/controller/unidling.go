package controller

import (
	"time"

	appsv1client "github.com/openshift/origin/pkg/apps/client/v1"
	appstypedclient "github.com/openshift/origin/pkg/apps/generated/clientset/typed/apps/v1"
	deployclient "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	unidlingcontroller "github.com/openshift/origin/pkg/unidling/controller"
)

type UnidlingControllerConfig struct {
	ResyncPeriod time.Duration
}

func (c *UnidlingControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	clientConfig := ctx.ClientBuilder.ConfigOrDie(bootstrappolicy.InfraUnidlingControllerServiceAccountName)
	appsClient, err := appstypedclient.NewForConfig(clientConfig)
	if err != nil {
		return false, err
	}

	scaleNamespacer := appsv1client.NewDelegatingScaleNamespacer(appsClient,
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraUnidlingControllerServiceAccountName).ExtensionsV1beta1())

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
