package controller

import (
	buildcontroller "github.com/openshift/origin/pkg/build/controller/build"
	builddefaults "github.com/openshift/origin/pkg/build/controller/build/defaults"
	buildoverrides "github.com/openshift/origin/pkg/build/controller/build/overrides"
	buildconfigcontroller "github.com/openshift/origin/pkg/build/controller/buildconfig"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

// RunController starts the build sync loop for builds and buildConfig processing.
func RunBuildController(ctx *ControllerContext) (bool, error) {
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = ctx.OpenshiftControllerConfig.Build.ImageTemplateFormat.Format
	imageTemplate.Latest = ctx.OpenshiftControllerConfig.Build.ImageTemplateFormat.Latest

	buildClient := ctx.ClientBuilder.OpenshiftBuildClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)
	externalKubeClient := ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)
	securityClient := ctx.ClientBuilder.OpenshiftSecurityClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)

	buildInformer := ctx.BuildInformers.Build().V1().Builds()
	buildConfigInformer := ctx.BuildInformers.Build().V1().BuildConfigs()
	imageStreamInformer := ctx.ImageInformers.Image().V1().ImageStreams()
	podInformer := ctx.KubernetesInformers.Core().V1().Pods()
	secretInformer := ctx.KubernetesInformers.Core().V1().Secrets()
	controllerConfigInformer := ctx.ConfigInformers.Config().V1().Builds()
	imageConfigInformer := ctx.ConfigInformers.Config().V1().Images()
	configMapInformer := ctx.OpenshiftConfigKubernetesInformers.Core().V1().ConfigMaps()

	buildControllerParams := &buildcontroller.BuildControllerParams{
		BuildInformer:                    buildInformer,
		BuildConfigInformer:              buildConfigInformer,
		BuildControllerConfigInformer:    controllerConfigInformer,
		ImageConfigInformer:              imageConfigInformer,
		ImageStreamInformer:              imageStreamInformer,
		PodInformer:                      podInformer,
		SecretInformer:                   secretInformer,
		OpenshiftConfigConfigMapInformer: configMapInformer,
		KubeClient:                       externalKubeClient,
		BuildClient:                      buildClient,
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image: imageTemplate.ExpandOrDie("docker-builder"),
		},
		SourceBuildStrategy: &buildstrategy.SourceBuildStrategy{
			Image:          imageTemplate.ExpandOrDie("docker-builder"),
			SecurityClient: securityClient.SecurityV1(),
		},
		CustomBuildStrategy:      &buildstrategy.CustomBuildStrategy{},
		BuildDefaults:            builddefaults.BuildDefaults{Config: ctx.OpenshiftControllerConfig.Build.BuildDefaults},
		BuildOverrides:           buildoverrides.BuildOverrides{Config: ctx.OpenshiftControllerConfig.Build.BuildOverrides},
		InternalRegistryHostname: ctx.OpenshiftControllerConfig.DockerPullSecret.InternalRegistryHostname,
	}

	go buildcontroller.NewBuildController(buildControllerParams).Run(5, ctx.Stop)
	return true, nil
}

func RunBuildConfigChangeController(ctx *ControllerContext) (bool, error) {
	clientName := bootstrappolicy.InfraBuildConfigChangeControllerServiceAccountName
	kubeExternalClient := ctx.ClientBuilder.ClientOrDie(clientName)
	buildClient := ctx.ClientBuilder.OpenshiftBuildClientOrDie(clientName)
	buildConfigInformer := ctx.BuildInformers.Build().V1().BuildConfigs()
	buildInformer := ctx.BuildInformers.Build().V1().Builds()

	controller := buildconfigcontroller.NewBuildConfigController(buildClient, kubeExternalClient, buildConfigInformer, buildInformer)
	go controller.Run(5, ctx.Stop)
	return true, nil
}
