package controller

import (
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	projectcontroller "github.com/openshift/origin/pkg/project/controller"
)

func RunOriginNamespaceController(ctx ControllerContext) (bool, error) {
	controller := projectcontroller.NewProjectFinalizerController(
		ctx.ExternalKubeInformers.Core().V1().Namespaces(),
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraOriginNamespaceServiceAccountName),
	)
	go controller.Run(ctx.Stop, 5)
	return true, nil
}
