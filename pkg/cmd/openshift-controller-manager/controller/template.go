package controller

import (
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templatecontroller "github.com/openshift/origin/pkg/template/controller"
)

func RunTemplateInstanceController(ctx ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraTemplateInstanceControllerServiceAccountName

	restConfig, err := ctx.ClientBuilder.Config(saName)
	if err != nil {
		return true, err
	}

	go templatecontroller.NewTemplateInstanceController(
		ctx.DynamicRestMapper,
		restConfig,
		ctx.ClientBuilder.KubeInternalClientOrDie(saName),
		ctx.ClientBuilder.OpenshiftInternalBuildClientOrDie(saName),
		ctx.ClientBuilder.OpenshiftInternalTemplateClientOrDie(saName),
		ctx.TemplateInformers.Template().InternalVersion().TemplateInstances(),
	).Run(5, ctx.Stop)

	return true, nil
}

func RunTemplateInstanceFinalizerController(ctx ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraTemplateInstanceFinalizerControllerServiceAccountName

	restConfig, err := ctx.ClientBuilder.Config(saName)
	if err != nil {
		return true, err
	}

	go templatecontroller.NewTemplateInstanceFinalizerController(
		ctx.DynamicRestMapper,
		restConfig,
		ctx.ClientBuilder.OpenshiftInternalTemplateClientOrDie(saName),
		ctx.TemplateInformers.Template().InternalVersion().TemplateInstances(),
	).Run(5, ctx.Stop)

	return true, nil
}
