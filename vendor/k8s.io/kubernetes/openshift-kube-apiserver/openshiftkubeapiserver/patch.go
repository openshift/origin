package openshiftkubeapiserver

import (
	"fmt"
	"time"

	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	clientgoinformers "k8s.io/client-go/informers"
	kexternalinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/quota/v1/generic"
	"k8s.io/kubernetes/pkg/quota/v1/install"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	"github.com/openshift/apiserver-library-go/pkg/admission/imagepolicy"
	"github.com/openshift/apiserver-library-go/pkg/admission/imagepolicy/imagereferencemutators"
	"github.com/openshift/apiserver-library-go/pkg/admission/quota/clusterresourcequota"
	"github.com/openshift/apiserver-library-go/pkg/securitycontextconstraints/sccadmission"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthinformer "github.com/openshift/client-go/oauth/informers/externalversions"
	quotaclient "github.com/openshift/client-go/quota/clientset/versioned"
	quotainformer "github.com/openshift/client-go/quota/informers/externalversions"
	quotav1informer "github.com/openshift/client-go/quota/informers/externalversions/quota/v1"
	securityv1client "github.com/openshift/client-go/security/clientset/versioned"
	securityv1informer "github.com/openshift/client-go/security/informers/externalversions"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	"github.com/openshift/library-go/pkg/apiserver/admission/admissionrestconfig"
	"github.com/openshift/library-go/pkg/apiserver/apiserverconfig"
	"github.com/openshift/library-go/pkg/quota/clusterquotamapping"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/authorization/restrictusers"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/authorization/restrictusers/usercache"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/scheduler/nodeenv"
)

type KubeAPIServerServerPatchContext struct {
	initialized bool

	postStartHooks     map[string]genericapiserver.PostStartHookFunc
	informerStartFuncs []func(stopCh <-chan struct{})
}

type KubeAPIServerConfigFunc func(config *genericapiserver.Config, versionedInformers clientgoinformers.SharedInformerFactory, pluginInitializers *[]admission.PluginInitializer) (genericapiserver.DelegationTarget, error)

func NewOpenShiftKubeAPIServerConfigPatch(delegateAPIServer genericapiserver.DelegationTarget, kubeAPIServerConfig *kubecontrolplanev1.KubeAPIServerConfig) (KubeAPIServerConfigFunc, *KubeAPIServerServerPatchContext) {
	patchContext := &KubeAPIServerServerPatchContext{
		postStartHooks: map[string]genericapiserver.PostStartHookFunc{},
	}
	return func(genericConfig *genericapiserver.Config, kubeInformers clientgoinformers.SharedInformerFactory, pluginInitializers *[]admission.PluginInitializer) (genericapiserver.DelegationTarget, error) {
		kubeAPIServerInformers, err := NewInformers(kubeInformers, genericConfig.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}

		// AUTHENTICATOR
		authenticator, postStartHooks, err := NewAuthenticator(
			kubeAPIServerConfig.ServingInfo.ServingInfo,
			kubeAPIServerConfig.ServiceAccountPublicKeyFiles, kubeAPIServerConfig.OAuthConfig, kubeAPIServerConfig.AuthConfig,
			genericConfig.LoopbackClientConfig,
			kubeAPIServerInformers.KubernetesInformers.Core().V1().Pods().Lister(),
			kubeAPIServerInformers.KubernetesInformers.Core().V1().Secrets().Lister(),
			kubeAPIServerInformers.KubernetesInformers.Core().V1().ServiceAccounts().Lister(),
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
		genericConfig.RequestInfoResolver = apiserverconfig.OpenshiftRequestInfoResolver()
		authorizer := NewAuthorizer(kubeInformers)
		genericConfig.Authorization.Authorizer = authorizer
		// END AUTHORIZER

		// Inject OpenShift API long running endpoints (like for binary builds).
		// TODO: We should disable the timeout code for aggregated endpoints as this can cause problems when upstream add additional endpoints.
		genericConfig.LongRunningFunc = apiserverconfig.IsLongRunningRequest

		// ADMISSION
		clusterQuotaMappingController := newClusterQuotaMappingController(kubeAPIServerInformers.KubernetesInformers.Core().V1().Namespaces(), kubeAPIServerInformers.OpenshiftQuotaInformers.Quota().V1().ClusterResourceQuotas())
		patchContext.postStartHooks["quota.openshift.io-clusterquotamapping"] = func(context genericapiserver.PostStartHookContext) error {
			go clusterQuotaMappingController.Run(5, context.StopCh)
			return nil
		}

		*pluginInitializers = append(*pluginInitializers,
			imagepolicy.NewInitializer(imagereferencemutators.KubeImageMutators{}, kubeAPIServerConfig.ImagePolicyConfig.InternalRegistryHostname),
			restrictusers.NewInitializer(kubeAPIServerInformers.GetOpenshiftUserInformers()),
			sccadmission.NewInitializer(kubeAPIServerInformers.GetOpenshiftSecurityInformers().Security().V1().SecurityContextConstraints()),
			clusterresourcequota.NewInitializer(
				kubeAPIServerInformers.GetOpenshiftQuotaInformers().Quota().V1().ClusterResourceQuotas(),
				clusterQuotaMappingController.GetClusterQuotaMapper(),
				generic.NewRegistry(install.NewQuotaConfigurationForAdmission().Evaluators()),
			),
			nodeenv.NewInitializer(kubeAPIServerConfig.ProjectConfig.DefaultNodeSelector),
			admissionrestconfig.NewInitializer(*rest.CopyConfig(genericConfig.LoopbackClientConfig)),
		)
		// END ADMISSION

		// HANDLER CHAIN (with oauth server and web console)
		genericConfig.BuildHandlerChainFunc, err = BuildHandlerChain(kubeAPIServerConfig.ConsolePublicURL)
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
		patchContext.postStartHooks["openshift.io-kubernetes-informers-synched"] = func(context genericapiserver.PostStartHookContext) error {
			kubeInformers.WaitForCacheSync(context.StopCh)
			return nil
		}
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
func NewInformers(versionedInformers clientgoinformers.SharedInformerFactory, loopbackClientConfig *rest.Config) (*KubeAPIServerInformers, error) {
	// ClusterResourceQuota is served using CRD resource any status update must use JSON
	jsonLoopbackClientConfig := rest.CopyConfig(loopbackClientConfig)
	jsonLoopbackClientConfig.ContentConfig.AcceptContentTypes = "application/json"
	jsonLoopbackClientConfig.ContentConfig.ContentType = "application/json"

	oauthClient, err := oauthclient.NewForConfig(loopbackClientConfig)
	if err != nil {
		return nil, err
	}
	quotaClient, err := quotaclient.NewForConfig(jsonLoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	securityClient, err := securityv1client.NewForConfig(jsonLoopbackClientConfig)
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
		KubernetesInformers:        versionedInformers,
		OpenshiftOAuthInformers:    oauthinformer.NewSharedInformerFactory(oauthClient, defaultInformerResyncPeriod),
		OpenshiftQuotaInformers:    quotainformer.NewSharedInformerFactory(quotaClient, defaultInformerResyncPeriod),
		OpenshiftSecurityInformers: securityv1informer.NewSharedInformerFactory(securityClient, defaultInformerResyncPeriod),
		OpenshiftUserInformers:     userinformer.NewSharedInformerFactory(userClient, defaultInformerResyncPeriod),
	}
	if err := ret.OpenshiftUserInformers.User().V1().Groups().Informer().AddIndexers(cache.Indexers{
		usercache.ByUserIndexName: usercache.ByUserIndexKeys,
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

type KubeAPIServerInformers struct {
	KubernetesInformers        kexternalinformers.SharedInformerFactory
	OpenshiftOAuthInformers    oauthinformer.SharedInformerFactory
	OpenshiftQuotaInformers    quotainformer.SharedInformerFactory
	OpenshiftSecurityInformers securityv1informer.SharedInformerFactory
	OpenshiftUserInformers     userinformer.SharedInformerFactory
}

func (i *KubeAPIServerInformers) GetKubernetesInformers() kexternalinformers.SharedInformerFactory {
	return i.KubernetesInformers
}
func (i *KubeAPIServerInformers) GetOpenshiftQuotaInformers() quotainformer.SharedInformerFactory {
	return i.OpenshiftQuotaInformers
}
func (i *KubeAPIServerInformers) GetOpenshiftSecurityInformers() securityv1informer.SharedInformerFactory {
	return i.OpenshiftSecurityInformers
}
func (i *KubeAPIServerInformers) GetOpenshiftUserInformers() userinformer.SharedInformerFactory {
	return i.OpenshiftUserInformers
}

func (i *KubeAPIServerInformers) Start(stopCh <-chan struct{}) {
	i.KubernetesInformers.Start(stopCh)
	i.OpenshiftOAuthInformers.Start(stopCh)
	i.OpenshiftQuotaInformers.Start(stopCh)
	i.OpenshiftSecurityInformers.Start(stopCh)
	i.OpenshiftUserInformers.Start(stopCh)
}

func newClusterQuotaMappingController(nsInternalInformer corev1informers.NamespaceInformer, clusterQuotaInformer quotav1informer.ClusterResourceQuotaInformer) *clusterquotamapping.ClusterQuotaMappingController {
	return clusterquotamapping.NewClusterQuotaMappingController(nsInternalInformer, clusterQuotaInformer)
}
