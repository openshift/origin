package controller

import (
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	deployercontroller "github.com/openshift/origin/pkg/deploy/controller/deployer"
	deployconfigcontroller "github.com/openshift/origin/pkg/deploy/controller/deploymentconfig"
	triggercontroller "github.com/openshift/origin/pkg/deploy/controller/generictrigger"
)

type DeployerControllerConfig struct {
	ImageName     string
	ClientEnvVars []kapi.EnvVar

	Codec runtime.Codec
}

type DeploymentConfigControllerConfig struct {
	Codec runtime.Codec
}

type DeploymentTriggerControllerConfig struct {
	Codec runtime.Codec
}

func (c *DeployerControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	kubeClient, err := ctx.ClientBuilder.Client(bootstrappolicy.InfraDeployerControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	go deployercontroller.NewDeployerController(
		ctx.ExternalKubeInformers.Core().V1().ReplicationControllers(),
		ctx.ExternalKubeInformers.Core().V1().Pods(),
		kubeClient,
		bootstrappolicy.DeployerServiceAccountName,
		c.ImageName,
		c.ClientEnvVars,
		c.Codec,
	).Run(5, ctx.Stop)

	return true, nil
}

func (c *DeploymentConfigControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraDeploymentConfigControllerServiceAccountName

	kubeClient, err := ctx.ClientBuilder.Client(saName)
	if err != nil {
		return true, err
	}
	deprecatedOcDcClient, err := ctx.ClientBuilder.DeprecatedOpenshiftClient(saName)
	if err != nil {
		return true, err
	}

	go deployconfigcontroller.NewDeploymentConfigController(
		ctx.AppInformers.Apps().InternalVersion().DeploymentConfigs().Informer(),
		ctx.ExternalKubeInformers.Core().V1().ReplicationControllers(),
		deprecatedOcDcClient,
		kubeClient,
		c.Codec,
	).Run(5, ctx.Stop)

	return true, nil
}

func (c *DeploymentTriggerControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraDeploymentTriggerControllerServiceAccountName

	deprecatedOcTriggerClient, err := ctx.ClientBuilder.DeprecatedOpenshiftClient(saName)
	if err != nil {
		return true, err
	}

	go triggercontroller.NewDeploymentTriggerController(
		ctx.AppInformers.Apps().InternalVersion().DeploymentConfigs().Informer(),
		ctx.ExternalKubeInformers.Core().V1().ReplicationControllers().Informer(),
		ctx.ImageInformers.Image().InternalVersion().ImageStreams().Informer(),
		deprecatedOcTriggerClient,
		c.Codec,
	).Run(5, ctx.Stop)

	return true, nil
}
