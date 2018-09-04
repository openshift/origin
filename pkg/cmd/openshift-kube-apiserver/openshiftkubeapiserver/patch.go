package openshiftkubeapiserver

import (
	"fmt"
	"time"

	"k8s.io/apiserver/pkg/admission"
	admissionmetrics "k8s.io/apiserver/pkg/admission/metrics"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/options"
	clientgoinformers "k8s.io/client-go/informers"
	kexternalinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
	internalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/quota/generic"
	"k8s.io/kubernetes/pkg/quota/install"

	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthinformer "github.com/openshift/client-go/oauth/informers/externalversions"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	"github.com/openshift/origin/pkg/admission/namespaceconditions"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	originadmission "github.com/openshift/origin/pkg/cmd/server/origin/admission"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
	usercache "github.com/openshift/origin/pkg/user/cache"
)

type KubeAPIServerServerPatchContext struct {
	initialized bool

	postStartHooks     map[string]genericapiserver.PostStartHookFunc
	informerStartFuncs []func(stopCh <-chan struct{})
}

func NewOpenShiftKubeAPIServerConfigPatch(delegateAPIServer genericapiserver.DelegationTarget, kubeAPIServerConfig *configapi.KubeAPIServerConfig) (app.KubeAPIServerConfigFunc, *KubeAPIServerServerPatchContext) {
	patchContext := &KubeAPIServerServerPatchContext{
		postStartHooks: map[string]genericapiserver.PostStartHookFunc{},
	}
	return func(genericConfig *genericapiserver.Config, internalInformers internalinformers.SharedInformerFactory, kubeInformers clientgoinformers.SharedInformerFactory, pluginInitializers *[]admission.PluginInitializer) (genericapiserver.DelegationTarget, error) {
		kubeAPIServerInformers, err := NewInformers(internalInformers, kubeInformers, genericConfig.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}

		// AUTHENTICATOR
		authenticator, postStartHooks, err := NewAuthenticator(
			kubeAPIServerConfig.ServingInfo.ServingInfo,
			kubeAPIServerConfig.ServiceAccountPublicKeyFiles, kubeAPIServerConfig.OAuthConfig, kubeAPIServerConfig.AuthConfig,
			genericConfig.LoopbackClientConfig,
			kubeAPIServerInformers.OpenshiftOAuthInformers.Oauth().V1().OAuthClients().Lister(),
			kubeAPIServerInformers.OpenshiftUserInformers.User().V1().Groups())
		if err != nil {
			return nil, err
		}
		genericConfig.Authentication.Authenticator = authenticator
		for key, fn := range postStartHooks {
			patchContext.postStartHooks[key] = fn
		}
		// END AUTHENTICATOR

		// AUTHORIZER
		genericConfig.RequestInfoResolver = configprocessing.OpenshiftRequestInfoResolver()
		authorizer := NewAuthorizer(internalInformers, kubeInformers)
		genericConfig.Authorization.Authorizer = authorizer
		// END AUTHORIZER

		// ADMISSION
		projectCache, err := openshiftapiserver.NewProjectCache(kubeAPIServerInformers.InternalKubernetesInformers.Core().InternalVersion().Namespaces(), genericConfig.LoopbackClientConfig, kubeAPIServerConfig.ProjectConfig.DefaultNodeSelector)
		if err != nil {
			return nil, err
		}
		clusterQuotaMappingController := openshiftapiserver.NewClusterQuotaMappingController(kubeAPIServerInformers.InternalKubernetesInformers.Core().InternalVersion().Namespaces(), kubeAPIServerInformers.InternalOpenshiftQuotaInformers.Quota().InternalVersion().ClusterResourceQuotas())
		patchContext.postStartHooks["quota.openshift.io-clusterquotamapping"] = func(context genericapiserver.PostStartHookContext) error {
			go clusterQuotaMappingController.Run(5, context.StopCh)
			return nil
		}
		kubeClient, err := kubernetes.NewForConfig(genericConfig.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}
		registryHostnameRetriever, err := registryhostname.DefaultRegistryHostnameRetriever(genericConfig.LoopbackClientConfig, kubeAPIServerConfig.ImagePolicyConfig.ExternalRegistryHostname, kubeAPIServerConfig.ImagePolicyConfig.InternalRegistryHostname)
		if err != nil {
			return nil, err
		}
		// TODO make a union registry
		quotaRegistry := generic.NewRegistry(install.NewQuotaConfigurationForAdmission().Evaluators())
		openshiftPluginInitializer := &oadmission.PluginInitializer{
			ProjectCache:                 projectCache,
			OriginQuotaRegistry:          quotaRegistry,
			RESTClientConfig:             *genericConfig.LoopbackClientConfig,
			ClusterResourceQuotaInformer: kubeAPIServerInformers.GetInternalOpenshiftQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas(),
			ClusterQuotaMapper:           clusterQuotaMappingController.GetClusterQuotaMapper(),
			RegistryHostnameRetriever:    registryHostnameRetriever,
			SecurityInformers:            kubeAPIServerInformers.GetInternalOpenshiftSecurityInformers(),
			UserInformers:                kubeAPIServerInformers.GetOpenshiftUserInformers(),
		}
		*pluginInitializers = append(*pluginInitializers, openshiftPluginInitializer)

		// set up the decorators we need
		namespaceLabelDecorator := namespaceconditions.NamespaceLabelConditions{
			NamespaceClient: kubeClient.CoreV1(),
			NamespaceLister: kubeInformers.Core().V1().Namespaces().Lister(),

			SkipLevelZeroNames: originadmission.SkipRunLevelZeroPlugins,
			SkipLevelOneNames:  originadmission.SkipRunLevelOnePlugins,
		}
		options.AdmissionDecorator = admission.Decorators{
			admission.DecoratorFunc(namespaceLabelDecorator.WithNamespaceLabelConditions),
			admission.DecoratorFunc(admissionmetrics.WithControllerMetrics),
		}
		// END ADMISSION

		// HANDLER CHAIN (with oauth server and web console)
		genericConfig.BuildHandlerChainFunc, postStartHooks, err = BuildHandlerChain(genericConfig, kubeInformers, kubeAPIServerConfig.LegacyServiceServingCertSignerCABundle, kubeAPIServerConfig.OAuthConfig, kubeAPIServerConfig.UserAgentMatchingConfig)
		if err != nil {
			return nil, err
		}
		for key, fn := range postStartHooks {
			patchContext.postStartHooks[key] = fn
		}
		// END HANDLER CHAIN

		// CONSTRUCT DELEGATE
		nonAPIServerConfig, err := NewOpenshiftNonAPIConfig(genericConfig, kubeInformers, kubeAPIServerConfig.OAuthConfig, kubeAPIServerConfig.AuthConfig)
		if err != nil {
			return nil, err
		}
		openshiftNonAPIServer, err := nonAPIServerConfig.Complete().New(delegateAPIServer)
		if err != nil {
			return nil, err
		}
		// END CONSTRUCT DELEGATE

		patchContext.informerStartFuncs = append(patchContext.informerStartFuncs, kubeAPIServerInformers.Start)
		patchContext.initialized = true

		return openshiftNonAPIServer.GenericAPIServer, nil
	}, patchContext
}

func (c *KubeAPIServerServerPatchContext) PatchServer(server *master.Master) error {
	if !c.initialized {
		return fmt.Errorf("not initialized with config")
	}

	for name, fn := range c.postStartHooks {
		server.GenericAPIServer.AddPostStartHookOrDie(name, fn)
	}
	server.GenericAPIServer.AddPostStartHookOrDie("openshift.io-startkubeinformers", func(context genericapiserver.PostStartHookContext) error {
		for _, fn := range c.informerStartFuncs {
			fn(context.StopCh)
		}
		return nil
	})

	return nil
}

// NewInformers is only exposed for the build's integration testing until it can be fixed more appropriately.
func NewInformers(internalInformers internalinformers.SharedInformerFactory, versionedInformers clientgoinformers.SharedInformerFactory, loopbackClientConfig *rest.Config) (*KubeAPIServerInformers, error) {
	imageClient, err := imageclient.NewForConfig(loopbackClientConfig)
	if err != nil {
		return nil, err
	}
	oauthClient, err := oauthclient.NewForConfig(loopbackClientConfig)
	if err != nil {
		return nil, err
	}
	quotaClient, err := quotaclient.NewForConfig(loopbackClientConfig)
	if err != nil {
		return nil, err
	}
	securityClient, err := securityclient.NewForConfig(loopbackClientConfig)
	if err != nil {
		return nil, err
	}
	userClient, err := userclient.NewForConfig(loopbackClientConfig)
	if err != nil {
		return nil, err
	}

	// TODO find a single place to create and start informers.  During the 1.7 rebase this will come more naturally in a config object,
	// before then we should try to eliminate our direct to storage access.  It's making us do weird things.
	const defaultInformerResyncPeriod = 10 * time.Minute

	ret := &KubeAPIServerInformers{
		InternalKubernetesInformers:        internalInformers,
		KubernetesInformers:                versionedInformers,
		InternalOpenshiftImageInformers:    imageinformer.NewSharedInformerFactory(imageClient, defaultInformerResyncPeriod),
		OpenshiftOAuthInformers:            oauthinformer.NewSharedInformerFactory(oauthClient, defaultInformerResyncPeriod),
		InternalOpenshiftQuotaInformers:    quotainformer.NewSharedInformerFactory(quotaClient, defaultInformerResyncPeriod),
		InternalOpenshiftSecurityInformers: securityinformer.NewSharedInformerFactory(securityClient, defaultInformerResyncPeriod),
		OpenshiftUserInformers:             userinformer.NewSharedInformerFactory(userClient, defaultInformerResyncPeriod),
	}
	if err := ret.OpenshiftUserInformers.User().V1().Groups().Informer().AddIndexers(cache.Indexers{
		usercache.ByUserIndexName: usercache.ByUserIndexKeys,
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

type KubeAPIServerInformers struct {
	InternalKubernetesInformers        kinternalinformers.SharedInformerFactory
	KubernetesInformers                kexternalinformers.SharedInformerFactory
	OpenshiftOAuthInformers            oauthinformer.SharedInformerFactory
	InternalOpenshiftImageInformers    imageinformer.SharedInformerFactory
	InternalOpenshiftQuotaInformers    quotainformer.SharedInformerFactory
	InternalOpenshiftSecurityInformers securityinformer.SharedInformerFactory
	OpenshiftUserInformers             userinformer.SharedInformerFactory
}

func (i *KubeAPIServerInformers) GetInternalKubernetesInformers() kinternalinformers.SharedInformerFactory {
	return i.InternalKubernetesInformers
}
func (i *KubeAPIServerInformers) GetKubernetesInformers() kexternalinformers.SharedInformerFactory {
	return i.KubernetesInformers
}
func (i *KubeAPIServerInformers) GetInternalOpenshiftImageInformers() imageinformer.SharedInformerFactory {
	return i.InternalOpenshiftImageInformers
}
func (i *KubeAPIServerInformers) GetInternalOpenshiftQuotaInformers() quotainformer.SharedInformerFactory {
	return i.InternalOpenshiftQuotaInformers
}
func (i *KubeAPIServerInformers) GetInternalOpenshiftSecurityInformers() securityinformer.SharedInformerFactory {
	return i.InternalOpenshiftSecurityInformers
}
func (i *KubeAPIServerInformers) GetOpenshiftUserInformers() userinformer.SharedInformerFactory {
	return i.OpenshiftUserInformers
}

func (i *KubeAPIServerInformers) Start(stopCh <-chan struct{}) {
	i.InternalKubernetesInformers.Start(stopCh)
	i.KubernetesInformers.Start(stopCh)
	i.OpenshiftOAuthInformers.Start(stopCh)
	i.InternalOpenshiftImageInformers.Start(stopCh)
	i.InternalOpenshiftQuotaInformers.Start(stopCh)
	i.InternalOpenshiftSecurityInformers.Start(stopCh)
	i.OpenshiftUserInformers.Start(stopCh)
}
