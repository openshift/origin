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

	imagev1client "github.com/openshift/client-go/image/clientset/versioned"
	imagev1informer "github.com/openshift/client-go/image/informers/externalversions"
	userv1informer "github.com/openshift/client-go/user/informers/externalversions"
	"github.com/openshift/origin/pkg/build/apiserver/admission/jenkinsbootstrapper"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/quota/image"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
)

type InformerAccess interface {
	GetInternalKubernetesInformers() kinternalinformers.SharedInformerFactory
	GetKubernetesInformers() kexternalinformers.SharedInformerFactory
	GetOpenshiftImageInformers() imagev1informer.SharedInformerFactory
	GetInternalOpenshiftQuotaInformers() quotainformer.SharedInformerFactory
	GetInternalOpenshiftSecurityInformers() securityinformer.SharedInformerFactory
	GetOpenshiftUserInformers() userv1informer.SharedInformerFactory
}

func NewPluginInitializer(
	externalImageRegistryHostname string,
	internalImageRegistryHostname string,
	cloudConfigFile string,
	jenkinsConfig configapi.JenkinsPipelineConfig,
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
	imageClient, err := imagev1client.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}

	// TODO make a union registry
	quotaRegistry := generic.NewRegistry(install.NewQuotaConfigurationForAdmission().Evaluators())
	imageEvaluators := image.NewReplenishmentEvaluatorsForAdmission(
		informers.GetOpenshiftImageInformers().Image().V1().ImageStreams(),
		imageClient.ImageV1(),
	)
	for i := range imageEvaluators {
		quotaRegistry.Add(imageEvaluators[i])
	}

	registryHostnameRetriever, err := registryhostname.DefaultRegistryHostnameRetriever(privilegedLoopbackConfig, externalImageRegistryHostname, internalImageRegistryHostname)
	if err != nil {
		return nil, err
	}

	var cloudConfig []byte
	if len(cloudConfigFile) != 0 {
		var err error
		cloudConfig, err = ioutil.ReadFile(cloudConfigFile)
		if err != nil {
			return nil, fmt.Errorf("error reading from cloud configuration file %s: %v", cloudConfigFile, err)
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
		RESTClientConfig:             *privilegedLoopbackConfig,
		ClusterResourceQuotaInformer: informers.GetInternalOpenshiftQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas(),
		ClusterQuotaMapper:           clusterQuotaMappingController.GetClusterQuotaMapper(),
		RegistryHostnameRetriever:    registryHostnameRetriever,
		SecurityInformers:            informers.GetInternalOpenshiftSecurityInformers(),
		UserInformers:                informers.GetOpenshiftUserInformers(),
	}
	jenkinsPipelineConfigInitializer := &jenkinsbootstrapper.PluginInitializer{
		JenkinsPipelineConfig: jenkinsConfig,
	}

	return admission.PluginInitializers{genericInitializer, webhookInitializer, kubePluginInitializer, openshiftPluginInitializer, jenkinsPipelineConfigInitializer}, nil
}
