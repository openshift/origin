package controller

import (
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	buildclient "github.com/openshift/client-go/build/clientset/versioned"

	builddefaults "github.com/openshift/openshift-controller-manager/pkg/build/controller/build/defaults"
	buildoverrides "github.com/openshift/openshift-controller-manager/pkg/build/controller/build/overrides"
	buildconfigcontroller "github.com/openshift/openshift-controller-manager/pkg/build/controller/buildconfig"
	buildstrategy "github.com/openshift/openshift-controller-manager/pkg/build/controller/strategy"
	"github.com/openshift/openshift-controller-manager/pkg/cmd/imageformat"
	buildcontroller "github.com/openshift/origin/pkg/build/controller/build"
)

// RunController starts the build sync loop for builds and buildConfig processing.
func RunBuildController(ctx *ControllerContext) (bool, error) {

	imageTemplate := imageformat.NewDefaultImageTemplate()
	imageTemplate.Format = ctx.OpenshiftControllerConfig.Build.ImageTemplateFormat.Format
	imageTemplate.Latest = ctx.OpenshiftControllerConfig.Build.ImageTemplateFormat.Latest

	cfg := ctx.ClientBuilder.ConfigOrDie(infraBuildControllerServiceAccountName)
	cfg.QPS = cfg.QPS * 2
	cfg.Burst = cfg.Burst * 2

	buildClient, err := buildclient.NewForConfig(cfg)
	if err != nil {
		klog.Fatal(err)
	}

	externalKubeClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatal(err)
	}
	securityClient := ctx.ClientBuilder.OpenshiftSecurityClientOrDie(infraBuildControllerServiceAccountName)

	buildInformer := ctx.BuildInformers.Build().V1().Builds()
	buildConfigInformer := ctx.BuildInformers.Build().V1().BuildConfigs()
	imageStreamInformer := ctx.ImageInformers.Image().V1().ImageStreams()
	podInformer := ctx.KubernetesInformers.Core().V1().Pods()
	secretInformer := ctx.KubernetesInformers.Core().V1().Secrets()
	serviceAccountInformer := ctx.KubernetesInformers.Core().V1().ServiceAccounts()
	controllerConfigInformer := ctx.ConfigInformers.Config().V1().Builds()
	imageConfigInformer := ctx.ConfigInformers.Config().V1().Images()
	openshiftConfigConfigMapInformer := ctx.OpenshiftConfigKubernetesInformers.Core().V1().ConfigMaps()
	controllerManagerConfigMapInformer := ctx.ControllerManagerKubeInformers.Core().V1().ConfigMaps()

	buildControllerParams := &buildcontroller.BuildControllerParams{
		BuildInformer:                      buildInformer,
		BuildConfigInformer:                buildConfigInformer,
		BuildControllerConfigInformer:      controllerConfigInformer,
		ImageConfigInformer:                imageConfigInformer,
		ImageStreamInformer:                imageStreamInformer,
		PodInformer:                        podInformer,
		SecretInformer:                     secretInformer,
		ServiceAccountInformer:             serviceAccountInformer,
		OpenshiftConfigConfigMapInformer:   openshiftConfigConfigMapInformer,
		ControllerManagerConfigMapInformer: controllerManagerConfigMapInformer,
		KubeClient:                         externalKubeClient,
		BuildClient:                        buildClient,
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
	clientName := infraBuildConfigChangeControllerServiceAccountName
	kubeExternalClient := ctx.ClientBuilder.ClientOrDie(clientName)
	buildClient := ctx.ClientBuilder.OpenshiftBuildClientOrDie(clientName)
	buildConfigInformer := ctx.BuildInformers.Build().V1().BuildConfigs()
	buildInformer := ctx.BuildInformers.Build().V1().Builds()

	controller := buildconfigcontroller.NewBuildConfigController(buildClient, kubeExternalClient, buildConfigInformer, buildInformer)
	go controller.Run(5, ctx.Stop)
	return true, nil
}
