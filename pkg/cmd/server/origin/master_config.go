package origin

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
	"strings"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	"k8s.io/apiserver/pkg/audit"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	"k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/request/websocket"
	x509request "k8s.io/apiserver/pkg/authentication/request/x509"
	"k8s.io/apiserver/pkg/authentication/user"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	authorizerunion "k8s.io/apiserver/pkg/authorization/union"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kubeclientgoinformers "k8s.io/client-go/informers"
	kubeclientgoclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	rbacinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/rbac/internalversion"
	rbaclisters "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
	"k8s.io/kubernetes/pkg/serviceaccount"
	saadmit "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
	storageclassdefaultadmission "k8s.io/kubernetes/plugin/pkg/admission/storageclass/setdefault"
	rbacauthorizer "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	"github.com/openshift/origin/pkg/auth/authenticator/request/paramtoken"
	authnregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	osclient "github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	admissionregistry "github.com/openshift/origin/pkg/cmd/server/origin/admission"
	originrest "github.com/openshift/origin/pkg/cmd/server/origin/rest"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imagepolicy "github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	ingressadmission "github.com/openshift/origin/pkg/ingress/admission"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	userinformer "github.com/openshift/origin/pkg/user/generated/informers/internalversion"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"

	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/service"
	serviceadmit "github.com/openshift/origin/pkg/service/admission"
	usercache "github.com/openshift/origin/pkg/user/cache"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	Options configapi.MasterConfig

	// RESTOptionsGetter provides access to storage and RESTOptions for a particular resource
	RESTOptionsGetter restoptions.Getter

	RuleResolver   rbacregistryvalidation.AuthorizationRuleResolver
	Authenticator  authenticator.Request
	Authorizer     kauthorizer.Authorizer
	SubjectLocator authorizer.SubjectLocator

	// TODO(sttts): replace AuthorizationAttributeBuilder with apiserverfilters.NewRequestAttributeGetter
	AuthorizationAttributeBuilder authorizer.AuthorizationAttributeBuilder

	ProjectAuthorizationCache     *projectauth.AuthorizationCache
	ProjectCache                  *projectcache.ProjectCache
	ClusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController
	LimitVerifier                 imageadmission.LimitVerifier

	// RequestContextMapper maps requests to contexts
	RequestContextMapper apirequest.RequestContextMapper

	AdmissionControl admission.Interface

	// KubeAdmissionControl holds the kube admission chain.  Because of the way the plugin initializer is built
	// you'll be passing information in this direction either way.  Knowing how to build this chain requires knowledge
	// of both the origin config AND the kube config, so this spot makes more sense.
	KubeAdmissionControl admission.Interface

	// RegistryNameFn retrieves the name of the integrated registry, or false if no such registry
	// is available.
	RegistryNameFn imageapi.DefaultRegistryFunc

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
	// PrivilegedLoopbackOpenShiftClient is the client used to call OpenShift APIs from system components,
	// built from PrivilegedLoopbackClientConfig. It should only be accessed via the *TestingClient() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically
	// for that component.
	PrivilegedLoopbackOpenShiftClient *osclient.Client

	AuditBackend audit.Backend

	// TODO inspect uses to eliminate them
	InternalKubeInformers  kinternalinformers.SharedInformerFactory
	ClientGoKubeInformers  kubeclientgoinformers.SharedInformerFactory
	AuthorizationInformers authorizationinformer.SharedInformerFactory
	QuotaInformers         quotainformer.SharedInformerFactory
	SecurityInformers      securityinformer.SharedInformerFactory
	UserInformers          userinformer.SharedInformerFactory
}

type InformerAccess interface {
	GetInternalKubeInformers() kinternalinformers.SharedInformerFactory
	GetExternalKubeInformers() kinformers.SharedInformerFactory
	GetClientGoKubeInformers() kubeclientgoinformers.SharedInformerFactory
	GetAuthorizationInformers() authorizationinformer.SharedInformerFactory
	GetImageInformers() imageinformer.SharedInformerFactory
	GetQuotaInformers() quotainformer.SharedInformerFactory
	GetSecurityInformers() securityinformer.SharedInformerFactory
	GetUserInformers() userinformer.SharedInformerFactory
}

// BuildMasterConfig builds and returns the OpenShift master configuration based on the
// provided options
func BuildMasterConfig(options configapi.MasterConfig, informers InformerAccess) (*MasterConfig, error) {
	restOptsGetter, err := originrest.StorageOptions(options)
	if err != nil {
		return nil, err
	}

	privilegedLoopbackKubeClientsetInternal, _, err := configapi.GetInternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	privilegedLoopbackKubeClientsetExternal, _, err := configapi.GetExternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	privilegedLoopbackOpenShiftClient, privilegedLoopbackClientConfig, err := configapi.GetOpenShiftClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	kubeClientGoClientSet, err := kubeclientgoclient.NewForConfig(privilegedLoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(privilegedLoopbackKubeClientsetInternal.Core().Services(metav1.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		return nil, fmt.Errorf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	requestContextMapper := apirequest.NewRequestContextMapper()

	projectCache := projectcache.NewProjectCache(
		informers.GetInternalKubeInformers().Core().InternalVersion().Namespaces().Informer(),
		privilegedLoopbackKubeClientsetInternal.Core().Namespaces(),
		options.ProjectConfig.DefaultNodeSelector)
	clusterQuotaMappingController := clusterquotamapping.NewClusterQuotaMappingControllerInternal(
		informers.GetInternalKubeInformers().Core().InternalVersion().Namespaces(),
		informers.GetQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas())

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)

	quotaRegistry := quota.NewAllResourceQuotaRegistryForAdmission(
		informers.GetExternalKubeInformers(),
		informers.GetImageInformers().Image().InternalVersion().ImageStreams(),
		privilegedLoopbackOpenShiftClient,
		privilegedLoopbackKubeClientsetExternal,
	)

	kubeAuthorizer, ruleResolver, kubeSubjectLocator := buildKubeAuth(informers.GetInternalKubeInformers().Rbac().InternalVersion())
	authorizer, subjectLocator := newAuthorizer(
		kubeAuthorizer,
		kubeSubjectLocator,
		informers.GetInternalKubeInformers().Rbac().InternalVersion().ClusterRoles().Lister(),
		options.ProjectConfig.ProjectRequestMessage,
	)

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
	genericInitializer, err := initializer.New(kubeClientGoClientSet, informers.GetClientGoKubeInformers(), authorizer)
	if err != nil {
		return nil, err
	}
	kubePluginInitializer := kadmission.NewPluginInitializer(
		privilegedLoopbackKubeClientsetInternal,
		privilegedLoopbackKubeClientsetExternal,
		informers.GetInternalKubeInformers(),
		authorizer,
		cloudConfig,
		// TODO: use a dynamic restmapper. See https://github.com/kubernetes/kubernetes/pull/42615.
		kapi.Registry.RESTMapper(),
		quotaRegistry)
	openshiftPluginInitializer := &oadmission.PluginInitializer{
		OpenshiftClient:              privilegedLoopbackOpenShiftClient,
		ProjectCache:                 projectCache,
		OriginQuotaRegistry:          quotaRegistry,
		Authorizer:                   authorizer,
		JenkinsPipelineConfig:        options.JenkinsPipelineConfig,
		RESTClientConfig:             *privilegedLoopbackClientConfig,
		Informers:                    informers.GetInternalKubeInformers(),
		ClusterResourceQuotaInformer: informers.GetQuotaInformers().Quota().InternalVersion().ClusterResourceQuotas(),
		ClusterQuotaMapper:           clusterQuotaMappingController.GetClusterQuotaMapper(),
		DefaultRegistryFn:            imageapi.DefaultRegistryFunc(defaultRegistryFunc),
		SecurityInformers:            informers.GetSecurityInformers(),
		UserInformers:                informers.GetUserInformers(),
	}
	initializersChain := admission.PluginInitializers{genericInitializer, kubePluginInitializer, openshiftPluginInitializer}

	originAdmission, kubeAdmission, err := buildAdmissionChains(options, privilegedLoopbackKubeClientsetInternal, initializersChain)
	if err != nil {
		return nil, err
	}

	// this is safe because the server does a quorum read and we're hitting a "magic" authorizer to get permissions based on system:masters
	// once the cache is added, we won't be paying a double hop cost to etcd on each request, so the simplification will help.
	serviceAccountTokenGetter := sacontroller.NewGetterFromClient(privilegedLoopbackKubeClientsetExternal)
	userClient, err := userclient.NewForConfig(privilegedLoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	oauthClient, err := oauthclient.NewForConfig(privilegedLoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	apiClientCAs, err := configapi.GetAPIClientCertCAPool(options)
	if err != nil {
		return nil, err
	}
	authenticator, err := newAuthenticator(options, oauthClient.OAuthAccessTokens(), serviceAccountTokenGetter, userClient.Users(), apiClientCAs, usercache.NewGroupCache(informers.GetUserInformers().User().InternalVersion().Groups()))
	if err != nil {
		return nil, err
	}

	config := &MasterConfig{
		Options: options,

		RESTOptionsGetter: restOptsGetter,

		RuleResolver:                  ruleResolver,
		Authenticator:                 authenticator,
		Authorizer:                    authorizer,
		SubjectLocator:                subjectLocator,
		AuthorizationAttributeBuilder: newAuthorizationAttributeBuilder(requestContextMapper),

		ProjectAuthorizationCache: newProjectAuthorizationCache(
			subjectLocator,
			informers.GetInternalKubeInformers().Core().InternalVersion().Namespaces().Informer(),
			informers.GetInternalKubeInformers().Rbac().InternalVersion(),
		),
		ProjectCache:                  projectCache,
		ClusterQuotaMappingController: clusterQuotaMappingController,

		RequestContextMapper: requestContextMapper,

		AdmissionControl:     originAdmission,
		KubeAdmissionControl: kubeAdmission,

		RegistryNameFn: imageapi.DefaultRegistryFunc(defaultRegistryFunc),

		KubeletClientConfig: kubeletClientConfig,

		PrivilegedLoopbackClientConfig:                *privilegedLoopbackClientConfig,
		PrivilegedLoopbackOpenShiftClient:             privilegedLoopbackOpenShiftClient,
		PrivilegedLoopbackKubernetesClientsetInternal: privilegedLoopbackKubeClientsetInternal,
		PrivilegedLoopbackKubernetesClientsetExternal: privilegedLoopbackKubeClientsetExternal,

		InternalKubeInformers:  informers.GetInternalKubeInformers(),
		ClientGoKubeInformers:  informers.GetClientGoKubeInformers(),
		AuthorizationInformers: informers.GetAuthorizationInformers(),
		QuotaInformers:         informers.GetQuotaInformers(),
		SecurityInformers:      informers.GetSecurityInformers(),
		UserInformers:          informers.GetUserInformers(),
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

var (
	// openshiftAdmissionControlPlugins gives the in-order default admission chain for openshift resources.
	openshiftAdmissionControlPlugins = []string{
		"ProjectRequestLimit",
		"OriginNamespaceLifecycle",
		"openshift.io/RestrictSubjectBindings",
		"PodNodeConstraints",
		"openshift.io/JenkinsBootstrapper",
		"openshift.io/BuildConfigSecretInjector",
		"BuildByStrategy",
		imageadmission.PluginName,
		"OwnerReferencesPermissionEnforcement",
		"Initializers",
		"GenericAdmissionWebhook",
		"ResourceQuota",
	}

	// KubeAdmissionPlugins gives the in-order default admission chain for kube resources.
	KubeAdmissionPlugins = []string{
		lifecycle.PluginName,
		"RunOnceDuration",
		"PodNodeConstraints",
		"OriginPodNodeEnvironment",
		"PodNodeSelector",
		overrideapi.PluginName,
		serviceadmit.ExternalIPPluginName,
		serviceadmit.RestrictedEndpointsPluginName,
		imagepolicy.PluginName,
		"ImagePolicyWebhook",
		"PodPreset",
		"LimitRanger",
		"ServiceAccount",
		"SecurityContextConstraint",
		storageclassdefaultadmission.PluginName,
		"AlwaysPullImages",
		"LimitPodHardAntiAffinityTopology",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"OwnerReferencesPermissionEnforcement",
		ingressadmission.IngressAdmission,
		"DefaultTolerationSeconds",
		"Initializers",
		"GenericAdmissionWebhook",
		"NodeRestriction",
		"PodTolerationRestriction",
		// NOTE: ResourceQuota and ClusterResourceQuota must be the last 2 plugins.
		// DO NOT ADD ANY PLUGINS AFTER THIS LINE!
		"ResourceQuota",
		"openshift.io/ClusterResourceQuota",
	}

	// CombinedAdmissionControlPlugins gives the in-order default admission chain for all resources resources.
	// When possible, this list is used.  The set of openshift+kube chains must exactly match this set.  In addition,
	// the order specified in the openshift and kube chains must match the order here.
	CombinedAdmissionControlPlugins = []string{
		lifecycle.PluginName,
		"ProjectRequestLimit",
		"OriginNamespaceLifecycle",
		"openshift.io/RestrictSubjectBindings",
		"PodNodeConstraints",
		"openshift.io/JenkinsBootstrapper",
		"openshift.io/BuildConfigSecretInjector",
		"BuildByStrategy",
		imageadmission.PluginName,
		"RunOnceDuration",
		"PodNodeConstraints",
		"OriginPodNodeEnvironment",
		"PodNodeSelector",
		overrideapi.PluginName,
		serviceadmit.ExternalIPPluginName,
		serviceadmit.RestrictedEndpointsPluginName,
		imagepolicy.PluginName,
		"ImagePolicyWebhook",
		"PodPreset",
		"LimitRanger",
		"ServiceAccount",
		"SecurityContextConstraint",
		storageclassdefaultadmission.PluginName,
		"AlwaysPullImages",
		"LimitPodHardAntiAffinityTopology",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"OwnerReferencesPermissionEnforcement",
		ingressadmission.IngressAdmission,
		"DefaultTolerationSeconds",
		"Initializers",
		"GenericAdmissionWebhook",
		"NodeRestriction",
		"PodTolerationRestriction",
		// NOTE: ResourceQuota and ClusterResourceQuota must be the last 2 plugins.
		// DO NOT ADD ANY PLUGINS AFTER THIS LINE!
		"ResourceQuota",
		"openshift.io/ClusterResourceQuota",
	}
)

// replace returns a slice where each instance of the input that is x is replaced with y
func replace(input []string, x, y string) []string {
	result := []string{}
	for i := range input {
		if input[i] == x {
			result = append(result, y)
		} else {
			result = append(result, input[i])
		}
	}
	return result
}

// dedupe removes duplicate items from the input list.
// the last instance of a duplicate is kept in the input list.
func dedupe(input []string) []string {
	items := sets.NewString()
	result := []string{}
	for i := len(input) - 1; i >= 0; i-- {
		if items.Has(input[i]) {
			continue
		}
		items.Insert(input[i])
		result = append([]string{input[i]}, result...)
	}
	return result
}

// fixupAdmissionPlugins fixes the input plugins to handle deprecation and duplicates.
func fixupAdmissionPlugins(plugins []string) []string {
	result := replace(plugins, "openshift.io/OriginResourceQuota", "ResourceQuota")
	result = dedupe(result)
	return result
}

func buildAdmissionChains(options configapi.MasterConfig, kubeClientSet kclientsetinternal.Interface, admissionInitializer admission.PluginInitializer) (admission.Interface /*origin*/, admission.Interface /*kube*/, error) {
	// check to see if they've taken explicit control of the kube admission chain
	// this happens when any of the following are true:
	// 1. extended kube server args are used to change the admission plugin list
	// 2. kube explicit config changes the admission plugin list
	// 3. extended kube server args are used to change the admission config file
	// 4. openshift explicit config changes the admission plugins list
	// 5. kube and openshift plugin config try to configure the same plugin differently
	// TODO: one release from now, I think we should start failing on setting the kube admission config
	//       two releases from now, I think we should start removing it
	//       two releases from now, I think we should remove the PluginOverrideOrder entirely
	hasSeparateKubeAdmissionChain := false
	KubeAdmissionPlugins := KubeAdmissionPlugins
	if options.KubernetesMasterConfig != nil && len(options.KubernetesMasterConfig.APIServerArguments["admission-control"]) > 0 {
		KubeAdmissionPlugins = strings.Split(options.KubernetesMasterConfig.APIServerArguments["admission-control"][0], ",")
		hasSeparateKubeAdmissionChain = true
	}
	if options.KubernetesMasterConfig != nil && len(options.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride) > 0 {
		KubeAdmissionPlugins = options.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride
		hasSeparateKubeAdmissionChain = true
	}
	KubeAdmissionPlugins = fixupAdmissionPlugins(KubeAdmissionPlugins)

	kubeAdmissionPluginConfigFilename := ""
	if options.KubernetesMasterConfig != nil && len(options.KubernetesMasterConfig.APIServerArguments["admission-control-config-file"]) > 0 {
		kubeAdmissionPluginConfigFilename = options.KubernetesMasterConfig.APIServerArguments["admission-control-config-file"][0]
		hasSeparateKubeAdmissionChain = true
	}

	openshiftAdmissionPlugins := openshiftAdmissionControlPlugins
	if len(options.AdmissionConfig.PluginOrderOverride) > 0 {
		openshiftAdmissionPlugins = options.AdmissionConfig.PluginOrderOverride
		hasSeparateKubeAdmissionChain = true
	}
	openshiftAdmissionPlugins = fixupAdmissionPlugins(openshiftAdmissionPlugins)

	if options.KubernetesMasterConfig != nil && !hasSeparateKubeAdmissionChain {
		// check for collisions between openshift and kube plugin config
		for pluginName, kubeConfig := range options.KubernetesMasterConfig.AdmissionConfig.PluginConfig {
			if openshiftConfig, exists := options.AdmissionConfig.PluginConfig[pluginName]; exists && !reflect.DeepEqual(kubeConfig, openshiftConfig) {
				hasSeparateKubeAdmissionChain = true
				break
			}
		}
	}

	if hasSeparateKubeAdmissionChain {
		// build kube admission
		var kubeAdmission admission.Interface
		if options.KubernetesMasterConfig != nil {
			var err error
			kubeAdmission, err = newAdmissionChainFunc(KubeAdmissionPlugins, kubeAdmissionPluginConfigFilename, options.KubernetesMasterConfig.AdmissionConfig.PluginConfig, options, kubeClientSet, admissionInitializer)
			if err != nil {
				return nil, nil, err
			}
		}

		// build openshift admission
		openshiftAdmission, err := newAdmissionChainFunc(openshiftAdmissionPlugins, "", options.AdmissionConfig.PluginConfig, options, kubeClientSet, admissionInitializer)
		if err != nil {
			return nil, nil, err
		}

		return openshiftAdmission, kubeAdmission, nil
	}

	// if we have a unified chain, build the combined config
	pluginConfig := map[string]configapi.AdmissionPluginConfig{}
	if options.KubernetesMasterConfig != nil {
		for pluginName, config := range options.KubernetesMasterConfig.AdmissionConfig.PluginConfig {
			pluginConfig[pluginName] = config
		}
	}
	for pluginName, config := range options.AdmissionConfig.PluginConfig {
		pluginConfig[pluginName] = config
	}

	admissionChain, err := newAdmissionChainFunc(CombinedAdmissionControlPlugins, "", pluginConfig, options, kubeClientSet, admissionInitializer)
	if err != nil {
		return nil, nil, err
	}

	return admissionChain, admissionChain, err
}

// newAdmissionChainFunc is for unit testing only.  You should NEVER OVERRIDE THIS outside of a unit test.
var newAdmissionChainFunc = newAdmissionChain

func newAdmissionChain(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet kclientsetinternal.Interface, admissionInitializer admission.PluginInitializer) (admission.Interface, error) {
	plugins := []admission.Interface{}
	for _, pluginName := range pluginNames {
		var (
			plugin             admission.Interface
			skipInitialization bool
		)

		switch pluginName {
		case lifecycle.PluginName:
			// We need to include our infrastructure and shared resource namespaces in the immortal namespaces list
			immortalNamespaces := sets.NewString(metav1.NamespaceDefault)
			if len(options.PolicyConfig.OpenShiftSharedResourcesNamespace) > 0 {
				immortalNamespaces.Insert(options.PolicyConfig.OpenShiftSharedResourcesNamespace)
			}
			if len(options.PolicyConfig.OpenShiftInfrastructureNamespace) > 0 {
				immortalNamespaces.Insert(options.PolicyConfig.OpenShiftInfrastructureNamespace)
			}
			lc, err := lifecycle.NewLifecycle(immortalNamespaces)
			if err != nil {
				return nil, err
			}
			admissionInitializer.Initialize(lc)
			if err := lc.(admission.Validator).Validate(); err != nil {
				return nil, err
			}
			plugin = lc

		case serviceadmit.ExternalIPPluginName:
			// this needs to be moved upstream to be part of core config
			reject, admit, err := serviceadmit.ParseRejectAdmitCIDRRules(options.NetworkConfig.ExternalIPNetworkCIDRs)
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			allowIngressIP := false
			if _, ipNet, err := net.ParseCIDR(options.NetworkConfig.IngressIPNetworkCIDR); err == nil && !ipNet.IP.IsUnspecified() {
				allowIngressIP = true
			}
			plugin = serviceadmit.NewExternalIPRanger(reject, admit, allowIngressIP)

		case serviceadmit.RestrictedEndpointsPluginName:
			// we need to set some customer parameters, so create by hand
			restrictedNetworks, err := serviceadmit.ParseSimpleCIDRRules([]string{options.NetworkConfig.ClusterNetworkCIDR, options.NetworkConfig.ServiceNetworkCIDR})
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			plugin = serviceadmit.NewRestrictedEndpointsAdmission(restrictedNetworks)

		case saadmit.PluginName:
			// we need to set some custom parameters on the service account admission controller, so create that one by hand
			saAdmitter := saadmit.NewServiceAccount()
			saAdmitter.SetInternalKubeClientSet(kubeClientSet)
			saAdmitter.LimitSecretReferences = options.ServiceAccountConfig.LimitSecretReferences
			plugin = saAdmitter

		default:
			configFile, err := pluginconfig.GetAdmissionConfigurationFile(pluginConfig, pluginName, admissionConfigFilename)
			if err != nil {
				return nil, err
			}
			configReader, err := admission.ReadAdmissionConfiguration([]string{pluginName}, configFile)
			if err != nil {
				return nil, err
			}
			pluginConfigReader, err := configReader.ConfigFor(pluginName)
			if err != nil {
				return nil, err
			}

			plugin, err = admissionregistry.OriginAdmissionPlugins.InitPlugin(pluginName, pluginConfigReader, admissionInitializer)
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			if plugin == nil {
				continue
			}

			// skip initialization below because admission.InitPlugin does all the work
			skipInitialization = true
		}

		plugins = append(plugins, plugin)

		if !skipInitialization {
			admissionInitializer.Initialize(plugin)
		}
	}

	// ensure that plugins have been properly initialized
	if err := oadmission.Validate(plugins); err != nil {
		return nil, err
	}

	return admission.NewChainHandler(plugins...), nil
}

func newAuthenticator(config configapi.MasterConfig, accessTokenGetter oauthclient.OAuthAccessTokenInterface, tokenGetter serviceaccount.ServiceAccountTokenGetter, userGetter userclient.UserResourceInterface, apiClientCAs *x509.CertPool, groupMapper identitymapper.UserToGroupMapper) (authenticator.Request, error) {
	authenticators := []authenticator.Request{}
	tokenAuthenticators := []authenticator.Request{}

	// ServiceAccount token
	if len(config.ServiceAccountConfig.PublicKeyFiles) > 0 {
		publicKeys := []interface{}{}
		for _, keyFile := range config.ServiceAccountConfig.PublicKeyFiles {
			readPublicKeys, err := serviceaccount.ReadPublicKeys(keyFile)
			if err != nil {
				return nil, fmt.Errorf("Error reading service account key file %s: %v", keyFile, err)
			}
			publicKeys = append(publicKeys, readPublicKeys...)
		}
		serviceAccountTokenAuthenticator := serviceaccount.JWTTokenAuthenticator(publicKeys, true, tokenGetter)
		tokenAuthenticators = append(
			tokenAuthenticators,
			bearertoken.New(serviceAccountTokenAuthenticator),
			websocket.NewProtocolAuthenticator(serviceAccountTokenAuthenticator),
			paramtoken.New("access_token", serviceAccountTokenAuthenticator, true),
		)
	}

	// OAuth token
	if config.OAuthConfig != nil {
		oauthTokenAuthenticator := authnregistry.NewTokenAuthenticator(accessTokenGetter, userGetter, groupMapper)
		oauthTokenRequestAuthenticators := []authenticator.Request{
			bearertoken.New(oauthTokenAuthenticator),
			websocket.NewProtocolAuthenticator(oauthTokenAuthenticator),
			paramtoken.New("access_token", oauthTokenAuthenticator, true),
		}

		tokenAuthenticators = append(tokenAuthenticators,
			// if you have a bearer token, you're a human (usually)
			// if you change this, have a look at the impersonationFilter where we attach groups to the impersonated user
			group.NewGroupAdder(union.New(oauthTokenRequestAuthenticators...), []string{bootstrappolicy.AuthenticatedOAuthGroup}))
	}

	if len(tokenAuthenticators) > 0 {
		authenticators = append(authenticators, union.New(tokenAuthenticators...))
	}

	// build cert authenticator
	// TODO: add "system:" prefix in authenticator, limit cert to username
	// TODO: add "system:" prefix to groups in authenticator, limit cert to group name
	opts := x509request.DefaultVerifyOptions()
	opts.Roots = apiClientCAs
	certauth := x509request.New(opts, x509request.CommonNameUserConversion)
	authenticators = append(authenticators, certauth)

	resultingAuthenticator := union.NewFailOnError(authenticators...)

	topLevelAuthenticators := []authenticator.Request{}
	// if we have a front proxy providing authentication configuration, wire it up and it should come first
	if config.AuthConfig.RequestHeader != nil {
		requestHeaderAuthenticator, err := headerrequest.NewSecure(
			config.AuthConfig.RequestHeader.ClientCA,
			config.AuthConfig.RequestHeader.ClientCommonNames,
			config.AuthConfig.RequestHeader.UsernameHeaders,
			config.AuthConfig.RequestHeader.GroupHeaders,
			config.AuthConfig.RequestHeader.ExtraHeaderPrefixes,
		)
		if err != nil {
			return nil, fmt.Errorf("Error building front proxy auth config: %v", err)
		}
		topLevelAuthenticators = append(topLevelAuthenticators, union.New(requestHeaderAuthenticator, resultingAuthenticator))

	} else {
		topLevelAuthenticators = append(topLevelAuthenticators, resultingAuthenticator)

	}
	topLevelAuthenticators = append(topLevelAuthenticators, anonymous.NewAuthenticator())

	return group.NewAuthenticatedGroupAdder(union.NewFailOnError(topLevelAuthenticators...)), nil
}

func newProjectAuthorizationCache(subjectLocator authorizer.SubjectLocator, namespaces cache.SharedIndexInformer, internalRBACInformers rbacinformers.Interface) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		namespaces,
		projectauth.NewAuthorizerReviewer(subjectLocator),
		internalRBACInformers,
	)
}

func buildKubeAuth(r rbacinformers.Interface) (kauthorizer.Authorizer, rbacregistryvalidation.AuthorizationRuleResolver, rbacauthorizer.SubjectLocator) {
	roles := &rbacauthorizer.RoleGetter{Lister: r.Roles().Lister()}
	roleBindings := &rbacauthorizer.RoleBindingLister{Lister: r.RoleBindings().Lister()}
	clusterRoles := &rbacauthorizer.ClusterRoleGetter{Lister: r.ClusterRoles().Lister()}
	clusterRoleBindings := &rbacauthorizer.ClusterRoleBindingLister{Lister: r.ClusterRoleBindings().Lister()}
	kubeAuthorizer := rbacauthorizer.New(roles, roleBindings, clusterRoles, clusterRoleBindings)
	ruleResolver := rbacregistryvalidation.NewDefaultRuleResolver(roles, roleBindings, clusterRoles, clusterRoleBindings)
	kubeSubjectLocator := rbacauthorizer.NewSubjectAccessEvaluator(roles, roleBindings, clusterRoles, clusterRoleBindings, "")
	return kubeAuthorizer, ruleResolver, kubeSubjectLocator
}

func newAuthorizer(kubeAuthorizer kauthorizer.Authorizer, kubeSubjectLocator rbacauthorizer.SubjectLocator, clusterRoleGetter rbaclisters.ClusterRoleLister, projectRequestDenyMessage string) (kauthorizer.Authorizer, authorizer.SubjectLocator) {
	messageMaker := authorizer.NewForbiddenMessageResolver(projectRequestDenyMessage)
	roleBasedAuthorizer := authorizer.NewAuthorizer(kubeAuthorizer, messageMaker)
	subjectLocator := authorizer.NewSubjectLocator(kubeSubjectLocator)
	scopeLimitedAuthorizer := scope.NewAuthorizer(roleBasedAuthorizer, clusterRoleGetter, messageMaker)

	authorizer := authorizerunion.New(
		authorizerfactory.NewPrivilegedGroups(user.SystemPrivilegedGroup), // authorizes system:masters to do anything, just like upstream
		scopeLimitedAuthorizer)

	return authorizer, subjectLocator
}

func newAuthorizationAttributeBuilder(requestContextMapper apirequest.RequestContextMapper) authorizer.AuthorizationAttributeBuilder {
	// Default API request info factory
	requestInfoFactory := &apirequest.RequestInfoFactory{APIPrefixes: sets.NewString("api", "osapi", "oapi", "apis"), GrouplessAPIPrefixes: sets.NewString("api", "osapi", "oapi")}
	// Wrap with a request info factory that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately
	browserSafeRequestInfoResolver := authorizer.NewBrowserSafeRequestInfoResolver(
		requestContextMapper,
		sets.NewString(bootstrappolicy.AuthenticatedGroup),
		requestInfoFactory,
	)
	personalSARRequestInfoResolver := authorizer.NewPersonalSARRequestInfoResolver(browserSafeRequestInfoResolver)
	projectRequestInfoResolver := authorizer.NewProjectRequestInfoResolver(personalSARRequestInfoResolver)

	authorizationAttributeBuilder := authorizer.NewAuthorizationAttributeBuilder(requestContextMapper, projectRequestInfoResolver)
	return authorizationAttributeBuilder
}

// KubeClientsetInternal returns the kubernetes client object
func (c *MasterConfig) KubeClientsetInternal() kclientsetinternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetInternal
}

// KubeClientsetInternal returns the kubernetes client object
func (c *MasterConfig) KubeClientsetExternal() kclientsetexternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetExternal
}

// ServiceAccountRoleBindingClient returns the client object used to bind roles to service accounts
// It must have the following capabilities:
//  get, list, update, create policyBindings and clusterPolicyBindings in all namespaces
func (c *MasterConfig) ServiceAccountRoleBindingClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// RouteAllocatorClients returns the route allocator client objects
func (c *MasterConfig) RouteAllocatorClients() (*osclient.Client, kclientsetinternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal
}

// WebConsoleEnabled says whether web ui is not a disabled feature and asset service is configured.
func (c *MasterConfig) WebConsoleEnabled() bool {
	return c.Options.AssetConfig != nil && !c.Options.DisabledFeatures.Has(configapi.FeatureWebConsole)
}

func (c *MasterConfig) WebConsoleStandalone() bool {
	return c.Options.AssetConfig.ServingInfo.BindAddress != c.Options.ServingInfo.BindAddress
}
