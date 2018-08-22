package controller

import (
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templatecontroller "github.com/openshift/origin/pkg/template/controller"
	"k8s.io/client-go/dynamic"
)

func RunTemplateInstanceController(ctx *ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraTemplateInstanceControllerServiceAccountName

	restConfig, err := ctx.ClientBuilder.Config(saName)
	if err != nil {
		return true, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return true, err
	}

	go templatecontroller.NewTemplateInstanceController(
		ctx.RestMapper,
		dynamicClient,
		ctx.ClientBuilder.ClientGoClientOrDie(saName).AuthorizationV1(),
		ctx.ClientBuilder.ClientGoClientOrDie(saName),
		ctx.ClientBuilder.OpenshiftBuildClientOrDie(saName),
		ctx.ClientBuilder.OpenshiftTemplateClientOrDie(saName).TemplateV1(),
		ctx.InternalTemplateInformers.Template().V1().TemplateInstances(),
	).Run(5, ctx.Stop)

	return true, nil
}

func RunTemplateInstanceFinalizerController(ctx *ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraTemplateInstanceFinalizerControllerServiceAccountName

	restConfig, err := ctx.ClientBuilder.Config(saName)
	if err != nil {
		return true, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return true, err
	}

	go templatecontroller.NewTemplateInstanceFinalizerController(
		ctx.RestMapper,
		dynamicClient,
		ctx.ClientBuilder.OpenshiftTemplateClientOrDie(saName),
		ctx.InternalTemplateInformers.Template().V1().TemplateInstances(),
	).Run(5, ctx.Stop)

	return true, nil
}
