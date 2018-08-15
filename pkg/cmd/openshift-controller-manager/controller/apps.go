package controller

import (
	"k8s.io/client-go/kubernetes"

	deployercontroller "github.com/openshift/origin/pkg/apps/controller/deployer"
	deployconfigcontroller "github.com/openshift/origin/pkg/apps/controller/deploymentconfig"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

func RunDeployerController(ctx *ControllerContext) (bool, error) {
	clientConfig, err := ctx.ClientBuilder.Config(bootstrappolicy.InfraDeployerControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return true, err
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = ctx.OpenshiftControllerConfig.Deployer.ImageTemplateFormat.Format
	imageTemplate.Latest = ctx.OpenshiftControllerConfig.Deployer.ImageTemplateFormat.Latest

	go deployercontroller.NewDeployerController(
		ctx.KubernetesInformers.Core().V1().ReplicationControllers(),
		ctx.KubernetesInformers.Core().V1().Pods(),
		kubeClient,
		bootstrappolicy.DeployerServiceAccountName,
		imageTemplate.ExpandOrDie("deployer"),
		nil,
	).Run(5, ctx.Stop)

	return true, nil
}

func RunDeploymentConfigController(ctx *ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraDeploymentConfigControllerServiceAccountName

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
