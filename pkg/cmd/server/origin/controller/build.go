package controller

import (
	"k8s.io/apimachinery/pkg/runtime"

	buildcontroller "github.com/openshift/origin/pkg/build/controller/build"
	builddefaults "github.com/openshift/origin/pkg/build/controller/build/defaults"
	buildoverrides "github.com/openshift/origin/pkg/build/controller/build/overrides"
	buildconfigcontroller "github.com/openshift/origin/pkg/build/controller/buildconfig"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	sccadmission "github.com/openshift/origin/pkg/security/admission"
)

type BuildControllerConfig struct {
	DockerImage           string
	S2IImage              string
	AdmissionPluginConfig map[string]*configapi.AdmissionPluginConfig

	Codec runtime.Codec
}

// RunController starts the build sync loop for builds and buildConfig processing.
func (c *BuildControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	sccAdmission := sccadmission.NewConstraint()
	sccAdmission.SetSecurityInformers(ctx.SecurityInformers)
	sccAdmission.SetInternalKubeClientSet(ctx.ClientBuilder.KubeInternalClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName))
	if err := sccAdmission.ValidateInitialization(); err != nil {
		return true, err
	}

	buildDefaults, err := builddefaults.NewBuildDefaults(c.AdmissionPluginConfig)
	if err != nil {
		return true, err
	}
	buildOverrides, err := buildoverrides.NewBuildOverrides(c.AdmissionPluginConfig)
	if err != nil {
		return true, err
	}

	kubeClient := ctx.ClientBuilder.KubeInternalClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)
	buildClient := ctx.ClientBuilder.OpenshiftInternalBuildClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)
	externalKubeClient := ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)
	securityClient := ctx.ClientBuilder.OpenshiftInternalSecurityClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName)

	buildInformer := ctx.BuildInformers.Build().InternalVersion().Builds()
	buildConfigInformer := ctx.BuildInformers.Build().InternalVersion().BuildConfigs()
	imageStreamInformer := ctx.ImageInformers.Image().InternalVersion().ImageStreams()
	podInformer := ctx.ExternalKubeInformers.Core().V1().Pods()
	secretInformer := ctx.ExternalKubeInformers.Core().V1().Secrets()

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
			Image: c.DockerImage,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: c.Codec,
		},
		SourceBuildStrategy: &buildstrategy.SourceBuildStrategy{
			Image: c.S2IImage,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec:          c.Codec,
			SecurityClient: securityClient.Security(),
		},
		CustomBuildStrategy: &buildstrategy.CustomBuildStrategy{
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: c.Codec,
		},
		BuildDefaults:  buildDefaults,
		BuildOverrides: buildOverrides,
	}

	go buildcontroller.NewBuildController(buildControllerParams).Run(5, ctx.Stop)
	return true, nil
}

func RunBuildConfigChangeController(ctx ControllerContext) (bool, error) {
	clientName := bootstrappolicy.InfraBuildConfigChangeControllerServiceAccountName
	kubeExternalClient := ctx.ClientBuilder.ClientOrDie(clientName)
	buildClient := ctx.ClientBuilder.OpenshiftInternalBuildClientOrDie(clientName)
	buildConfigInformer := ctx.BuildInformers.Build().InternalVersion().BuildConfigs()
	buildInformer := ctx.BuildInformers.Build().InternalVersion().Builds()

	controller := buildconfigcontroller.NewBuildConfigController(buildClient, kubeExternalClient, buildConfigInformer, buildInformer)
	go controller.Run(5, ctx.Stop)
	return true, nil
}
