package controller

import (
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templatecontroller "github.com/openshift/origin/pkg/template/controller"
)

type TemplateInstanceControllerConfig struct {
}

func (c *TemplateInstanceControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraTemplateInstanceControllerServiceAccountName

	internalKubeClient, err := ctx.ClientBuilder.KubeInternalClient(saName)
	if err != nil {
		return true, err
	}

	deprecatedOcClient, err := ctx.ClientBuilder.DeprecatedOpenshiftClient(saName)
	if err != nil {
		return true, err
	}

	templateClient, err := ctx.ClientBuilder.OpenshiftTemplateClient(saName)
	if err != nil {
		return true, err
	}

	go templatecontroller.NewTemplateInstanceController(
		deprecatedOcClient,
		internalKubeClient,
		templateClient.Template(),
		ctx.TemplateInformers.Template().InternalVersion().TemplateInstances(),
	).Run(5, ctx.Stop)

	return true, nil
}
