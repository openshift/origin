package admission

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	authorizationclient "github.com/openshift/client-go/authorization/clientset/versioned"
	buildclient "github.com/openshift/client-go/build/clientset/versioned"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	"github.com/openshift/origin/pkg/quota/image"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/service"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	webhookconfig "k8s.io/apiserver/pkg/admission/plugin/webhook/config"
	webhookinitializer "k8s.io/apiserver/pkg/admission/plugin/webhook/initializer"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	kexternalinformers "k8s.io/client-go/informers"
	kubeclientgoinformers "k8s.io/client-go/informers"
	kubeclientgoclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	"k8s.io/kubernetes/pkg/quota/generic"
	"k8s.io/kubernetes/pkg/quota/install"
)

type InformerAccess interface {
	GetInternalKubeInformers() kinternalinformers.SharedInformerFactory
	GetExternalKubeInformers() kexternalinformers.SharedInformerFactory
	GetClientGoKubeInformers() kubeclientgoinformers.SharedInformerFactory
	GetImageInformers() imageinformer.SharedInformerFactory
	GetQuotaInformers() quotainformer.SharedInformerFactory
	GetSecurityInformers() securityinformer.SharedInformerFactory
	GetUserInformers() userinformer.SharedInformerFactory
}

func NewPluginInitializer(
	options configapi.MasterConfig,
	privilegedLoopbackConfig *rest.Config,
	informers InformerAccess,
	authorizer authorizer.Authorizer,
	projectCache *projectcache.ProjectCache,
	clusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController,
) (admission.PluginInitializer, genericapiserver.PostStartHookFunc, error) {
	kubeInternalClient, err := kclientsetinternal.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}
	kubeClientGoClientSet, err := kubeclientgoclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}
	authorizationClient, err := authorizationclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}
	buildClient, err := buildclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}
	imageClient, err := imageclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}
	quotaClient, err := quotaclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}
	templateClient, err := templateclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}
	userClient, err := userclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, nil, err
	}

	// TODO make a union registry
	quotaRegistry := generic.NewRegistry(install.NewQuotaConfigurationForAdmission().Evaluators())
	imageEvaluators := image.NewReplenishmentEvaluatorsForAdmission(
		informers.GetImageInformers().Image().InternalVersion().ImageStreams(),
		imageClient.Image(),
	)
	for i := range imageEvaluators {
		quotaRegistry.Add(imageEvaluators[i])
	}

	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(kubeInternalClient.Core().Services(metav1.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		return nil, nil, fmt.Errorf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	// Use a discovery client capable of being refreshed.
	discoveryClient := cacheddiscovery.NewMemCacheClient(kubeInternalClient.Discovery())
	restMapper := discovery.NewDeferredDiscoveryRESTMapper(discoveryClient, meta.InterfacesForUnstructured)

	// punch through layers to build this in order to get a string for a cloud provider file
	// TODO refactor us into a forward building flow with a side channel like this
	kubeOptions, err := kubernetes.BuildKubeAPIserverOptions(options)
	if err != nil {
		return nil, nil, err
	}

	var cloudConfig []byte
	if kubeOptions.CloudProvider.CloudConfigFile != "" {
		var err error
		cloudConfig, err = ioutil.ReadFile(kubeOptions.CloudProvider.CloudConfigFile)
		if err != nil {
			return nil, nil, fmt.Errorf("Error reading from cloud configuration file %s: %v", kubeOptions.CloudProvider.CloudConfigFile, err)
		}
	}
	// note: we are passing a combined quota registry here...
	genericInitializer := initializer.New(
		kubeClientGoClientSet,
		informers.GetClientGoKubeInformers(),
		authorizer,
		legacyscheme.Scheme,
	)
	kubePluginInitializer := kadmission.NewPluginInitializer(
		kubeInternalClient,
		informers.GetInternalKubeInformers(),
		cloudConfig,
		restMapper,
		generic.NewConfiguration(quotaRegistry.List(), map[schema.GroupResource]struct{}{}))

	webhookInitializer := webhookinitializer.NewPluginInitializer(
		func(delegate webhookconfig.AuthenticationInfoResolver) webhookconfig.AuthenticationInfoResolver {
			return webhookconfig.AuthenticationInfoResolverFunc(func(server string) (*rest.Config, error) {
				if server == "kubernetes.default.svc" {
					return rest.CopyConfig(privilegedLoopbackConfig), nil
				}
				return delegate.ClientConfigFor(server)
			})
		},
		aggregatorapiserver.NewClusterIPServiceResolver(informers.GetClientGoKubeInformers().Core().V1().Services().Lister()),
	)

	openshiftPluginInitializer := &oadmission.PluginInitializer{
		OpenshiftInternalAuthorizationClient: authorizationClient,
		OpenshiftInternalBuildClient:         buildClient,
		OpenshiftInternalImageClient:         imageClient,
		OpenshiftInternalQuotaClient:         quotaClient,
		OpenshiftInternalTemplateClient:      templateClient,
		OpenshiftInternalUserClient:          userClient,
		ProjectCache:                         projectCache,
		OriginQuotaRegistry:                  quotaRegistry,
		Authorizer:                           authorizer,
		JenkinsPipelineConfig:                options.JenkinsPipelineConfig,
		RESTClientConfig:                     *privilegedLoopbackConfig,
		Informers:                            informers.GetInternalKubeInformers(),
		ClusterResourceQuotaInformer:         informers.GetQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas(),
		ClusterQuotaMapper:                   clusterQuotaMappingController.GetClusterQuotaMapper(),
		RegistryHostnameRetriever:            imageapi.DefaultRegistryHostnameRetriever(defaultRegistryFunc, options.ImagePolicyConfig.ExternalRegistryHostname, options.ImagePolicyConfig.InternalRegistryHostname),
		SecurityInformers:                    informers.GetSecurityInformers(),
		UserInformers:                        informers.GetUserInformers(),
	}

	return admission.PluginInitializers{genericInitializer, webhookInitializer, kubePluginInitializer, openshiftPluginInitializer},
		func(context genericapiserver.PostStartHookContext) error {
			restMapper.Reset()
			go func() {
				wait.Until(func() {
					restMapper.Reset()
				}, 10*time.Second, context.StopCh)
			}()
			return nil
		},
		nil
}

// env returns an environment variable, or the defaultValue if it is not set.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}
