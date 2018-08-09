package openshiftkubeapiserver

import (
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	usercache "github.com/openshift/origin/pkg/user/cache"
	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	clientgoinformers "k8s.io/client-go/informers"
	kexternalinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	internalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/master"

	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthinformer "github.com/openshift/client-go/oauth/informers/externalversions"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	originadmission "github.com/openshift/origin/pkg/cmd/server/origin/admission"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"

	"fmt"
	"time"

	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type KubeAPIServerServerPatchContext struct {
	initialized bool

	RESTMapper         *restmapper.DeferredDiscoveryRESTMapper
	postStartHooks     map[string]genericapiserver.PostStartHookFunc
	informerStartFuncs []func(stopCh <-chan struct{})
}

type KubeAPIServerConfigFunc func(config *master.Config, internalInformers internalinformers.SharedInformerFactory, kubeInformers clientgoinformers.SharedInformerFactory, pluginInitializers *[]admission.PluginInitializer, stopCh <-chan struct{}) (genericapiserver.DelegationTarget, error)

func NewOpenShiftKubeAPIServerConfigPatch(delegateAPIServer genericapiserver.DelegationTarget, kubeAPIServerConfig *configapi.MasterConfig) (KubeAPIServerConfigFunc, *KubeAPIServerServerPatchContext) {
	patchContext := &KubeAPIServerServerPatchContext{
		postStartHooks: map[string]genericapiserver.PostStartHookFunc{},
	}
	return func(config *master.Config, internalInformers internalinformers.SharedInformerFactory, kubeInformers clientgoinformers.SharedInformerFactory, pluginInitializers *[]admission.PluginInitializer, stopCh <-chan struct{}) (genericapiserver.DelegationTarget, error) {
		kubeAPIServerInformers, err := NewInformers(internalInformers, kubeInformers, config.GenericConfig.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}

		// AUTHENTICATOR
		authenticator, postStartHooks, err := NewAuthenticator(
			*kubeAPIServerConfig,
			config.GenericConfig.LoopbackClientConfig,
			kubeAPIServerInformers.OpenshiftOAuthInformers.Oauth().V1().OAuthClients().Lister(),
			kubeAPIServerInformers.OpenshiftUserInformers.User().V1().Groups())
		if err != nil {
			return nil, err
		}
		config.GenericConfig.Authentication.Authenticator = authenticator
		for key, fn := range postStartHooks {
			patchContext.postStartHooks[key] = fn
		}
		// END AUTHENTICATOR

		// AUTHORIZER
		authorizer := NewAuthorizer(internalInformers, kubeInformers)
		config.GenericConfig.Authorization.Authorizer = authorizer
		// END AUTHORIZER

		// ADMISSION
		projectCache, err := openshiftapiserver.NewProjectCache(kubeAPIServerInformers.InternalKubernetesInformers.Core().InternalVersion().Namespaces(), config.GenericConfig.LoopbackClientConfig, kubeAPIServerConfig.ProjectConfig.DefaultNodeSelector)
		if err != nil {
			return nil, err
		}
		clusterQuotaMappingController := openshiftapiserver.NewClusterQuotaMappingController(kubeAPIServerInformers.InternalKubernetesInformers.Core().InternalVersion().Namespaces(), kubeAPIServerInformers.InternalOpenshiftQuotaInformers.Quota().InternalVersion().ClusterResourceQuotas())
		kubeClient, err := kubernetes.NewForConfig(config.GenericConfig.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}
		discoveryClient := cacheddiscovery.NewMemCacheClient(kubeClient.Discovery())
		restMapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
		admissionInitializer, err := originadmission.NewPluginInitializer(*kubeAPIServerConfig, config.GenericConfig.LoopbackClientConfig, kubeAPIServerInformers, config.GenericConfig.Authorization.Authorizer, projectCache, restMapper, clusterQuotaMappingController)
		if err != nil {
			return nil, err
		}
		*pluginInitializers = []admission.PluginInitializer{admissionInitializer}
		// END ADMISSION

		// HANDLER CHAIN (with oauth server and web console)
		config.GenericConfig.BuildHandlerChainFunc, postStartHooks, err = BuildHandlerChain(config.GenericConfig, kubeInformers, kubeAPIServerConfig, stopCh)
		if err != nil {
			return nil, err
		}
		for key, fn := range postStartHooks {
			patchContext.postStartHooks[key] = fn
		}
		// END HANDLER CHAIN

		// CONSTRUCT DELEGATE
		nonAPIServerConfig, err := NewOpenshiftNonAPIConfig(config.GenericConfig, kubeInformers, kubeAPIServerConfig)
		if err != nil {
			return nil, err
		}
		openshiftNonAPIServer, err := nonAPIServerConfig.Complete().New(delegateAPIServer)
		if err != nil {
			return nil, err
		}
		// END CONSTRUCT DELEGATE

		patchContext.informerStartFuncs = append(patchContext.informerStartFuncs, kubeAPIServerInformers.Start)
		patchContext.RESTMapper = restMapper
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
	server.GenericAPIServer.AddPostStartHookOrDie("openshift.io-restmapperupdater", func(context genericapiserver.PostStartHookContext) error {
		c.RESTMapper.Reset()
		go func() {
			wait.Until(func() {
				c.RESTMapper.Reset()
			}, 10*time.Second, context.StopCh)
		}()
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
