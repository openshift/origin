package controller

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	deployercontroller "github.com/openshift/origin/pkg/apps/controller/deployer"
	deployconfigcontroller "github.com/openshift/origin/pkg/apps/controller/deploymentconfig"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

func RunDeployerController(ctx ControllerContext) (bool, error) {
	clientConfig, err := ctx.ClientBuilder.Config(bootstrappolicy.InfraDeployerControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return true, err
	}

	groupVersion := schema.GroupVersion{Group: "", Version: "v1"}
	annotationCodec := legacyscheme.Codecs.LegacyCodec(groupVersion)

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = ctx.OpenshiftControllerConfig.Deployer.ImageTemplateFormat.Format
	imageTemplate.Latest = ctx.OpenshiftControllerConfig.Deployer.ImageTemplateFormat.Latest

	go deployercontroller.NewDeployerController(
		ctx.ExternalKubeInformers.Core().V1().ReplicationControllers(),
		ctx.ExternalKubeInformers.Core().V1().Pods(),
		kubeClient,
		bootstrappolicy.DeployerServiceAccountName,
		imageTemplate.ExpandOrDie("deployer"),
		nil,
		annotationCodec,
	).Run(5, ctx.Stop)

	return true, nil
}

func RunDeploymentConfigController(ctx ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraDeploymentConfigControllerServiceAccountName

	kubeClient, err := ctx.ClientBuilder.Client(saName)
	if err != nil {
		return true, err
	}

	groupVersion := schema.GroupVersion{Group: "", Version: "v1"}
	annotationCodec := legacyscheme.Codecs.LegacyCodec(groupVersion)

	go deployconfigcontroller.NewDeploymentConfigController(
		ctx.AppInformers.Apps().InternalVersion().DeploymentConfigs(),
		ctx.ExternalKubeInformers.Core().V1().ReplicationControllers(),
		ctx.ClientBuilder.OpenshiftInternalAppsClientOrDie(saName),
		kubeClient,
		annotationCodec,
	).Run(5, ctx.Stop)

	return true, nil
}
