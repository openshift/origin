package origin

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/admission/namespaceconditions"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	admissionmetrics "k8s.io/apiserver/pkg/admission/metrics"
	"k8s.io/apiserver/pkg/audit"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kinformers "k8s.io/client-go/informers"
	kubeclientgoinformers "k8s.io/client-go/informers"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	rbacinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/rbac/internalversion"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	kubeapiserver "k8s.io/kubernetes/pkg/master"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
	rbacauthorizer "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	appinformer "github.com/openshift/origin/pkg/apps/generated/informers/internalversion"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	buildinformer "github.com/openshift/origin/pkg/build/generated/informers/internalversion"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	originadmission "github.com/openshift/origin/pkg/cmd/server/origin/admission"
	originrest "github.com/openshift/origin/pkg/cmd/server/origin/rest"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	networkinformer "github.com/openshift/origin/pkg/network/generated/informers/internalversion"
	oauthinformer "github.com/openshift/origin/pkg/oauth/generated/informers/internalversion"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"

	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/service"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	Options configapi.MasterConfig

	kubeAPIServerConfig      *kubeapiserver.Config
	additionalPostStartHooks map[string]genericapiserver.PostStartHookFunc

	// RESTOptionsGetter provides access to storage and RESTOptions for a particular resource
	RESTOptionsGetter restoptions.Getter

	RuleResolver   rbacregistryvalidation.AuthorizationRuleResolver
	SubjectLocator rbacauthorizer.SubjectLocator

	ProjectAuthorizationCache     *projectauth.AuthorizationCache
	ProjectCache                  *projectcache.ProjectCache
	ClusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController
	LimitVerifier                 imageadmission.LimitVerifier

	// RegistryHostnameRetriever retrieves the name of the integrated registry, or false if no such registry
	// is available.
	RegistryHostnameRetriever imageapi.RegistryHostnameRetriever

	KubeletClientConfig *kubeletclient.KubeletClientConfig

	// PrivilegedLoopbackClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	PrivilegedLoopbackClientConfig restclient.Config

	// PrivilegedLoopbackKubernetesClientsetInternal is the client used to call Kubernetes APIs from system components,
	// built from KubeClientConfig. It should only be accessed via the *TestingClient() helper methods. To apply
	// different access control to a system component, create a separate client/config specifically for
	// that component.
	PrivilegedLoopbackKubernetesClientsetInternal kclientsetinternal.Interface
	// PrivilegedLoopbackKubernetesClientsetExternal is the client used to call Kubernetes APIs from system components,
	// built from KubeClientConfig. It should only be accessed via the *TestingClient() helper methods. To apply
	// different access control to a system component, create a separate client/config specifically for
	// that component.
	PrivilegedLoopbackKubernetesClientsetExternal kclientsetexternal.Interface

	AuditBackend audit.Backend

	// TODO inspect uses to eliminate them
	InternalKubeInformers  kinternalinformers.SharedInformerFactory
	ClientGoKubeInformers  kubeclientgoinformers.SharedInformerFactory
	AuthorizationInformers authorizationinformer.SharedInformerFactory
	QuotaInformers         quotainformer.SharedInformerFactory
	SecurityInformers      securityinformer.SharedInformerFactory
}

type InformerAccess interface {
	GetInternalKubeInformers() kinternalinformers.SharedInformerFactory
	GetExternalKubeInformers() kinformers.SharedInformerFactory
	GetClientGoKubeInformers() kubeclientgoinformers.SharedInformerFactory
	GetAppInformers() appinformer.SharedInformerFactory
	GetAuthorizationInformers() authorizationinformer.SharedInformerFactory
	GetBuildInformers() buildinformer.SharedInformerFactory
	GetImageInformers() imageinformer.SharedInformerFactory
	GetNetworkInformers() networkinformer.SharedInformerFactory
	GetOauthInformers() oauthinformer.SharedInformerFactory
	GetQuotaInformers() quotainformer.SharedInformerFactory
	GetSecurityInformers() securityinformer.SharedInformerFactory
	GetUserInformers() userinformer.SharedInformerFactory
	GetTemplateInformers() templateinformer.SharedInformerFactory
	ToGenericInformer() GenericResourceInformer

	Start(stopCh <-chan struct{})
}

// BuildMasterConfig builds and returns the OpenShift master configuration based on the
// provided options
func BuildMasterConfig(
	options configapi.MasterConfig,
	informers InformerAccess,
) (*MasterConfig, error) {
	restOptsGetter, err := originrest.StorageOptions(options)
	if err != nil {
		return nil, err
	}

	kubeInternalClient, _, err := configapi.GetInternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	privilegedLoopbackKubeClientsetExternal, privilegedLoopbackConfig, err := configapi.GetExternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}

	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(kubeInternalClient.Core().Services(metav1.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		return nil, fmt.Errorf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)

	authenticator, authenticatorPostStartHooks, err := NewAuthenticator(options, privilegedLoopbackConfig, informers)
	if err != nil {
		return nil, err
	}
	authorizer := NewAuthorizer(informers, options.ProjectConfig.ProjectRequestMessage)
	projectCache, err := newProjectCache(informers, privilegedLoopbackConfig, options.ProjectConfig.DefaultNodeSelector)
	if err != nil {
		return nil, err
	}
	clusterQuotaMappingController := newClusterQuotaMappingController(informers)
	admissionInitializer, admissionPostStartHook, err := originadmission.NewPluginInitializer(options, privilegedLoopbackConfig, informers, authorizer, projectCache, clusterQuotaMappingController)
	if err != nil {
		return nil, err
	}
	admissionDecorator := namespaceconditions.NamespaceLabelConditions{
		Delegate: admissionmetrics.WithControllerMetrics,

		NamespaceClient: privilegedLoopbackKubeClientsetExternal.CoreV1(),
		NamespaceLister: informers.GetExternalKubeInformers().Core().V1().Namespaces().Lister(),

		SkipLevelZeroNames: originadmission.SkipRunLevelZeroPlugins,
		SkipLevelOneNames:  originadmission.SkipRunLevelOnePlugins,
	}
	admission, err := originadmission.NewAdmissionChains(options, admissionInitializer, admissionDecorator.WithNamespaceLabelConditions)
	if err != nil {
		return nil, err
	}

	kubeAPIServerConfig, err := kubernetes.BuildKubernetesMasterConfig(
		options,
		admission,
		authenticator,
		authorizer,
	)
	if err != nil {
		return nil, err
	}

	subjectLocator := NewSubjectLocator(informers.GetInternalKubeInformers().Rbac().InternalVersion())

	config := &MasterConfig{
		Options: options,

		kubeAPIServerConfig: kubeAPIServerConfig,
		additionalPostStartHooks: map[string]genericapiserver.PostStartHookFunc{
			"openshift.io-AdmissionInit": admissionPostStartHook,
			"openshift.io-StartInformers": func(context genericapiserver.PostStartHookContext) error {
				informers.Start(context.StopCh)
				return nil
			},
		},

		RESTOptionsGetter: restOptsGetter,

		RuleResolver:   NewRuleResolver(informers.GetInternalKubeInformers().Rbac().InternalVersion()),
		SubjectLocator: subjectLocator,

		ProjectAuthorizationCache: newProjectAuthorizationCache(
			subjectLocator,
			informers.GetInternalKubeInformers().Core().InternalVersion().Namespaces().Informer(),
			informers.GetInternalKubeInformers().Rbac().InternalVersion(),
		),
		ProjectCache:                  projectCache,
		ClusterQuotaMappingController: clusterQuotaMappingController,

		RegistryHostnameRetriever: imageapi.DefaultRegistryHostnameRetriever(defaultRegistryFunc, options.ImagePolicyConfig.ExternalRegistryHostname, options.ImagePolicyConfig.InternalRegistryHostname),

		KubeletClientConfig: kubeletClientConfig,

		PrivilegedLoopbackClientConfig:                *privilegedLoopbackConfig,
		PrivilegedLoopbackKubernetesClientsetInternal: kubeInternalClient,
		PrivilegedLoopbackKubernetesClientsetExternal: privilegedLoopbackKubeClientsetExternal,

		InternalKubeInformers:  informers.GetInternalKubeInformers(),
		ClientGoKubeInformers:  informers.GetClientGoKubeInformers(),
		AuthorizationInformers: informers.GetAuthorizationInformers(),
		QuotaInformers:         informers.GetQuotaInformers(),
		SecurityInformers:      informers.GetSecurityInformers(),
	}

	for name, hook := range authenticatorPostStartHooks {
		config.additionalPostStartHooks[name] = hook
	}

	// ensure that the limit range informer will be started
	informer := config.InternalKubeInformers.Core().InternalVersion().LimitRanges().Informer()
	config.LimitVerifier = imageadmission.NewLimitVerifier(imageadmission.LimitRangesForNamespaceFunc(func(ns string) ([]*kapi.LimitRange, error) {
		list, err := config.InternalKubeInformers.Core().InternalVersion().LimitRanges().Lister().LimitRanges(ns).List(labels.Everything())
		if err != nil {
			return nil, err
		}
		// the verifier must return an error
		if len(list) == 0 && len(informer.LastSyncResourceVersion()) == 0 {
			glog.V(4).Infof("LimitVerifier still waiting for ranges to load: %#v", informer)
			forbiddenErr := kapierrors.NewForbidden(schema.GroupResource{Resource: "limitranges"}, "", fmt.Errorf("the server is still loading limit information"))
			forbiddenErr.ErrStatus.Details.RetryAfterSeconds = 1
			return nil, forbiddenErr
		}
		return list, nil
	}))

	return config, nil
}

func newClusterQuotaMappingController(informers InformerAccess) *clusterquotamapping.ClusterQuotaMappingController {
	return clusterquotamapping.NewClusterQuotaMappingControllerInternal(
		informers.GetInternalKubeInformers().Core().InternalVersion().Namespaces(),
		informers.GetQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas())
}

func newProjectCache(informers InformerAccess, privilegedLoopbackConfig *restclient.Config, defaultNodeSelector string) (*projectcache.ProjectCache, error) {
	kubeInternalClient, err := kclientsetinternal.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}
	return projectcache.NewProjectCache(
		informers.GetInternalKubeInformers().Core().InternalVersion().Namespaces().Informer(),
		kubeInternalClient.Core().Namespaces(),
		defaultNodeSelector), nil
}

func newProjectAuthorizationCache(subjectLocator rbacauthorizer.SubjectLocator, namespaces cache.SharedIndexInformer, internalRBACInformers rbacinformers.Interface) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		namespaces,
		projectauth.NewAuthorizerReviewer(subjectLocator),
		internalRBACInformers,
	)
}
