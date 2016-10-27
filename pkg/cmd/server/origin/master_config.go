package origin

import (
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"path"
	"reflect"
	"strings"
	"time"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apiserver"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	clientadapter "k8s.io/kubernetes/pkg/client/unversioned/adapters/internalclientset"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/serviceaccount"
	kutilrand "k8s.io/kubernetes/pkg/util/rand"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"
	"k8s.io/kubernetes/plugin/pkg/admission/namespace/lifecycle"
	saadmit "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
	storageclassdefaultadmission "k8s.io/kubernetes/plugin/pkg/admission/storageclass/default"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/anonymous"
	"github.com/openshift/origin/pkg/auth/authenticator/request/bearertoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/paramtoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/unionrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
	"github.com/openshift/origin/pkg/auth/group"
	authnregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
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
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
	originrest "github.com/openshift/origin/pkg/cmd/server/origin/rest"
	"github.com/openshift/origin/pkg/cmd/util/plug"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/controller/shared"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imagepolicy "github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	quotaadmission "github.com/openshift/origin/pkg/quota/admission/resourcequota"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	"github.com/openshift/origin/pkg/service"
	serviceadmit "github.com/openshift/origin/pkg/service/admission"
	"github.com/openshift/origin/pkg/serviceaccounts"
	usercache "github.com/openshift/origin/pkg/user/cache"
	groupregistry "github.com/openshift/origin/pkg/user/registry/group"
	groupstorage "github.com/openshift/origin/pkg/user/registry/group/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/util/leaderlease"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	Options configapi.MasterConfig

	// RESTOptionsGetter provides access to storage and RESTOptions for a particular resource
	RESTOptionsGetter restoptions.Getter

	RuleResolver                  rulevalidation.AuthorizationRuleResolver
	Authenticator                 authenticator.Request
	Authorizer                    authorizer.Authorizer
	AuthorizationAttributeBuilder authorizer.AuthorizationAttributeBuilder

	GroupCache                    *usercache.GroupCache
	ProjectAuthorizationCache     *projectauth.AuthorizationCache
	ProjectCache                  *projectcache.ProjectCache
	ClusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController
	LimitVerifier                 imageadmission.LimitVerifier

	// RequestContextMapper maps requests to contexts
	RequestContextMapper kapi.RequestContextMapper
	// RequestInfoResolver is responsible for reading request attributes
	RequestInfoResolver *apiserver.RequestInfoResolver

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

	// PrivilegedLoopbackKubernetesClient is the client used to call Kubernetes APIs from system components,
	// built from KubeClientConfig. It should only be accessed via the *Client() helper methods. To apply
	// different access control to a system component, create a separate client/config specifically for
	// that component.
	PrivilegedLoopbackKubernetesClient *kclient.Client
	// PrivilegedLoopbackOpenShiftClient is the client used to call OpenShift APIs from system components,
	// built from PrivilegedLoopbackClientConfig. It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically
	// for that component.
	PrivilegedLoopbackOpenShiftClient *osclient.Client

	// Informers is a shared factory for getting SharedInformers. It is important to get your informers, indexers, and listers
	// from here so that we only end up with a single cache of objects
	Informers shared.InformerFactory
}

// BuildMasterConfig builds and returns the OpenShift master configuration based on the
// provided options
func BuildMasterConfig(options configapi.MasterConfig) (*MasterConfig, error) {
	client, err := etcd.MakeEtcdClient(options.EtcdClientInfo)
	if err != nil {
		return nil, err
	}

	restOptsGetter := originrest.StorageOptions(options)

	clientCAs, err := configapi.GetClientCertCAPool(options)
	if err != nil {
		return nil, err
	}
	apiClientCAs, err := configapi.GetAPIClientCertCAPool(options)
	if err != nil {
		return nil, err
	}

	privilegedLoopbackKubeClient, _, err := configapi.GetKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	privilegedLoopbackOpenShiftClient, privilegedLoopbackClientConfig, err := configapi.GetOpenShiftClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}

	customListerWatchers := shared.DefaultListerWatcherOverrides{}
	if err := addAuthorizationListerWatchers(customListerWatchers, restOptsGetter); err != nil {
		return nil, err
	}
	informerFactory := shared.NewInformerFactory(privilegedLoopbackKubeClient, privilegedLoopbackOpenShiftClient, customListerWatchers, 10*time.Minute)

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(privilegedLoopbackKubeClient.Services(kapi.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		return nil, fmt.Errorf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	requestContextMapper := kapi.NewRequestContextMapper()

	groupStorage, err := groupstorage.NewREST(restOptsGetter)
	if err != nil {
		return nil, err
	}
	groupCache := usercache.NewGroupCache(groupregistry.NewRegistry(groupStorage))
	projectCache := projectcache.NewProjectCache(privilegedLoopbackKubeClient.Namespaces(), options.ProjectConfig.DefaultNodeSelector)
	clusterQuotaMappingController := clusterquotamapping.NewClusterQuotaMappingController(informerFactory.Namespaces(), informerFactory.ClusterResourceQuotas())

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)

	kubeClientSet := clientadapter.FromUnversionedClient(privilegedLoopbackKubeClient)
	quotaRegistry := quota.NewAllResourceQuotaRegistry(privilegedLoopbackOpenShiftClient, kubeClientSet)
	ruleResolver := rulevalidation.NewDefaultRuleResolver(
		informerFactory.Policies().Lister(),
		informerFactory.PolicyBindings().Lister(),
		informerFactory.ClusterPolicies().Lister().ClusterPolicies(),
		informerFactory.ClusterPolicyBindings().Lister().ClusterPolicyBindings(),
	)
	authorizer := newAuthorizer(ruleResolver, informerFactory, options.ProjectConfig.ProjectRequestMessage)

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
	}
	originAdmission, kubeAdmission, err := buildAdmissionChains(options, kubeClientSet, pluginInitializer)
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

	plug, plugStart := newControllerPlug(options, client)

	config := &MasterConfig{
		Options: options,

		RESTOptionsGetter: restOptsGetter,

		RuleResolver:                  ruleResolver,
		Authenticator:                 authenticator,
		Authorizer:                    authorizer,
		AuthorizationAttributeBuilder: newAuthorizationAttributeBuilder(requestContextMapper),

		GroupCache:                    groupCache,
		ProjectAuthorizationCache:     newProjectAuthorizationCache(authorizer, privilegedLoopbackKubeClient, informerFactory),
		ProjectCache:                  projectCache,
		ClusterQuotaMappingController: clusterQuotaMappingController,

		RequestContextMapper: requestContextMapper,

		AdmissionControl:     originAdmission,
		KubeAdmissionControl: kubeAdmission,

		TLS: configapi.UseTLS(options.ServingInfo.ServingInfo),

		ControllerPlug:      plug,
		ControllerPlugStart: plugStart,

		ImageFor:       imageTemplate.ExpandOrDie,
		RegistryNameFn: imageapi.DefaultRegistryFunc(defaultRegistryFunc),

		// TODO: migration of versions of resources stored in annotations must be sorted out
		ExternalVersionCodec: kapi.Codecs.LegacyCodec(unversioned.GroupVersion{Group: "", Version: "v1"}),

		KubeletClientConfig: kubeletClientConfig,

		ClientCAs:    clientCAs,
		APIClientCAs: apiClientCAs,

		PrivilegedLoopbackClientConfig:     *privilegedLoopbackClientConfig,
		PrivilegedLoopbackOpenShiftClient:  privilegedLoopbackOpenShiftClient,
		PrivilegedLoopbackKubernetesClient: privilegedLoopbackKubeClient,
		Informers:                          informerFactory,
	}

	// ensure that the limit range informer will be started
	informer := config.Informers.LimitRanges().Informer()
	config.LimitVerifier = imageadmission.NewLimitVerifier(imageadmission.LimitRangesForNamespaceFunc(func(ns string) ([]*kapi.LimitRange, error) {
		list, err := config.Informers.LimitRanges().Lister().LimitRanges(ns).List(labels.Everything())
		if err != nil {
			return nil, err
		}
		// the verifier must return an error
		if len(list) == 0 && len(informer.LastSyncResourceVersion()) == 0 {
			glog.V(4).Infof("LimitVerifier still waiting for ranges to load: %#v", informer)
			forbiddenErr := kapierrors.NewForbidden(unversioned.GroupResource{Resource: "limitranges"}, "", fmt.Errorf("the server is still loading limit information"))
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
		"PodNodeConstraints",
		"openshift.io/JenkinsBootstrapper",
		"BuildByStrategy",
		imageadmission.PluginName,
		"OwnerReferencesPermissionEnforcement",
		quotaadmission.PluginName,
	}

	// KubeAdmissionPlugins gives the in-order default admission chain for kube resources.
	KubeAdmissionPlugins = []string{
		lifecycle.PluginName,
		"RunOnceDuration",
		"PodNodeConstraints",
		"OriginPodNodeEnvironment",
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
		// NOTE: quotaadmission and ClusterResourceQuota must be the last 2 plugins.
		// DO NOT ADD ANY PLUGINS AFTER THIS LINE!
		quotaadmission.PluginName,
		"openshift.io/ClusterResourceQuota",
	}

	// CombinedAdmissionControlPlugins gives the in-order default admission chain for all resources resources.
	// When possible, this list is used.  The set of openshift+kube chains must exactly match this set.  In addition,
	// the order specified in the openshift and kube chains must match the order here.
	CombinedAdmissionControlPlugins = []string{
		lifecycle.PluginName,
		"ProjectRequestLimit",
		"OriginNamespaceLifecycle",
		"PodNodeConstraints",
		"openshift.io/JenkinsBootstrapper",
		"BuildByStrategy",
		imageadmission.PluginName,
		"RunOnceDuration",
		"PodNodeConstraints",
		"OriginPodNodeEnvironment",
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
		// NOTE: quotaadmission and ClusterResourceQuota must be the last 2 plugins.
		// DO NOT ADD ANY PLUGINS AFTER THIS LINE!
		quotaadmission.PluginName,
		"openshift.io/ClusterResourceQuota",
	}
)

func buildAdmissionChains(options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface /*origin*/, admission.Interface /*kube*/, error) {
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
			kubeAdmission, err = newAdmissionChainFunc(KubeAdmissionPlugins, kubeAdmissionPluginConfigFilename, options.KubernetesMasterConfig.AdmissionConfig.PluginConfig, options, kubeClientSet, pluginInitializer)
			if err != nil {
				return nil, nil, err
			}
		}

		// build openshift admission
		openshiftAdmission, err := newAdmissionChainFunc(openshiftAdmissionPlugins, "", options.AdmissionConfig.PluginConfig, options, kubeClientSet, pluginInitializer)
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

	admissionChain, err := newAdmissionChainFunc(CombinedAdmissionControlPlugins, "", pluginConfig, options, kubeClientSet, pluginInitializer)
	if err != nil {
		return nil, nil, err
	}

	return admissionChain, admissionChain, err
}

// newAdmissionChainFunc is for unit testing only.  You should NEVER OVERRIDE THIS outside of a unit test.
var newAdmissionChainFunc = newAdmissionChain

func newAdmissionChain(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
	plugins := []admission.Interface{}
	for _, pluginName := range pluginNames {
		switch pluginName {
		case lifecycle.PluginName:
			// We need to include our infrastructure and shared resource namespaces in the immortal namespaces list
			immortalNamespaces := sets.NewString(kapi.NamespaceDefault)
			if len(options.PolicyConfig.OpenShiftSharedResourcesNamespace) > 0 {
				immortalNamespaces.Insert(options.PolicyConfig.OpenShiftSharedResourcesNamespace)
			}
			if len(options.PolicyConfig.OpenShiftInfrastructureNamespace) > 0 {
				immortalNamespaces.Insert(options.PolicyConfig.OpenShiftInfrastructureNamespace)
			}
			lc, err := lifecycle.NewLifecycle(kubeClientSet, immortalNamespaces)
			if err != nil {
				return nil, err
			}
			plugins = append(plugins, lc)

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
			plugins = append(plugins, serviceadmit.NewExternalIPRanger(reject, admit, allowIngressIP))

		case serviceadmit.RestrictedEndpointsPluginName:
			// we need to set some customer parameters, so create by hand
			restrictedNetworks, err := serviceadmit.ParseSimpleCIDRRules([]string{options.NetworkConfig.ClusterNetworkCIDR, options.NetworkConfig.ServiceNetworkCIDR})
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			plugins = append(plugins, serviceadmit.NewRestrictedEndpointsAdmission(restrictedNetworks))

		case saadmit.PluginName:
			// we need to set some custom parameters on the service account admission controller, so create that one by hand
			saAdmitter := saadmit.NewServiceAccount(kubeClientSet)
			saAdmitter.LimitSecretReferences = options.ServiceAccountConfig.LimitSecretReferences
			saAdmitter.Run()
			plugins = append(plugins, saAdmitter)

		default:
			configFile, err := pluginconfig.GetPluginConfigFile(pluginConfig, pluginName, admissionConfigFilename)
			if err != nil {
				return nil, err
			}
			plugin := admission.InitPlugin(pluginName, kubeClientSet, configFile)
			if plugin != nil {
				plugins = append(plugins, plugin)
			}

		}
	}

	pluginInitializer.Initialize(plugins)
	// ensure that plugins have been properly initialized
	if err := oadmission.Validate(plugins); err != nil {
		return nil, err
	}

	return admission.NewChainHandler(plugins...), nil
}

func newControllerPlug(options configapi.MasterConfig, client etcdclient.Client) (plug.Plug, func()) {
	switch {
	case options.ControllerLeaseTTL > 0:
		// TODO: replace with future API for leasing from Kube
		id := fmt.Sprintf("master-%s", kutilrand.String(8))
		leaser := leaderlease.NewEtcd(
			client,
			path.Join(options.EtcdStorageConfig.OpenShiftStoragePrefix, "leases/controllers"),
			id,
			uint64(options.ControllerLeaseTTL),
		)
		leased := plug.NewLeased(leaser)
		return leased, func() {
			glog.V(2).Infof("Attempting to acquire controller lease as %s, renewing every %d seconds", id, options.ControllerLeaseTTL)
			go leased.Run()
		}
	default:
		return plug.New(!options.PauseControllers), func() {}
	}
}

func newServiceAccountTokenGetter(options configapi.MasterConfig) (serviceaccount.ServiceAccountTokenGetter, error) {
	if options.KubernetesMasterConfig == nil {
		// When we're running against an external Kubernetes, use the external kubernetes client to validate service account tokens
		// This prevents infinite auth loops if the privilegedLoopbackKubeClient authenticates using a service account token
		kubeClient, _, err := configapi.GetKubeClient(options.MasterClients.ExternalKubernetesKubeConfig, options.MasterClients.ExternalKubernetesClientConnectionOverrides)
		if err != nil {
			return nil, err
		}
		return sacontroller.NewGetterFromClient(clientadapter.FromUnversionedClient(kubeClient)), nil
	}

	// TODO: could be hoisted if other Origin code needs direct access to etcd, otherwise discourage this access pattern
	// as we move to be more on top of Kube.
	_, kubeStorageFactory, err := kubernetes.BuildDefaultAPIServer(options)
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
		publicKeys := []*rsa.PublicKey{}
		for _, keyFile := range config.ServiceAccountConfig.PublicKeyFiles {
			publicKey, err := serviceaccount.ReadPublicKey(keyFile)
			if err != nil {
				return nil, fmt.Errorf("Error reading service account key file %s: %v", keyFile, err)
			}
			publicKeys = append(publicKeys, publicKey)
		}
		serviceAccountTokenAuthenticator := serviceaccount.JWTTokenAuthenticator(publicKeys, true, tokenGetter)
		tokenAuthenticators = append(tokenAuthenticators, bearertoken.New(serviceAccountTokenAuthenticator, true))
	}

	// OAuth token
	if config.OAuthConfig != nil {
		oauthTokenAuthenticator, err := getEtcdTokenAuthenticator(restOptionsGetter, groupMapper)
		if err != nil {
			return nil, fmt.Errorf("Error building OAuth token authenticator: %v", err)
		}
		oauthTokenRequestAuthenticators := []authenticator.Request{
			bearertoken.New(oauthTokenAuthenticator, true),
			// Allow token as access_token param for WebSockets
			paramtoken.New("access_token", oauthTokenAuthenticator, true),
		}

		tokenAuthenticators = append(tokenAuthenticators,
			// if you have a bearer token, you're a human (usually)
			// if you change this, have a look at the impersonationFilter where we attach groups to the impersonated user
			group.NewGroupAdder(unionrequest.NewUnionAuthentication(oauthTokenRequestAuthenticators...), []string{bootstrappolicy.AuthenticatedOAuthGroup}))
	}

	if len(tokenAuthenticators) > 0 {
		authenticators = append(authenticators, unionrequest.NewUnionAuthentication(tokenAuthenticators...))
	}

	if configapi.UseTLS(config.ServingInfo.ServingInfo) {
		// build cert authenticator
		// TODO: add "system:" prefix in authenticator, limit cert to username
		// TODO: add "system:" prefix to groups in authenticator, limit cert to group name
		opts := x509request.DefaultVerifyOptions()
		opts.Roots = apiClientCAs
		certauth := x509request.New(opts, x509request.SubjectToUserConversion)
		authenticators = append(authenticators, certauth)
	}

	ret := &unionrequest.Authenticator{
		FailOnError: true,
		Handlers: []authenticator.Request{
			// if you change this, have a look at the impersonationFilter where we attach groups to the impersonated user
			group.NewGroupAdder(&unionrequest.Authenticator{FailOnError: true, Handlers: authenticators}, []string{bootstrappolicy.AuthenticatedGroup}),
			anonymous.NewAuthenticator(),
		},
	}

	return ret, nil
}

func newProjectAuthorizationCache(authorizer authorizer.Authorizer, kubeClient *kclient.Client, informerFactory shared.InformerFactory) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		projectauth.NewAuthorizerReviewer(authorizer),
		kubeClient.Namespaces(),
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
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	storage, err := clusterpolicyetcd.NewStorage(optsGetter)
	if err != nil {
		return nil, err
	}
	registry := clusterpolicyregistry.NewRegistry(storage)

	return &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			return registry.ListClusterPolicies(ctx, &options)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			return registry.WatchClusterPolicies(ctx, &options)
		},
	}, nil
}

func newClusterPolicyBindingLW(optsGetter restoptions.Getter) (cache.ListerWatcher, error) {
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	storage, err := clusterpolicybindingetcd.NewStorage(optsGetter)
	if err != nil {
		return nil, err
	}
	registry := clusterpolicybindingregistry.NewRegistry(storage)

	return &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			return registry.ListClusterPolicyBindings(ctx, &options)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			return registry.WatchClusterPolicyBindings(ctx, &options)
		},
	}, nil
}

func newPolicyLW(optsGetter restoptions.Getter) (cache.ListerWatcher, error) {
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	storage, err := policyetcd.NewStorage(optsGetter)
	if err != nil {
		return nil, err
	}
	registry := policyregistry.NewRegistry(storage)

	return &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			return registry.ListPolicies(ctx, &options)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			return registry.WatchPolicies(ctx, &options)
		},
	}, nil
}

func newPolicyBindingLW(optsGetter restoptions.Getter) (cache.ListerWatcher, error) {
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	storage, err := policybindingetcd.NewStorage(optsGetter)
	if err != nil {
		return nil, err
	}
	registry := policybindingregistry.NewRegistry(storage)

	return &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			return registry.ListPolicyBindings(ctx, &options)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			return registry.WatchPolicyBindings(ctx, &options)
		},
	}, nil
}

func newAuthorizer(ruleResolver rulevalidation.AuthorizationRuleResolver, informerFactory shared.InformerFactory, projectRequestDenyMessage string) authorizer.Authorizer {
	messageMaker := authorizer.NewForbiddenMessageResolver(projectRequestDenyMessage)
	roleBasedAuthorizer := authorizer.NewAuthorizer(ruleResolver, messageMaker)
	scopeLimitedAuthorizer := scope.NewAuthorizer(roleBasedAuthorizer, informerFactory.ClusterPolicies().Lister().ClusterPolicies(), messageMaker)
	return scopeLimitedAuthorizer
}

func newAuthorizationAttributeBuilder(requestContextMapper kapi.RequestContextMapper) authorizer.AuthorizationAttributeBuilder {
	// Default API request resolver
	requestInfoResolver := &apiserver.RequestInfoResolver{APIPrefixes: sets.NewString("api", "osapi", "oapi", "apis"), GrouplessAPIPrefixes: sets.NewString("api", "osapi", "oapi")}
	// Wrap with a resolver that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately
	browserSafeRequestInfoResolver := authorizer.NewBrowserSafeRequestInfoResolver(
		requestContextMapper,
		sets.NewString(bootstrappolicy.AuthenticatedGroup),
		requestInfoResolver,
	)

	authorizationAttributeBuilder := authorizer.NewAuthorizationAttributeBuilder(requestContextMapper, browserSafeRequestInfoResolver)
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

// KubeClient returns the kubernetes client object
func (c *MasterConfig) KubeClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// OAuthServerClients returns the openshift and kubernetes OAuth server client objects
// The returned clients are privileged
func (c *MasterConfig) OAuthServerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
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

// SdnClient returns the sdn client object
// It must have the capability to get/list/watch/create/delete
// HostSubnets. And have the capability to get ClusterNetwork.
func (c *MasterConfig) SdnClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// DeploymentClient returns the deployment client object
func (c *MasterConfig) DeploymentClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// DNSServerClient returns the DNS server client object
// It must have the following capabilities:
//   list, watch all services in all namespaces
func (c *MasterConfig) DNSServerClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// BuildLogClient returns the build log client object
func (c *MasterConfig) BuildLogClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// BuildConfigWebHookClient returns the webhook client object
func (c *MasterConfig) BuildConfigWebHookClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// BuildControllerClients returns the build controller client objects
func (c *MasterConfig) BuildControllerClients() (*osclient.Client, *kclient.Client) {
	_, osClient, kClient, err := c.GetServiceAccountClients(bootstrappolicy.InfraBuildControllerServiceAccountName)
	if err != nil {
		glog.Fatal(err)
	}
	return osClient, kClient
}

// BuildPodControllerClients returns the build pod controller client objects
func (c *MasterConfig) BuildPodControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// BuildImageChangeTriggerControllerClients returns the build image change trigger controller client objects
func (c *MasterConfig) BuildImageChangeTriggerControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// BuildConfigChangeControllerClients returns the build config change controller client objects
func (c *MasterConfig) BuildConfigChangeControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// ImageChangeControllerClient returns the openshift client object
func (c *MasterConfig) ImageChangeControllerClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// ImageImportControllerClient returns the deployment client object
func (c *MasterConfig) ImageImportControllerClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// DeploymentConfigInstantiateClients returns the clients used by the instantiate endpoint.
func (c *MasterConfig) DeploymentConfigInstantiateClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// DeploymentControllerClients returns the deployment controller client objects
func (c *MasterConfig) DeploymentControllerClients() (*osclient.Client, *kclient.Client) {
	_, osClient, kClient, err := c.GetServiceAccountClients(bootstrappolicy.InfraDeploymentConfigControllerServiceAccountName)
	if err != nil {
		glog.Fatal(err)
	}
	return osClient, kClient
}

// DeploymentConfigClients returns deploymentConfig and deployment client objects
func (c *MasterConfig) DeploymentConfigClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// DeploymentConfigControllerClients returns the deploymentConfig controller client objects
func (c *MasterConfig) DeploymentConfigControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// DeploymentTriggerControllerClient returns the deploymentConfig trigger controller client object
func (c *MasterConfig) DeploymentTriggerControllerClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// DeploymentLogClient returns the deployment log client object
func (c *MasterConfig) DeploymentLogClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// SecurityAllocationControllerClient returns the security allocation controller client object
func (c *MasterConfig) SecurityAllocationControllerClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// SDNControllerClients returns the SDN controller client objects
func (c *MasterConfig) SDNControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// RouteAllocatorClients returns the route allocator client objects
func (c *MasterConfig) RouteAllocatorClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// ImageStreamSecretClient returns the client capable of retrieving secrets for an image secret wrapper
func (c *MasterConfig) ImageStreamSecretClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// ImageStreamImportSecretClient returns the client capable of retrieving image secrets for a namespace
func (c *MasterConfig) ImageStreamImportSecretClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// ResourceQuotaManagerClients returns the client capable of retrieving resources needed for resource quota
// evaluation
func (c *MasterConfig) ResourceQuotaManagerClients() (*osclient.Client, *internalclientset.Clientset) {
	return c.PrivilegedLoopbackOpenShiftClient, clientadapter.FromUnversionedClient(c.PrivilegedLoopbackKubernetesClient)
}

// WebConsoleEnabled says whether web ui is not a disabled feature and asset service is configured.
func (c *MasterConfig) WebConsoleEnabled() bool {
	return c.Options.AssetConfig != nil && !c.Options.DisabledFeatures.Has(configapi.FeatureWebConsole)
}

// OriginNamespaceControllerClients returns a client for openshift and kubernetes.
// The openshift client object must have authority to delete openshift content in any namespace
// The kubernetes client object must have authority to execute a finalize request on a namespace
func (c *MasterConfig) OriginNamespaceControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// UnidlingControllerClients returns the unidling controller clients
func (c *MasterConfig) UnidlingControllerClients() (*osclient.Client, *kclient.Client) {
	_, osClient, kClient, err := c.GetServiceAccountClients(bootstrappolicy.InfraUnidlingControllerServiceAccountName)
	if err != nil {
		glog.Fatal(err)
	}
	return osClient, kClient
}

// GetServiceAccountClients returns an OpenShift and Kubernetes client with the credentials of the
// named service account in the infra namespace
func (c *MasterConfig) GetServiceAccountClients(name string) (*restclient.Config, *osclient.Client, *kclient.Client, error) {
	if len(name) == 0 {
		return nil, nil, nil, errors.New("No service account name specified")
	}
	return serviceaccounts.Clients(
		c.PrivilegedLoopbackClientConfig,
		&serviceaccounts.ClientLookupTokenRetriever{Client: c.PrivilegedLoopbackKubernetesClient},
		c.Options.PolicyConfig.OpenShiftInfrastructureNamespace,
		name,
	)
}
