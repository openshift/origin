package start

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	cmappoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	kexternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	origincontrollers "github.com/openshift/origin/pkg/cmd/server/origin/controller"
)

func getControllerContext(options configapi.MasterConfig, controllerManagerOptions *cmappoptions.CMServer, cloudProvider cloudprovider.Interface, informers *informers, stopCh <-chan struct{}) (origincontrollers.ControllerContext, error) {
	loopbackConfig, _, kubeExternal, _, _, err := getAllClients(options)
	if err != nil {
		return origincontrollers.ControllerContext{}, err
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
			InformerFactory: genericInformers{
				SharedInformerFactory: informers.GetExternalKubeInformers(),
				generic: []GenericResourceInformer{
					// use our existing internal informers to satisfy the generic informer requests (which don't require strong
					// types).
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
						return informers.appInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
						return informers.authorizationInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
						return informers.buildInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
						return informers.imageInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
						return informers.quotaInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
						return informers.securityInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
						return informers.templateInformers.ForResource(resource)
					}),
					informers.externalKubeInformers,
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
						return informers.internalKubeInformers.ForResource(resource)
					}),
				},
				bias: map[schema.GroupVersionResource]schema.GroupVersionResource{
					{Group: "rbac.authorization.k8s.io", Resource: "rolebindings", Version: "v1beta1"}:        {Group: "rbac.authorization.k8s.io", Resource: "rolebindings", Version: runtime.APIVersionInternal},
					{Group: "rbac.authorization.k8s.io", Resource: "clusterrolebindings", Version: "v1beta1"}: {Group: "rbac.authorization.k8s.io", Resource: "clusterrolebindings", Version: runtime.APIVersionInternal},
					{Group: "rbac.authorization.k8s.io", Resource: "roles", Version: "v1beta1"}:               {Group: "rbac.authorization.k8s.io", Resource: "roles", Version: runtime.APIVersionInternal},
					{Group: "rbac.authorization.k8s.io", Resource: "clusterroles", Version: "v1beta1"}:        {Group: "rbac.authorization.k8s.io", Resource: "clusterroles", Version: runtime.APIVersionInternal},
					{Group: "", Resource: "securitycontextconstraints", Version: "v1"}:                        {Group: "", Resource: "securitycontextconstraints", Version: runtime.APIVersionInternal},
				},
			},
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
