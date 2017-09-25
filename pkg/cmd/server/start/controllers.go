package start

import (
	"k8s.io/client-go/rest"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	cmappoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	origincontrollers "github.com/openshift/origin/pkg/cmd/server/origin/controller"
)

func getControllerContext(options configapi.MasterConfig, controllerManagerOptions *cmappoptions.CMServer, cloudProvider cloudprovider.Interface, informers *informers, stopCh <-chan struct{}) (origincontrollers.ControllerContext, error) {
	loopbackConfig, _, kubeExternal, _, err := getAllClients(options)
	if err != nil {
		return origincontrollers.ControllerContext{}, err
	}
	// divide up the QPS since it re-used separately for every client
	// TODO, eventually make this configurable individually in some way.
	if loopbackConfig.QPS > 0 {
		loopbackConfig.QPS = loopbackConfig.QPS/10 + 1
	}
	if loopbackConfig.Burst > 0 {
		loopbackConfig.Burst = loopbackConfig.Burst/10 + 1
	}

	rootClientBuilder := controller.SimpleControllerClientBuilder{
		ClientConfig: loopbackConfig,
	}

	availableResources, err := cmapp.GetAvailableResources(rootClientBuilder)
	if err != nil {
		return origincontrollers.ControllerContext{}, err
	}

	openshiftControllerContext := origincontrollers.ControllerContext{
		KubeControllerContext: cmapp.ControllerContext{
			ClientBuilder: controller.SAControllerClientBuilder{
				ClientConfig:         rest.AnonymousClientConfig(loopbackConfig),
				CoreClient:           kubeExternal.Core(),
				AuthenticationClient: kubeExternal.Authentication(),
				Namespace:            "kube-system",
			},
			InformerFactory:    newGenericInformers(informers),
			Options:            *controllerManagerOptions,
			AvailableResources: availableResources,
			Cloud:              cloudProvider,
			Stop:               stopCh,
		},
		ClientBuilder: origincontrollers.OpenshiftControllerClientBuilder{
			ControllerClientBuilder: controller.SAControllerClientBuilder{
				ClientConfig:         rest.AnonymousClientConfig(loopbackConfig),
				CoreClient:           kubeExternal.Core(),
				AuthenticationClient: kubeExternal.Authentication(),
				Namespace:            bootstrappolicy.DefaultOpenShiftInfraNamespace,
			},
		},
		InternalKubeInformers:  informers.internalKubeInformers,
		ExternalKubeInformers:  informers.externalKubeInformers,
		AppInformers:           informers.appInformers,
		AuthorizationInformers: informers.authorizationInformers,
		BuildInformers:         informers.buildInformers,
		ImageInformers:         informers.imageInformers,
		QuotaInformers:         informers.quotaInformers,
		SecurityInformers:      informers.securityInformers,
		TemplateInformers:      informers.templateInformers,
		Stop:                   stopCh,
	}

	return openshiftControllerContext, nil
}
