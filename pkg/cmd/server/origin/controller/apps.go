package controller

import (
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	deployercontroller "github.com/openshift/origin/pkg/deploy/controller/deployer"
	deployconfigcontroller "github.com/openshift/origin/pkg/deploy/controller/deploymentconfig"
	triggercontroller "github.com/openshift/origin/pkg/deploy/controller/generictrigger"
	templatecontroller "github.com/openshift/origin/pkg/template/controller"
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

type TemplateInstanceControllerConfig struct {
}

func (c *DeployerControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	internalDeployerKubeClient, err := ctx.ClientBuilder.KubeInternalClient(bootstrappolicy.InfraDeployerControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	go deployercontroller.NewDeployerController(
		ctx.DeprecatedOpenshiftInformers.InternalKubernetesInformers().Core().InternalVersion().ReplicationControllers(),
		ctx.DeprecatedOpenshiftInformers.InternalKubernetesInformers().Core().InternalVersion().Pods(),
		internalDeployerKubeClient,
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraDeployerControllerServiceAccountName),
		bootstrappolicy.DeployerServiceAccountName,
		c.ImageName,
		c.ClientEnvVars,
		c.Codec,
	).Run(5, ctx.Stop)

	return true, nil
}

func (c *DeploymentConfigControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraDeploymentConfigControllerServiceAccountName

	internalDcKubeClient, err := ctx.ClientBuilder.KubeInternalClient(saName)
	if err != nil {
		return true, err
	}
	deprecatedOcDcClient, err := ctx.ClientBuilder.DeprecatedOpenshiftClient(saName)
	if err != nil {
		return true, err
	}

	go deployconfigcontroller.NewDeploymentConfigController(
		ctx.AppInformers.Apps().InternalVersion().DeploymentConfigs().Informer(),
		ctx.DeprecatedOpenshiftInformers.InternalKubernetesInformers().Core().InternalVersion().ReplicationControllers(),
		deprecatedOcDcClient,
		internalDcKubeClient,
		ctx.ClientBuilder.ClientOrDie(saName),
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
		ctx.DeprecatedOpenshiftInformers.InternalKubernetesInformers().Core().InternalVersion().ReplicationControllers().Informer(),
		ctx.ImageInformers.Image().InternalVersion().ImageStreams().Informer(),
		deprecatedOcTriggerClient,
		c.Codec,
	).Run(5, ctx.Stop)

	return true, nil
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
