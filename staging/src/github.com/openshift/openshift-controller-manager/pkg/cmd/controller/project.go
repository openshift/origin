package controller

import (
	projectcontroller "github.com/openshift/openshift-controller-manager/pkg/project/controller"
)

func RunOriginNamespaceController(ctx *ControllerContext) (bool, error) {
	controller := projectcontroller.NewProjectFinalizerController(
		ctx.KubernetesInformers.Core().V1().Namespaces(),
		ctx.ClientBuilder.ClientOrDie(infraOriginNamespaceServiceAccountName),
	)
	go controller.Run(ctx.Stop, 5)
	return true, nil
}
