package origin

import (
	"github.com/openshift/origin/pkg/admission/namespaceconditions"
	usercache "github.com/openshift/origin/pkg/user/cache"
	"k8s.io/client-go/restmapper"

	"k8s.io/apiserver/pkg/admission"
	admissionmetrics "k8s.io/apiserver/pkg/admission/metrics"
	"k8s.io/apiserver/pkg/audit"
	genericapiserver "k8s.io/apiserver/pkg/server"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	kinformers "k8s.io/client-go/informers"
	kubeclientgoinformers "k8s.io/client-go/informers"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kubeapiserver "k8s.io/kubernetes/pkg/master"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
	rbacauthorizer "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	imagev1informer "github.com/openshift/client-go/image/informers/externalversions"
	oauthv1informer "github.com/openshift/client-go/oauth/informers/externalversions"
	routev1informer "github.com/openshift/client-go/route/informers/externalversions"
	userv1informer "github.com/openshift/client-go/user/informers/externalversions"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/openshiftkubeapiserver"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	originadmission "github.com/openshift/origin/pkg/cmd/server/origin/admission"
	imageadmission "github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	_ "github.com/openshift/origin/pkg/printers/internalversion"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"

	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	Options configapi.MasterConfig

	// we phrase it like this so we can build the post-start-hook, but no one can take more indirect dependencies on informers
	InformerStart            func(stopCh <-chan struct{})
	kubeAPIServerConfig      *kubeapiserver.Config
	additionalPostStartHooks map[string]genericapiserver.PostStartHookFunc

	RuleResolver   rbacregistryvalidation.AuthorizationRuleResolver
	SubjectLocator rbacauthorizer.SubjectLocator

	ProjectAuthorizationCache     *projectauth.AuthorizationCache
	ProjectCache                  *projectcache.ProjectCache
	ClusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController
	LimitVerifier                 imageadmission.LimitVerifier
	RESTMapper                    *restmapper.DeferredDiscoveryRESTMapper

	// RegistryHostnameRetriever retrieves the name of the integrated registry, or false if no such registry
	// is available.
	RegistryHostnameRetriever registryhostname.RegistryHostnameRetriever

	// PrivilegedLoopbackClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	PrivilegedLoopbackClientConfig restclient.Config

	AuditBackend audit.Backend

	// TODO inspect uses to eliminate them
	InternalKubeInformers  kinternalinformers.SharedInformerFactory
	ClientGoKubeInformers  kubeclientgoinformers.SharedInformerFactory
	AuthorizationInformers authorizationinformer.SharedInformerFactory
	RouteInformers         routev1informer.SharedInformerFactory
	QuotaInformers         quotainformer.SharedInformerFactory
	SecurityInformers      securityinformer.SharedInformerFactory
}

type InformerAccess interface {
	GetInternalKubernetesInformers() kinternalinformers.SharedInformerFactory
	GetKubernetesInformers() kinformers.SharedInformerFactory

	GetOpenshiftImageInformers() imagev1informer.SharedInformerFactory
	GetOpenshiftOauthInformers() oauthv1informer.SharedInformerFactory
	GetOpenshiftRouteInformers() routev1informer.SharedInformerFactory
	GetOpenshiftUserInformers() userv1informer.SharedInformerFactory

	GetInternalOpenshiftAuthorizationInformers() authorizationinformer.SharedInformerFactory
	GetInternalOpenshiftQuotaInformers() quotainformer.SharedInformerFactory
	GetInternalOpenshiftSecurityInformers() securityinformer.SharedInformerFactory

	Start(stopCh <-chan struct{})
}

// BuildMasterConfig builds and returns the OpenShift master configuration based on the
// provided options
func BuildMasterConfig(
	options configapi.MasterConfig,
	informers InformerAccess,
) (*MasterConfig, error) {
	incompleteKubeAPIServerConfig, err := kubernetes.BuildKubernetesMasterConfig(options)
	if err != nil {
		return nil, err
	}
	if informers == nil {
		// use the real Kubernetes loopback client (using a secret token and preferibly localhost networking), not
		// the one provided by options.MasterClients.OpenShiftLoopbackKubeConfig. The latter is meant for out-of-process
		// components of the master.
		realLoopbackInformers, err := NewInformers(incompleteKubeAPIServerConfig.LoopbackConfig())
		if err != nil {
			return nil, err
		}
		if err := realLoopbackInformers.GetOpenshiftUserInformers().User().V1().Groups().Informer().AddIndexers(cache.Indexers{
			usercache.ByUserIndexName: usercache.ByUserIndexKeys,
		}); err != nil {
			return nil, err
		}
		informers = realLoopbackInformers
	}

	privilegedLoopbackConfig, err := configapi.GetClientConfig(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	privilegedLoopbackKubeClientsetExternal, err := kclientsetexternal.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}

	registryHostnameRetriever, err := registryhostname.DefaultRegistryHostnameRetriever(privilegedLoopbackConfig, options.ImagePolicyConfig.ExternalRegistryHostname, options.ImagePolicyConfig.InternalRegistryHostname)
	if err != nil {
		return nil, err
	}

	authenticator, authenticatorPostStartHooks, err := openshiftkubeapiserver.NewAuthenticator(
		options.ServingInfo.ServingInfo,
		options.ServiceAccountConfig.PublicKeyFiles, options.OAuthConfig, options.AuthConfig,
		privilegedLoopbackConfig, informers.GetOpenshiftOauthInformers().Oauth().V1().OAuthClients().Lister(), informers.GetOpenshiftUserInformers().User().V1().Groups())
	if err != nil {
		return nil, err
	}
	authorizer := openshiftkubeapiserver.NewAuthorizer(informers.GetInternalKubernetesInformers(), informers.GetKubernetesInformers())
	projectCache, err := openshiftapiserver.NewProjectCache(informers.GetInternalKubernetesInformers().Core().InternalVersion().Namespaces(), privilegedLoopbackConfig, options.ProjectConfig.DefaultNodeSelector)
	if err != nil {
		return nil, err
	}
	clusterQuotaMappingController := openshiftapiserver.NewClusterQuotaMappingController(informers.GetInternalKubernetesInformers().Core().InternalVersion().Namespaces(), informers.GetInternalOpenshiftQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas())
	discoveryClient := cacheddiscovery.NewMemCacheClient(privilegedLoopbackKubeClientsetExternal.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	cloudConfigFile, err := configprocessing.GetCloudProviderConfigFile(options.KubernetesMasterConfig.APIServerArguments)
	if err != nil {
		return nil, err
	}
	admissionInitializer, err := originadmission.NewPluginInitializer(options.ImagePolicyConfig.ExternalRegistryHostname, options.ImagePolicyConfig.InternalRegistryHostname, cloudConfigFile, options.JenkinsPipelineConfig, privilegedLoopbackConfig, informers, authorizer, projectCache, restMapper, clusterQuotaMappingController)
	if err != nil {
		return nil, err
	}
	namespaceLabelDecorator := namespaceconditions.NamespaceLabelConditions{
		NamespaceClient: privilegedLoopbackKubeClientsetExternal.CoreV1(),
		NamespaceLister: informers.GetKubernetesInformers().Core().V1().Namespaces().Lister(),

		SkipLevelZeroNames: originadmission.SkipRunLevelZeroPlugins,
		SkipLevelOneNames:  originadmission.SkipRunLevelOnePlugins,
	}
	admissionDecorators := admission.Decorators{
		admission.DecoratorFunc(namespaceLabelDecorator.WithNamespaceLabelConditions),
		admission.DecoratorFunc(admissionmetrics.WithControllerMetrics),
	}
	admission, err := originadmission.NewAdmissionChains(options.KubernetesMasterConfig.APIServerArguments["admission-control-config-file"], options.AdmissionConfig.PluginConfig, options.AdmissionConfig.PluginOrderOverride, admissionInitializer, admissionDecorators)
	if err != nil {
		return nil, err
	}

	kubeAPIServerConfig, err := incompleteKubeAPIServerConfig.Complete(
		admission,
		authenticator,
		authorizer,
	)
	if err != nil {
		return nil, err
	}

	subjectLocator := openshiftapiserver.NewSubjectLocator(informers.GetKubernetesInformers().Rbac().V1())

	config := &MasterConfig{
		Options: options,

		InformerStart:            informers.Start,
		kubeAPIServerConfig:      kubeAPIServerConfig,
		additionalPostStartHooks: map[string]genericapiserver.PostStartHookFunc{},

		RuleResolver:   openshiftapiserver.NewRuleResolver(informers.GetKubernetesInformers().Rbac().V1()),
		SubjectLocator: subjectLocator,

		ProjectAuthorizationCache: openshiftapiserver.NewProjectAuthorizationCache(
			subjectLocator,
			informers.GetInternalKubernetesInformers().Core().InternalVersion().Namespaces().Informer(),
			informers.GetKubernetesInformers().Rbac().V1(),
		),
		ProjectCache:                  projectCache,
		ClusterQuotaMappingController: clusterQuotaMappingController,
		RESTMapper:                    restMapper,

		RegistryHostnameRetriever: registryHostnameRetriever,

		PrivilegedLoopbackClientConfig: *privilegedLoopbackConfig,

		InternalKubeInformers:  informers.GetInternalKubernetesInformers(),
		ClientGoKubeInformers:  informers.GetKubernetesInformers(),
		AuthorizationInformers: informers.GetInternalOpenshiftAuthorizationInformers(),
		QuotaInformers:         informers.GetInternalOpenshiftQuotaInformers(),
		SecurityInformers:      informers.GetInternalOpenshiftSecurityInformers(),
		RouteInformers:         informers.GetOpenshiftRouteInformers(),
	}

	for name, hook := range authenticatorPostStartHooks {
		config.additionalPostStartHooks[name] = hook
	}

	// ensure that the limit range informer will be started
	config.LimitVerifier = openshiftapiserver.ImageLimitVerifier(config.InternalKubeInformers.Core().InternalVersion().LimitRanges())

	return config, nil
}
