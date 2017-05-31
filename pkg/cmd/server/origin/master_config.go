package origin

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	"k8s.io/apiserver/pkg/authentication/request/union"
	x509request "k8s.io/apiserver/pkg/authentication/request/x509"
	"k8s.io/apiserver/pkg/authentication/user"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	authorizerunion "k8s.io/apiserver/pkg/authorization/union"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kextensionsclient "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/extensions/v1beta1"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/plugin/pkg/admission/namespace/lifecycle"
	saadmit "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
	storageclassdefaultadmission "k8s.io/kubernetes/plugin/pkg/admission/storageclass/default"

	"github.com/openshift/origin/pkg/auth/authenticator/request/paramtoken"
	authnregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicyetcd "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy/etcd"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	clusterpolicybindingetcd "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding/etcd"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policyetcd "github.com/openshift/origin/pkg/authorization/registry/policy/etcd"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	policybindingetcd "github.com/openshift/origin/pkg/authorization/registry/policybinding/etcd"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	osclient "github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	originrest "github.com/openshift/origin/pkg/cmd/server/origin/rest"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/plug"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/controller"
	"github.com/openshift/origin/pkg/controller/shared"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imagepolicy "github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	ingressadmission "github.com/openshift/origin/pkg/ingress/admission"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	"github.com/openshift/origin/pkg/service"
	serviceadmit "github.com/openshift/origin/pkg/service/admission"
	"github.com/openshift/origin/pkg/serviceaccounts"
	templateapi "github.com/openshift/origin/pkg/template/api"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	usercache "github.com/openshift/origin/pkg/user/cache"
	groupregistry "github.com/openshift/origin/pkg/user/registry/group"
	groupstorage "github.com/openshift/origin/pkg/user/registry/group/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	Options configapi.MasterConfig

	// RESTOptionsGetter provides access to storage and RESTOptions for a particular resource
	RESTOptionsGetter restoptions.Getter

	RuleResolver   rulevalidation.AuthorizationRuleResolver
	Authenticator  authenticator.Request
	Authorizer     kauthorizer.Authorizer
	SubjectLocator authorizer.SubjectLocator

	// TODO(sttts): replace AuthorizationAttributeBuilder with apiserverfilters.NewRequestAttributeGetter
	AuthorizationAttributeBuilder authorizer.AuthorizationAttributeBuilder

	GroupCache                    *usercache.GroupCache
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

	TLS bool

	ControllerPlug      plug.Plug
	ControllerPlugStart func()

	// ImageFor is a function that returns the appropriate image to use for a named component
	ImageFor func(component string) string
	// RegistryNameFn retrieves the name of the integrated registry, or false if no such registry
	// is available.
	RegistryNameFn imageapi.DefaultRegistryFunc

	// ExternalVersionCodec is the codec used when serializing annotations, which cannot be changed
	// without all clients being aware of the new version.
	ExternalVersionCodec runtime.Codec

	KubeletClientConfig *kubeletclient.KubeletClientConfig

	// ClientCAs will be used to request client certificates in connections to the API.
	// This CertPool should contain all the CAs that will be used for client certificate verification.
	ClientCAs *x509.CertPool
	// APIClientCAs is used to verify client certificates presented for API auth
	APIClientCAs *x509.CertPool

	// PrivilegedLoopbackClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	PrivilegedLoopbackClientConfig restclient.Config

	// PrivilegedLoopbackKubernetesClientsetInternal is the client used to call Kubernetes APIs from system components,
	// built from KubeClientConfig. It should only be accessed via the *Client() helper methods. To apply
	// different access control to a system component, create a separate client/config specifically for
	// that component.
	PrivilegedLoopbackKubernetesClientsetInternal kclientsetinternal.Interface
	// PrivilegedLoopbackKubernetesClientsetExternal is the client used to call Kubernetes APIs from system components,
	// built from KubeClientConfig. It should only be accessed via the *Client() helper methods. To apply
	// different access control to a system component, create a separate client/config specifically for
	// that component.
	PrivilegedLoopbackKubernetesClientsetExternal kclientsetexternal.Interface
	// PrivilegedLoopbackOpenShiftClient is the client used to call OpenShift APIs from system components,
	// built from PrivilegedLoopbackClientConfig. It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically
	// for that component.
	PrivilegedLoopbackOpenShiftClient *osclient.Client

	// Informers is a shared factory for getting SharedInformers. It is important to get your informers, indexers, and listers
	// from here so that we only end up with a single cache of objects
	Informers shared.InformerFactory

	AuthorizationInformers authorizationinformer.SharedInformerFactory
	TemplateInformers      templateinformer.SharedInformerFactory
}

// BuildMasterConfig builds and returns the OpenShift master configuration based on the
// provided options
func BuildMasterConfig(options configapi.MasterConfig) (*MasterConfig, error) {
	restOptsGetter, err := originrest.StorageOptions(options)
	if err != nil {
		return nil, err
	}

	clientCAs, err := configapi.GetClientCertCAPool(options)
	if err != nil {
		return nil, err
	}
	apiClientCAs, err := configapi.GetAPIClientCertCAPool(options)
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
	authorizationClient, err := authorizationclient.NewForConfig(privilegedLoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	templateClient, err := templateclient.NewForConfig(privilegedLoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	customListerWatchers := shared.DefaultListerWatcherOverrides{}
	if err := addAuthorizationListerWatchers(customListerWatchers, restOptsGetter); err != nil {
		return nil, err
	}
	// TODO find a single place to create and start informers.  During the 1.7 rebase this will come more naturally in a config object,
	// before then we should try to eliminate our direct to storage access.  It's making us do weird things.
	const defaultInformerResyncPeriod = 10 * time.Minute
	internalkubeInformerFactory := kinternalinformers.NewSharedInformerFactory(privilegedLoopbackKubeClientsetInternal, defaultInformerResyncPeriod)
	externalkubeInformerFactory := kinformers.NewSharedInformerFactory(privilegedLoopbackKubeClientsetExternal, defaultInformerResyncPeriod)
	authorizationInformers := authorizationinformer.NewSharedInformerFactory(authorizationClient, defaultInformerResyncPeriod)
	templateInformers := templateinformer.NewSharedInformerFactory(templateClient, defaultInformerResyncPeriod)
	informerFactory := shared.NewInformerFactory(internalkubeInformerFactory, externalkubeInformerFactory, privilegedLoopbackKubeClientsetInternal, privilegedLoopbackOpenShiftClient, customListerWatchers, defaultInformerResyncPeriod)

	err = templateInformers.Template().InternalVersion().Templates().Informer().AddIndexers(cache.Indexers{
		templateapi.TemplateUIDIndex: func(obj interface{}) ([]string, error) {
			return []string{string(obj.(*templateapi.Template).UID)}, nil
		},
	})
	if err != nil {
		return nil, err
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(privilegedLoopbackKubeClientsetInternal.Core().Services(metav1.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		return nil, fmt.Errorf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	requestContextMapper := apirequest.NewRequestContextMapper()

	groupStorage, err := groupstorage.NewREST(restOptsGetter)
	if err != nil {
		return nil, err
	}
	groupCache := usercache.NewGroupCache(groupregistry.NewRegistry(groupStorage))
	projectCache := projectcache.NewProjectCache(informerFactory.InternalKubernetesInformers().Core().InternalVersion().Namespaces().Informer(), privilegedLoopbackKubeClientsetInternal.Core().Namespaces(), options.ProjectConfig.DefaultNodeSelector)
	clusterQuotaMappingController := clusterquotamapping.NewClusterQuotaMappingController(internalkubeInformerFactory.Core().InternalVersion().Namespaces(), informerFactory.ClusterResourceQuotas())

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)

	quotaRegistry := quota.NewAllResourceQuotaRegistryForAdmission(informerFactory, privilegedLoopbackOpenShiftClient, privilegedLoopbackKubeClientsetExternal)
	ruleResolver := rulevalidation.NewDefaultRuleResolver(
		informerFactory.Policies().Lister(),
		informerFactory.PolicyBindings().Lister(),
		informerFactory.ClusterPolicies().Lister().ClusterPolicies(),
		informerFactory.ClusterPolicyBindings().Lister().ClusterPolicyBindings(),
	)
	authorizer, subjectLocator := newAuthorizer(ruleResolver, informerFactory, options.ProjectConfig.ProjectRequestMessage)

	pluginInitializer := oadmission.PluginInitializer{
		OpenshiftClient:       privilegedLoopbackOpenShiftClient,
		ProjectCache:          projectCache,
		OriginQuotaRegistry:   quotaRegistry,
		Authorizer:            authorizer,
		JenkinsPipelineConfig: options.JenkinsPipelineConfig,
		RESTClientConfig:      *privilegedLoopbackClientConfig,
		Informers:             informerFactory,
		ClusterQuotaMapper:    clusterQuotaMappingController.GetClusterQuotaMapper(),
		DefaultRegistryFn:     imageapi.DefaultRegistryFunc(defaultRegistryFunc),
		GroupCache:            groupCache,
	}

	// TODO if we want to support WantsAuthorizer, we need to pass in a kube
	// Authorizer as the 2nd arg. It's currently only used by PSP.
	// note: we are passing a combined quota registry here...
	kubePluginInitializer := kadmission.NewPluginInitializer(privilegedLoopbackKubeClientsetInternal, internalkubeInformerFactory, nil, nil, quotaRegistry)
	originAdmission, kubeAdmission, err := buildAdmissionChains(options, privilegedLoopbackKubeClientsetInternal, pluginInitializer, kubePluginInitializer)
	if err != nil {
		return nil, err
	}

	serviceAccountTokenGetter, err := newServiceAccountTokenGetter(options)
	if err != nil {
		return nil, err
	}

	authenticator, err := newAuthenticator(options, restOptsGetter, serviceAccountTokenGetter, apiClientCAs, groupCache)
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

		GroupCache:                    groupCache,
		ProjectAuthorizationCache:     newProjectAuthorizationCache(subjectLocator, privilegedLoopbackKubeClientsetInternal, informerFactory),
		ProjectCache:                  projectCache,
		ClusterQuotaMappingController: clusterQuotaMappingController,

		RequestContextMapper: requestContextMapper,

		AdmissionControl:     originAdmission,
		KubeAdmissionControl: kubeAdmission,

		TLS: configapi.UseTLS(options.ServingInfo.ServingInfo),

		ImageFor:       imageTemplate.ExpandOrDie,
		RegistryNameFn: imageapi.DefaultRegistryFunc(defaultRegistryFunc),

		// TODO: migration of versions of resources stored in annotations must be sorted out
		ExternalVersionCodec: kapi.Codecs.LegacyCodec(schema.GroupVersion{Group: "", Version: "v1"}),

		KubeletClientConfig: kubeletClientConfig,

		ClientCAs:    clientCAs,
		APIClientCAs: apiClientCAs,

		PrivilegedLoopbackClientConfig:                *privilegedLoopbackClientConfig,
		PrivilegedLoopbackOpenShiftClient:             privilegedLoopbackOpenShiftClient,
		PrivilegedLoopbackKubernetesClientsetInternal: privilegedLoopbackKubeClientsetInternal,
		PrivilegedLoopbackKubernetesClientsetExternal: privilegedLoopbackKubeClientsetExternal,
		Informers:              informerFactory,
		AuthorizationInformers: authorizationInformers,
		TemplateInformers:      templateInformers,
	}

	// ensure that the limit range informer will be started
	informer := config.Informers.InternalKubernetesInformers().Core().InternalVersion().LimitRanges().Informer()
	config.LimitVerifier = imageadmission.NewLimitVerifier(imageadmission.LimitRangesForNamespaceFunc(func(ns string) ([]*kapi.LimitRange, error) {
		list, err := config.Informers.InternalKubernetesInformers().Core().InternalVersion().LimitRanges().Lister().LimitRanges(ns).List(labels.Everything())
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

func buildAdmissionChains(options configapi.MasterConfig, kubeClientSet kclientsetinternal.Interface, pluginInitializer oadmission.PluginInitializer, kubePluginInitializer admission.PluginInitializer) (admission.Interface /*origin*/, admission.Interface /*kube*/, error) {
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
			kubeAdmission, err = newAdmissionChainFunc(KubeAdmissionPlugins, kubeAdmissionPluginConfigFilename, options.KubernetesMasterConfig.AdmissionConfig.PluginConfig, options, kubeClientSet, pluginInitializer, kubePluginInitializer)
			if err != nil {
				return nil, nil, err
			}
		}

		// build openshift admission
		openshiftAdmission, err := newAdmissionChainFunc(openshiftAdmissionPlugins, "", options.AdmissionConfig.PluginConfig, options, kubeClientSet, pluginInitializer, kubePluginInitializer)
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

	admissionChain, err := newAdmissionChainFunc(CombinedAdmissionControlPlugins, "", pluginConfig, options, kubeClientSet, pluginInitializer, kubePluginInitializer)
	if err != nil {
		return nil, nil, err
	}

	return admissionChain, admissionChain, err
}

// newAdmissionChainFunc is for unit testing only.  You should NEVER OVERRIDE THIS outside of a unit test.
var newAdmissionChainFunc = newAdmissionChain

func newAdmissionChain(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet kclientsetinternal.Interface, pluginInitializer oadmission.PluginInitializer, kubePluginInitializer admission.PluginInitializer) (admission.Interface, error) {
	plugins := []admission.Interface{}
	allPluginInitializers := admission.PluginInitializers([]admission.PluginInitializer{kubePluginInitializer, &pluginInitializer})
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
			lc.(kadmission.WantsInternalKubeClientSet).SetInternalKubeClientSet(kubeClientSet)
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
			plugin, err = admission.InitPlugin(pluginName, pluginConfigReader, allPluginInitializers)
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
			allPluginInitializers.Initialize(plugin)
		}
	}

	// ensure that plugins have been properly initialized
	if err := oadmission.Validate(plugins); err != nil {
		return nil, err
	}

	return admission.NewChainHandler(plugins...), nil
}

func newServiceAccountTokenGetter(options configapi.MasterConfig) (serviceaccount.ServiceAccountTokenGetter, error) {
	if options.KubernetesMasterConfig == nil {
		// When we're running against an external Kubernetes, use the external kubernetes client to validate service account tokens
		// This prevents infinite auth loops if the privilegedLoopbackKubeClient authenticates using a service account token
		kubeClientset, _, err := configapi.GetExternalKubeClient(options.MasterClients.ExternalKubernetesKubeConfig, options.MasterClients.ExternalKubernetesClientConnectionOverrides)
		if err != nil {
			return nil, err
		}
		return sacontroller.NewGetterFromClient(kubeClientset), nil
	}

	// TODO: could be hoisted if other Origin code needs direct access to etcd, otherwise discourage this access pattern
	// as we move to be more on top of Kube.
	apiserverOptions, err := kubernetes.BuildKubeAPIserverOptions(options)
	if err != nil {
		return nil, err
	}
	kubeStorageFactory, err := kubernetes.BuildStorageFactory(options, apiserverOptions, nil)
	if err != nil {
		return nil, err
	}

	storageConfig, err := kubeStorageFactory.NewConfig(kapi.Resource("serviceaccounts"))
	if err != nil {
		return nil, err
	}
	// TODO: by doing this we will not be able to authenticate while a master quorum is not present - reimplement
	// as two storages called in succession (non quorum and then quorum).
	storageConfig.Quorum = true
	return sacontroller.NewGetterFromStorageInterface(storageConfig, kubeStorageFactory.ResourcePrefix(kapi.Resource("serviceaccounts")), kubeStorageFactory.ResourcePrefix(kapi.Resource("secrets"))), nil
}

func newAuthenticator(config configapi.MasterConfig, restOptionsGetter restoptions.Getter, tokenGetter serviceaccount.ServiceAccountTokenGetter, apiClientCAs *x509.CertPool, groupMapper identitymapper.UserToGroupMapper) (authenticator.Request, error) {
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
		tokenAuthenticators = append(tokenAuthenticators, bearertoken.New(serviceAccountTokenAuthenticator), paramtoken.New("access_token", serviceAccountTokenAuthenticator, true))
	}

	// OAuth token
	if config.OAuthConfig != nil {
		oauthTokenAuthenticator, err := getEtcdTokenAuthenticator(restOptionsGetter, groupMapper)
		if err != nil {
			return nil, fmt.Errorf("Error building OAuth token authenticator: %v", err)
		}
		oauthTokenRequestAuthenticators := []authenticator.Request{
			bearertoken.New(oauthTokenAuthenticator),
			// Allow token as access_token param for WebSockets
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

	if configapi.UseTLS(config.ServingInfo.ServingInfo) {
		// build cert authenticator
		// TODO: add "system:" prefix in authenticator, limit cert to username
		// TODO: add "system:" prefix to groups in authenticator, limit cert to group name
		opts := x509request.DefaultVerifyOptions()
		opts.Roots = apiClientCAs
		certauth := x509request.New(opts, x509request.CommonNameUserConversion)
		authenticators = append(authenticators, certauth)
	}

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

func newProjectAuthorizationCache(subjectLocator authorizer.SubjectLocator, kubeClient kclientsetinternal.Interface, informerFactory shared.InformerFactory) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		informerFactory.InternalKubernetesInformers().Core().InternalVersion().Namespaces().Informer(),
		projectauth.NewAuthorizerReviewer(subjectLocator),
		informerFactory.ClusterPolicies().Lister(),
		informerFactory.ClusterPolicyBindings().Lister(),
		informerFactory.Policies().Lister(),
		informerFactory.PolicyBindings().Lister(),
	)
}

func addAuthorizationListerWatchers(customListerWatchers shared.DefaultListerWatcherOverrides, optsGetter restoptions.Getter) error {
	lw, err := newClusterPolicyLW(optsGetter)
	if err != nil {
		return err
	}
	customListerWatchers[authorizationapi.Resource("clusterpolicies")] = lw
	lw, err = newClusterPolicyBindingLW(optsGetter)
	if err != nil {
		return err
	}
	customListerWatchers[authorizationapi.Resource("clusterpolicybindings")] = lw
	lw, err = newPolicyLW(optsGetter)
	if err != nil {
		return err
	}
	customListerWatchers[authorizationapi.Resource("policies")] = lw
	lw, err = newPolicyBindingLW(optsGetter)
	if err != nil {
		return err
	}
	customListerWatchers[authorizationapi.Resource("policybindings")] = lw

	return nil
}

func newClusterPolicyLW(optsGetter restoptions.Getter) (cache.ListerWatcher, error) {
	ctx := apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll)

	storage, err := clusterpolicyetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	registry := clusterpolicyregistry.NewRegistry(storage)

	return &controller.InternalListWatch{
		ListFunc: func(options metainternal.ListOptions) (runtime.Object, error) {
			return registry.ListClusterPolicies(ctx, &options)
		},
		WatchFunc: func(options metainternal.ListOptions) (watch.Interface, error) {
			return registry.WatchClusterPolicies(ctx, &options)
		},
	}, nil
}

func newClusterPolicyBindingLW(optsGetter restoptions.Getter) (cache.ListerWatcher, error) {
	ctx := apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll)

	storage, err := clusterpolicybindingetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	registry := clusterpolicybindingregistry.NewRegistry(storage)

	return &controller.InternalListWatch{
		ListFunc: func(options metainternal.ListOptions) (runtime.Object, error) {
			return registry.ListClusterPolicyBindings(ctx, &options)
		},
		WatchFunc: func(options metainternal.ListOptions) (watch.Interface, error) {
			return registry.WatchClusterPolicyBindings(ctx, &options)
		},
	}, nil
}

func newPolicyLW(optsGetter restoptions.Getter) (cache.ListerWatcher, error) {
	ctx := apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll)

	storage, err := policyetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	registry := policyregistry.NewRegistry(storage)

	return &controller.InternalListWatch{
		ListFunc: func(options metainternal.ListOptions) (runtime.Object, error) {
			return registry.ListPolicies(ctx, &options)
		},
		WatchFunc: func(options metainternal.ListOptions) (watch.Interface, error) {
			return registry.WatchPolicies(ctx, &options)
		},
	}, nil
}

func newPolicyBindingLW(optsGetter restoptions.Getter) (cache.ListerWatcher, error) {
	ctx := apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll)

	storage, err := policybindingetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	registry := policybindingregistry.NewRegistry(storage)

	return &controller.InternalListWatch{
		ListFunc: func(options metainternal.ListOptions) (runtime.Object, error) {
			return registry.ListPolicyBindings(ctx, &options)
		},
		WatchFunc: func(options metainternal.ListOptions) (watch.Interface, error) {
			return registry.WatchPolicyBindings(ctx, &options)
		},
	}, nil
}

func newAuthorizer(ruleResolver rulevalidation.AuthorizationRuleResolver, informerFactory shared.InformerFactory, projectRequestDenyMessage string) (kauthorizer.Authorizer, authorizer.SubjectLocator) {
	messageMaker := authorizer.NewForbiddenMessageResolver(projectRequestDenyMessage)
	roleBasedAuthorizer, subjectLocator := authorizer.NewAuthorizer(ruleResolver, messageMaker)
	scopeLimitedAuthorizer := scope.NewAuthorizer(roleBasedAuthorizer, informerFactory.ClusterPolicies().Lister().ClusterPolicies(), messageMaker)

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

func getEtcdTokenAuthenticator(optsGetter restoptions.Getter, groupMapper identitymapper.UserToGroupMapper) (authenticator.Token, error) {
	// this never does a create for access tokens, so we don't need to be able to validate scopes against the client
	accessTokenStorage, err := accesstokenetcd.NewREST(optsGetter, nil)
	if err != nil {
		return nil, err
	}
	accessTokenRegistry := accesstokenregistry.NewRegistry(accessTokenStorage)

	userStorage, err := useretcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	userRegistry := userregistry.NewRegistry(userStorage)

	return authnregistry.NewTokenAuthenticator(accessTokenRegistry, userRegistry, groupMapper), nil
}

// KubeClientsetInternal returns the kubernetes client object
func (c *MasterConfig) KubeClientsetInternal() kclientsetinternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetInternal
}

// KubeClientsetInternal returns the kubernetes client object
func (c *MasterConfig) KubeClientsetExternal() kclientsetexternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetExternal
}

// OAuthServerClients returns the openshift and kubernetes OAuth server client objects
// The returned clients are privileged
func (c *MasterConfig) OAuthServerClients() (*osclient.Client, kclientsetinternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal
}

// PolicyClient returns the policy client object
// It must have the following capabilities:
//  list, watch all policyBindings in all namespaces
//  list, watch all policies in all namespaces
//  create resourceAccessReviews in all namespaces
func (c *MasterConfig) PolicyClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// ServiceAccountRoleBindingClient returns the client object used to bind roles to service accounts
// It must have the following capabilities:
//  get, list, update, create policyBindings and clusterPolicyBindings in all namespaces
func (c *MasterConfig) ServiceAccountRoleBindingClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// BuildLogClient returns the build log client object
func (c *MasterConfig) BuildLogClient() kclientsetinternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetInternal
}

// BuildConfigWebHookClient returns the webhook client object
func (c *MasterConfig) BuildConfigWebHookClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

func (c *MasterConfig) GetOpenShiftClientEnvVars() ([]kapi.EnvVar, error) {
	_, kclientConfig, err := configapi.GetInternalKubeClient(
		c.Options.MasterClients.OpenShiftLoopbackKubeConfig,
		c.Options.MasterClients.OpenShiftLoopbackClientConnectionOverrides,
	)
	if err != nil {
		return nil, err
	}
	return clientcmd.EnvVars(
		kclientConfig.Host,
		kclientConfig.CAData,
		kclientConfig.Insecure,
		path.Join(serviceaccountadmission.DefaultAPITokenMountPath, kapi.ServiceAccountTokenKey),
	), nil
}

// BuildControllerClients returns the build controller client objects
func (c *MasterConfig) BuildControllerClients() (*osclient.Client, kclientsetinternal.Interface, kclientsetexternal.Interface) {
	_, osClient, internalKubeClientset, externalKubeClientset, err := c.GetServiceAccountClients(bootstrappolicy.InfraBuildControllerServiceAccountName)
	if err != nil {
		glog.Fatal(err)
	}
	return osClient, internalKubeClientset, externalKubeClientset
}

// BuildPodControllerClients returns the build pod controller client objects
func (c *MasterConfig) BuildPodControllerClients() (*osclient.Client, kclientsetinternal.Interface, kclientsetexternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal, c.PrivilegedLoopbackKubernetesClientsetExternal
}

// BuildImageChangeTriggerControllerClients returns the build image change trigger controller client objects
func (c *MasterConfig) BuildImageChangeTriggerControllerClients() (*osclient.Client, kclientsetinternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal
}

// BuildConfigChangeControllerClients returns the build config change controller client objects
func (c *MasterConfig) BuildConfigChangeControllerClients() (*osclient.Client, kclientsetinternal.Interface, kclientsetexternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal, c.PrivilegedLoopbackKubernetesClientsetExternal
}

// ImageImportControllerClient returns the deployment client object
func (c *MasterConfig) ImageImportControllerClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

func (c *MasterConfig) DeploymentConfigInstantiateClients() (*osclient.Client, kclientsetinternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal
}

// DeploymentConfigClients returns deploymentConfig and deployment client objects
func (c *MasterConfig) DeploymentConfigClients() (*osclient.Client, kclientsetinternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal
}

// ImageTriggerControllerClients returns the trigger controller client objects
func (c *MasterConfig) ImageTriggerControllerClients() (*osclient.Client, kclientsetinternal.Interface, kclientsetexternal.Interface) {
	_, osClient, internalKubeClientset, externalKubeClientset, err := c.GetServiceAccountClients(bootstrappolicy.InfraImageTriggerControllerServiceAccountName)
	if err != nil {
		glog.Fatal(err)
	}
	return osClient, internalKubeClientset, externalKubeClientset
}

// DeploymentLogClient returns the deployment log client object
func (c *MasterConfig) DeploymentLogClient() kclientsetinternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetInternal
}

// SecurityAllocationControllerClient returns the security allocation controller client object
func (c *MasterConfig) SecurityAllocationControllerClient() kclientsetinternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetInternal
}

// SDNControllerClients returns the SDN controller client objects
func (c *MasterConfig) SDNControllerClients() (*osclient.Client, kclientsetinternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal
}

// RouteAllocatorClients returns the route allocator client objects
func (c *MasterConfig) RouteAllocatorClients() (*osclient.Client, kclientsetinternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal
}

// ImageStreamSecretClient returns the client capable of retrieving secrets for an image secret wrapper
func (c *MasterConfig) ImageStreamSecretClient() kclientsetinternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetInternal
}

// ImageStreamImportSecretClient returns the client capable of retrieving image secrets for a namespace
func (c *MasterConfig) ImageStreamImportSecretClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// ImageStreamImportSARClient returns the client capable of performing self-SAR requests
func (c *MasterConfig) ImageStreamImportSARClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// ResourceQuotaManagerClients returns the client capable of retrieving resources needed for resource quota
// evaluation
func (c *MasterConfig) ResourceQuotaManagerClients() (*osclient.Client, kclientsetinternal.Interface, kclientsetexternal.Interface) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientsetInternal, c.PrivilegedLoopbackKubernetesClientsetExternal
}

// WebConsoleEnabled says whether web ui is not a disabled feature and asset service is configured.
func (c *MasterConfig) WebConsoleEnabled() bool {
	return c.Options.AssetConfig != nil && !c.Options.DisabledFeatures.Has(configapi.FeatureWebConsole)
}

// OriginNamespaceControllerClient returns a client for openshift and kubernetes.
// The kubernetes client object must have authority to execute a finalize request on a namespace
func (c *MasterConfig) OriginNamespaceControllerClient() kclientsetinternal.Interface {
	return c.PrivilegedLoopbackKubernetesClientsetInternal
}

// UnidlingControllerClients returns the unidling controller clients
func (c *MasterConfig) UnidlingControllerClients() (*osclient.Client, kclientsetinternal.Interface, kextensionsclient.ScalesGetter) {
	_, osClient, kClient, _, err := c.GetServiceAccountClients(bootstrappolicy.InfraUnidlingControllerServiceAccountName)
	if err != nil {
		glog.Fatal(err)
	}
	extensionsClient := kextensionsclient.New(kClient.Core().RESTClient())
	return osClient, kClient, extensionsClient
}

// GetServiceAccountClients returns an OpenShift and Kubernetes client with the credentials of the
// named service account in the infra namespace
func (c *MasterConfig) GetServiceAccountClients(name string) (*restclient.Config, *osclient.Client, kclientsetinternal.Interface, kclientsetexternal.Interface, error) {
	return c.GetServiceAccountClientsWithConfig(name, c.PrivilegedLoopbackClientConfig)
}

func (c *MasterConfig) GetServiceAccountClientsWithConfig(name string, config restclient.Config) (*restclient.Config, *osclient.Client, kclientsetinternal.Interface, kclientsetexternal.Interface, error) {
	if len(name) == 0 {
		return nil, nil, nil, nil, errors.New("No service account name specified")
	}
	configToReturn, oc, internalKubeClientset, externalKubeClientset, err := serviceaccounts.Clients(
		config,
		&serviceaccounts.ClientLookupTokenRetriever{Client: c.PrivilegedLoopbackKubernetesClientsetInternal},
		c.Options.PolicyConfig.OpenShiftInfrastructureNamespace,
		name,
	)
	return configToReturn, oc, internalKubeClientset, externalKubeClientset, err
}
