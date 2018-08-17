package admission

import (
	"fmt"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	webhookconfig "k8s.io/apiserver/pkg/admission/plugin/webhook/config"
	webhookinitializer "k8s.io/apiserver/pkg/admission/plugin/webhook/initializer"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kexternalinformers "k8s.io/client-go/informers"
	kubeclientgoclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	"k8s.io/kubernetes/pkg/quota/generic"
	"k8s.io/kubernetes/pkg/quota/install"

	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/quota/image"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
)

type InformerAccess interface {
	GetInternalKubernetesInformers() kinternalinformers.SharedInformerFactory
	GetKubernetesInformers() kexternalinformers.SharedInformerFactory
	GetInternalOpenshiftImageInformers() imageinformer.SharedInformerFactory
	GetInternalOpenshiftQuotaInformers() quotainformer.SharedInformerFactory
	GetInternalOpenshiftSecurityInformers() securityinformer.SharedInformerFactory
	GetOpenshiftUserInformers() userinformer.SharedInformerFactory
}

func NewPluginInitializer(
	options configapi.MasterConfig,
	privilegedLoopbackConfig *rest.Config,
	informers InformerAccess,
	authorizer authorizer.Authorizer,
	projectCache *projectcache.ProjectCache,
	restMapper meta.RESTMapper,
	clusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController,
) (admission.PluginInitializer, error) {
	kubeInternalClient, err := kclientsetinternal.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubeclientgoclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}
	imageClient, err := imageclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}

	// TODO make a union registry
	quotaRegistry := generic.NewRegistry(install.NewQuotaConfigurationForAdmission().Evaluators())
	imageEvaluators := image.NewReplenishmentEvaluatorsForAdmission(
		informers.GetInternalOpenshiftImageInformers().Image().InternalVersion().ImageStreams(),
		imageClient.Image(),
	)
	for i := range imageEvaluators {
		quotaRegistry.Add(imageEvaluators[i])
	}

	registryHostnameRetriever, err := registryhostname.DefaultRegistryHostnameRetriever(privilegedLoopbackConfig, options.ImagePolicyConfig.ExternalRegistryHostname, options.ImagePolicyConfig.InternalRegistryHostname)
	if err != nil {
		return nil, err
	}

	// punch through layers to build this in order to get a string for a cloud provider file
	// TODO refactor us into a forward building flow with a side channel like this
	kubeOptions, err := kubernetes.BuildKubeAPIserverOptions(options)
	if err != nil {
		return nil, err
	}

	var cloudConfig []byte
	if kubeOptions.CloudProvider.CloudConfigFile != "" {
		var err error
		cloudConfig, err = ioutil.ReadFile(kubeOptions.CloudProvider.CloudConfigFile)
		if err != nil {
			return nil, fmt.Errorf("Error reading from cloud configuration file %s: %v", kubeOptions.CloudProvider.CloudConfigFile, err)
		}
	}
	// note: we are passing a combined quota registry here...
	genericInitializer := initializer.New(
		kubeClient,
		informers.GetKubernetesInformers(),
		authorizer,
		legacyscheme.Scheme,
	)
	kubePluginInitializer := kadmission.NewPluginInitializer(
		kubeInternalClient,
		informers.GetInternalKubernetesInformers(),
		cloudConfig,
		restMapper,
		generic.NewConfiguration(quotaRegistry.List(), map[schema.GroupResource]struct{}{}))

	webhookAuthResolverWrapper := func(delegate webhookconfig.AuthenticationInfoResolver) webhookconfig.AuthenticationInfoResolver {
		return &webhookconfig.AuthenticationInfoResolverDelegator{
			ClientConfigForFunc: func(server string) (*rest.Config, error) {
				if server == "kubernetes.default.svc" {
					return rest.CopyConfig(privilegedLoopbackConfig), nil
				}
				return delegate.ClientConfigFor(server)
			},
			ClientConfigForServiceFunc: func(serviceName, serviceNamespace string) (*rest.Config, error) {
				if serviceName == "kubernetes" && serviceNamespace == "default" {
					return rest.CopyConfig(privilegedLoopbackConfig), nil
				}
				return delegate.ClientConfigForService(serviceName, serviceNamespace)
			},
		}
	}

	webhookInitializer := webhookinitializer.NewPluginInitializer(
		webhookAuthResolverWrapper,
		aggregatorapiserver.NewClusterIPServiceResolver(informers.GetKubernetesInformers().Core().V1().Services().Lister()),
	)

	openshiftPluginInitializer := &oadmission.PluginInitializer{
		ProjectCache:                 projectCache,
		OriginQuotaRegistry:          quotaRegistry,
		JenkinsPipelineConfig:        options.JenkinsPipelineConfig,
		RESTClientConfig:             *privilegedLoopbackConfig,
		ClusterResourceQuotaInformer: informers.GetInternalOpenshiftQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas(),
		ClusterQuotaMapper:           clusterQuotaMappingController.GetClusterQuotaMapper(),
		RegistryHostnameRetriever:    registryHostnameRetriever,
		SecurityInformers:            informers.GetInternalOpenshiftSecurityInformers(),
		UserInformers:                informers.GetOpenshiftUserInformers(),
	}

	return admission.PluginInitializers{genericInitializer, webhookInitializer, kubePluginInitializer, openshiftPluginInitializer}, nil
}

type DefaultInformerAccess struct {
	InternalKubernetesInformers        kinternalinformers.SharedInformerFactory
	KubernetesInformers                kexternalinformers.SharedInformerFactory
	InternalOpenshiftImageInformers    imageinformer.SharedInformerFactory
	InternalOpenshiftQuotaInformers    quotainformer.SharedInformerFactory
	InternalOpenshiftSecurityInformers securityinformer.SharedInformerFactory
	OpenshiftUserInformers             userinformer.SharedInformerFactory
}

func (i *DefaultInformerAccess) GetInternalKubernetesInformers() kinternalinformers.SharedInformerFactory {
	return i.InternalKubernetesInformers
}
func (i *DefaultInformerAccess) GetKubernetesInformers() kexternalinformers.SharedInformerFactory {
	return i.KubernetesInformers
}
func (i *DefaultInformerAccess) GetInternalOpenshiftImageInformers() imageinformer.SharedInformerFactory {
	return i.InternalOpenshiftImageInformers
}
func (i *DefaultInformerAccess) GetInternalOpenshiftQuotaInformers() quotainformer.SharedInformerFactory {
	return i.InternalOpenshiftQuotaInformers
}
func (i *DefaultInformerAccess) GetInternalOpenshiftSecurityInformers() securityinformer.SharedInformerFactory {
	return i.InternalOpenshiftSecurityInformers
}
func (i *DefaultInformerAccess) GetOpenshiftUserInformers() userinformer.SharedInformerFactory {
	return i.OpenshiftUserInformers
}
