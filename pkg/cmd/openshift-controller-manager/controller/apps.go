package controller

import (
	"k8s.io/client-go/kubernetes"

	deployercontroller "github.com/openshift/openshift-controller-manager/pkg/apps/deployer"
	deployconfigcontroller "github.com/openshift/openshift-controller-manager/pkg/apps/deploymentconfig"
	"github.com/openshift/openshift-controller-manager/pkg/cmd/imageformat"
)

func RunDeployerController(ctx *ControllerContext) (bool, error) {
	clientConfig, err := ctx.ClientBuilder.Config(infraDeployerControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return true, err
	}

	imageTemplate := imageformat.NewDefaultImageTemplate()
	imageTemplate.Format = ctx.OpenshiftControllerConfig.Deployer.ImageTemplateFormat.Format
	imageTemplate.Latest = ctx.OpenshiftControllerConfig.Deployer.ImageTemplateFormat.Latest

	go deployercontroller.NewDeployerController(
		ctx.KubernetesInformers.Core().V1().ReplicationControllers(),
		ctx.KubernetesInformers.Core().V1().Pods(),
		kubeClient,
		deployerServiceAccountName,
		imageTemplate.ExpandOrDie("deployer"),
		nil,
	).Run(5, ctx.Stop)

	return true, nil
}

func RunDeploymentConfigController(ctx *ControllerContext) (bool, error) {
	saName := infraDeploymentConfigControllerServiceAccountName

	kubeClient, err := ctx.ClientBuilder.Client(saName)
	if err != nil {
		return true, err
	}

	go deployconfigcontroller.NewDeploymentConfigController(
		ctx.AppsInformers.Apps().V1().DeploymentConfigs(),
		ctx.KubernetesInformers.Core().V1().ReplicationControllers(),
		ctx.ClientBuilder.OpenshiftAppsClientOrDie(saName),
		kubeClient,
	).Run(5, ctx.Stop)

	return true, nil
}
