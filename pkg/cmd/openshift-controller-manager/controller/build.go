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
func RunBuildController(ctx ControllerContext) (bool, error) {
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = ctx.OpenshiftControllerConfig.Build.ImageTemplateFormat.Format
	imageTemplate.Latest = ctx.OpenshiftControllerConfig.Build.ImageTemplateFormat.Latest

	buildDefaults, err := builddefaults.NewBuildDefaults(ctx.OpenshiftControllerConfig.Build.AdmissionPluginConfig)
	if err != nil {
		return true, err
	}
	buildOverrides, err := buildoverrides.NewBuildOverrides(ctx.OpenshiftControllerConfig.Build.AdmissionPluginConfig)
	if err != nil {
		return true, err
	}

	kubeClient := ctx.ClientBuilder.KubeInternalClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)
	buildClient := ctx.ClientBuilder.OpenshiftInternalBuildClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)
	externalKubeClient := ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)
	securityClient := ctx.ClientBuilder.OpenshiftInternalSecurityClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)

	buildInformer := ctx.InternalBuildInformers.Build().InternalVersion().Builds()
	buildConfigInformer := ctx.InternalBuildInformers.Build().InternalVersion().BuildConfigs()
	imageStreamInformer := ctx.InternalImageInformers.Image().InternalVersion().ImageStreams()
	podInformer := ctx.KubernetesInformers.Core().V1().Pods()
	secretInformer := ctx.KubernetesInformers.Core().V1().Secrets()

	buildControllerParams := &buildcontroller.BuildControllerParams{
		BuildInformer:       buildInformer,
		BuildConfigInformer: buildConfigInformer,
		ImageStreamInformer: imageStreamInformer,
		PodInformer:         podInformer,
		SecretInformer:      secretInformer,
		KubeClientInternal:  kubeClient,
		KubeClientExternal:  externalKubeClient,
		BuildClientInternal: buildClient,
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image: imageTemplate.ExpandOrDie("docker-builder"),
		},
		SourceBuildStrategy: &buildstrategy.SourceBuildStrategy{
			Image:          imageTemplate.ExpandOrDie("docker-builder"),
			SecurityClient: securityClient.Security(),
		},
		CustomBuildStrategy: &buildstrategy.CustomBuildStrategy{},
		BuildDefaults:       buildDefaults,
		BuildOverrides:      buildOverrides,
	}

	go buildcontroller.NewBuildController(buildControllerParams).Run(5, ctx.Stop)
	return true, nil
}

func RunBuildConfigChangeController(ctx ControllerContext) (bool, error) {
	clientName := bootstrappolicy.InfraBuildConfigChangeControllerServiceAccountName
	kubeExternalClient := ctx.ClientBuilder.ClientOrDie(clientName)
	buildClient := ctx.ClientBuilder.OpenshiftInternalBuildClientOrDie(clientName)
	buildConfigInformer := ctx.InternalBuildInformers.Build().InternalVersion().BuildConfigs()
	buildInformer := ctx.InternalBuildInformers.Build().InternalVersion().Builds()

	controller := buildconfigcontroller.NewBuildConfigController(buildClient, kubeExternalClient, buildConfigInformer, buildInformer)
	go controller.Run(5, ctx.Stop)
	return true, nil
}
