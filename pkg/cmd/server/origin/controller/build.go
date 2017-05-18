package controller

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	kubeadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"

	builddefaults "github.com/openshift/origin/pkg/build/admission/defaults"
	buildoverrides "github.com/openshift/origin/pkg/build/admission/overrides"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

type BuildControllerConfig struct {
	DockerImage           string
	STIImage              string
	AdmissionPluginConfig map[string]configapi.AdmissionPluginConfig

	Codec runtime.Codec
}

// RunBuildController starts the build sync loop for builds and buildConfig processing.
func (c *BuildControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	pluginInitializer := kubeadmission.NewPluginInitializer(
		ctx.ClientBuilder.KubeInternalClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName),
		ctx.DeprecatedOpenshiftInformers.InternalKubernetesInformers(),
		nil, // api authorizer, only used by PSP
		nil, // cloud config
	)
	admissionControl, err := admission.InitPlugin("SecurityContextConstraint", nil, pluginInitializer)
	if err != nil {
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

	deprecatedOpenshiftClient, err := ctx.ClientBuilder.DeprecatedOpenshiftClient(bootstrappolicy.InfraBuildControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	factory := buildcontrollerfactory.BuildControllerFactory{
		KubeClient:         ctx.ClientBuilder.KubeInternalClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName),
		ExternalKubeClient: ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraBuildControllerServiceAccountName),
		OSClient:           deprecatedOpenshiftClient,
		BuildUpdater:       buildclient.NewOSClientBuildClient(deprecatedOpenshiftClient),
		BuildLister:        buildclient.NewOSClientBuildClient(deprecatedOpenshiftClient),
		BuildConfigGetter:  buildclient.NewOSClientBuildConfigClient(deprecatedOpenshiftClient),
		BuildDeleter:       buildclient.NewBuildDeleter(deprecatedOpenshiftClient),
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image: c.DockerImage,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: c.Codec,
		},
		SourceBuildStrategy: &buildstrategy.SourceBuildStrategy{
			Image: c.STIImage,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec:            c.Codec,
			AdmissionControl: admissionControl,
		},
		CustomBuildStrategy: &buildstrategy.CustomBuildStrategy{
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: c.Codec,
		},
		BuildDefaults:  buildDefaults,
		BuildOverrides: buildOverrides,
	}

	controller := factory.Create()
	controller.Run()
	deleteController := factory.CreateDeleteController()
	deleteController.Run()
	return true, nil
}
