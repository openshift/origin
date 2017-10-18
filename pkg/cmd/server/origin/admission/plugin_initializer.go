package admission

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	oquota "github.com/openshift/origin/pkg/quota"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/service"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	userinformer "github.com/openshift/origin/pkg/user/generated/informers/internalversion"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	kubeclientgoinformers "k8s.io/client-go/informers"
	kubeclientgoclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kexternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
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
	kubeExternalClient, err := kclientsetexternal.NewForConfig(privilegedLoopbackConfig)
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

	quotaRegistry := oquota.NewAllResourceQuotaRegistryForAdmission(
		informers.GetExternalKubeInformers(),
		informers.GetImageInformers().Image().InternalVersion().ImageStreams(),
		imageClient.Image(),
		kubeExternalClient,
	)

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
	genericInitializer, err := initializer.New(kubeClientGoClientSet, informers.GetClientGoKubeInformers(), authorizer)
	if err != nil {
		return nil, nil, err
	}
	kubePluginInitializer := kadmission.NewPluginInitializer(
		kubeInternalClient,
		kubeExternalClient,
		informers.GetInternalKubeInformers(),
		authorizer,
		cloudConfig,
		restMapper,
		quotaRegistry)
	// upstream broke this, so we can't use their mechanism.  We need to get an actual client cert and practically speaking privileged loopback will always have one
	kubePluginInitializer.SetClientCert(privilegedLoopbackConfig.TLSClientConfig.CertData, privilegedLoopbackConfig.TLSClientConfig.KeyData)
	// this is a really problematic thing, because it breaks DNS resolution and IP routing, but its for an alpha feature that
	// I need to work cluster-up
	kubePluginInitializer.SetServiceResolver(aggregatorapiserver.NewClusterIPServiceResolver(
		informers.GetClientGoKubeInformers().Core().V1().Services().Lister(),
	))

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

	return admission.PluginInitializers{genericInitializer, kubePluginInitializer, openshiftPluginInitializer},
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
