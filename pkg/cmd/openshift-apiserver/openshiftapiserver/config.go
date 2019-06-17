package openshiftapiserver

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	admissionmetrics "k8s.io/apiserver/pkg/admission/metrics"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericapiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/webhook"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/openshift-apiserver/pkg/version"
	"github.com/openshift/origin/pkg/admission/admissiontimeout"
	"github.com/openshift/origin/pkg/cmd/configflags"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftadmission"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
	usercache "github.com/openshift/origin/pkg/user/cache"
)

func NewOpenshiftAPIConfig(config *openshiftcontrolplanev1.OpenShiftAPIServerConfig) (*OpenshiftAPIConfig, error) {
	kubeClientConfig, err := helpers.GetKubeClientConfig(config.KubeClientConfig)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return nil, err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, 10*time.Minute)

	openshiftVersion := version.Get()

	restOptsGetter, err := NewRESTOptionsGetter(config.APIServerArguments, config.StorageConfig)
	if err != nil {
		return nil, err
	}

	genericConfig := genericapiserver.NewRecommendedConfig(legacyscheme.Codecs)
	// Current default values
	//Serializer:                   codecs,
	//ReadWritePort:                443,
	//BuildHandlerChainFunc:        DefaultBuildHandlerChain,
	//HandlerChainWaitGroup:        new(utilwaitgroup.SafeWaitGroup),
	//LegacyAPIGroupPrefixes:       sets.NewString(DefaultLegacyAPIPrefix),
	//DisabledPostStartHooks:       sets.NewString(),
	//HealthzChecks:                []healthz.HealthzChecker{healthz.PingHealthz, healthz.LogHealthz},
	//EnableIndex:                  true,
	//EnableDiscovery:              true,
	//EnableProfiling:              true,
	//EnableMetrics:                true,
	//MaxRequestsInFlight:          400,
	//MaxMutatingRequestsInFlight:  200,
	//RequestTimeout:               time.Duration(60) * time.Second,
	//MinRequestTimeout:            1800,
	//EnableAPIResponseCompression: utilfeature.DefaultFeatureGate.Enabled(features.APIResponseCompression),
	//LongRunningFunc: genericfilters.BasicLongRunningRequestCheck(sets.NewString("watch"), sets.NewString()),

	// TODO this is actually specific to the kubeapiserver
	//RuleResolver authorizer.RuleResolver
	genericConfig.SharedInformerFactory = kubeInformers
	genericConfig.ClientConfig = kubeClientConfig

	// these are set via options
	//SecureServing *SecureServingInfo
	//Authentication AuthenticationInfo
	//Authorization AuthorizationInfo
	//LoopbackClientConfig *restclient.Config
	// this is set after the options are overlayed to get the authorizer we need.
	//AdmissionControl      admission.Interface
	//ReadWritePort int
	//PublicAddress net.IP

	// these are defaulted sanely during complete
	//DiscoveryAddresses discovery.Addresses

	genericConfig.CorsAllowedOriginList = config.CORSAllowedOrigins
	genericConfig.Version = &openshiftVersion
	genericConfig.ExternalAddress = "apiserver.openshift-apiserver.svc"
	genericConfig.BuildHandlerChainFunc = OpenshiftHandlerChain
	genericConfig.RequestInfoResolver = configprocessing.OpenshiftRequestInfoResolver()
	genericConfig.OpenAPIConfig = configprocessing.DefaultOpenAPIConfig()
	genericConfig.RESTOptionsGetter = restOptsGetter
	// previously overwritten.  I don't know why
	genericConfig.RequestTimeout = time.Duration(60) * time.Second
	genericConfig.MinRequestTimeout = int(config.ServingInfo.RequestTimeoutSeconds)
	genericConfig.MaxRequestsInFlight = int(config.ServingInfo.MaxRequestsInFlight)
	genericConfig.MaxMutatingRequestsInFlight = int(config.ServingInfo.MaxRequestsInFlight / 2)
	genericConfig.LongRunningFunc = configprocessing.IsLongRunningRequest

	// I'm just hoping this works.  I don't think we use it.
	//MergedResourceConfig *serverstore.ResourceConfig

	servingOptions, err := configprocessing.ToServingOptions(config.ServingInfo)
	if err != nil {
		return nil, err
	}
	if err := servingOptions.ApplyTo(&genericConfig.Config.SecureServing, &genericConfig.Config.LoopbackClientConfig); err != nil {
		return nil, err
	}
	authenticationOptions := genericapiserveroptions.NewDelegatingAuthenticationOptions()
	// keep working for integration tests
	if len(config.AggregatorConfig.ClientCA) > 0 {
		authenticationOptions.ClientCert.ClientCA = config.ServingInfo.ClientCA
		authenticationOptions.RequestHeader.ClientCAFile = config.AggregatorConfig.ClientCA
		authenticationOptions.RequestHeader.AllowedNames = config.AggregatorConfig.AllowedNames
		authenticationOptions.RequestHeader.UsernameHeaders = config.AggregatorConfig.UsernameHeaders
		authenticationOptions.RequestHeader.GroupHeaders = config.AggregatorConfig.GroupHeaders
		authenticationOptions.RequestHeader.ExtraHeaderPrefixes = config.AggregatorConfig.ExtraHeaderPrefixes
	}
	authenticationOptions.RemoteKubeConfigFile = config.KubeClientConfig.KubeConfig
	if err := authenticationOptions.ApplyTo(&genericConfig.Authentication, genericConfig.SecureServing, genericConfig.OpenAPIConfig); err != nil {
		return nil, err
	}
	authorizationOptions := genericapiserveroptions.NewDelegatingAuthorizationOptions().WithAlwaysAllowPaths("/healthz", "/healthz/").WithAlwaysAllowGroups("system:masters")
	authorizationOptions.RemoteKubeConfigFile = config.KubeClientConfig.KubeConfig
	if err := authorizationOptions.ApplyTo(&genericConfig.Authorization); err != nil {
		return nil, err
	}

	informers, err := NewInformers(kubeInformers, kubeClientConfig, genericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	if err := informers.GetOpenshiftUserInformers().User().V1().Groups().Informer().AddIndexers(cache.Indexers{
		usercache.ByUserIndexName: usercache.ByUserIndexKeys,
	}); err != nil {
		return nil, err
	}

	authInfoResolverWrapper := webhook.NewDefaultAuthenticationInfoResolverWrapper(nil, genericConfig.LoopbackClientConfig)
	auditFlags := configflags.AuditFlags(&config.AuditConfig, configflags.ArgsWithPrefix(config.APIServerArguments, "audit-"))
	auditOpt := genericapiserveroptions.NewAuditOptions()
	fs := pflag.NewFlagSet("audit", pflag.ContinueOnError)
	auditOpt.AddFlags(fs)
	if err := fs.Parse(configflags.ToFlagSlice(auditFlags)); err != nil {
		return nil, err
	}
	if errs := auditOpt.Validate(); len(errs) > 0 {
		return nil, errors.NewAggregate(errs)
	}
	if err := auditOpt.ApplyTo(
		&genericConfig.Config,
		genericConfig.Config.LoopbackClientConfig,
		informers.kubernetesInformers,
		genericapiserveroptions.NewProcessInfo("openshift-apiserver", "openshift-apiserver"),
		&genericapiserveroptions.WebhookOptions{
			AuthInfoResolverWrapper: authInfoResolverWrapper,
			// the openshift-apiserver runs on cluster as a normal pod, accessed by a service, so it should always have access to the service network
			ServiceResolver: aggregatorapiserver.NewClusterIPServiceResolver(informers.kubernetesInformers.Core().V1().Services().Lister()),
		},
	); err != nil {
		return nil, err
	}

	projectCache, err := NewProjectCache(informers.kubernetesInformers.Core().V1().Namespaces(), kubeClientConfig, config.ProjectConfig.DefaultNodeSelector)
	if err != nil {
		return nil, err
	}
	clusterQuotaMappingController := NewClusterQuotaMappingController(informers.kubernetesInformers.Core().V1().Namespaces(), informers.quotaInformers.Quota().V1().ClusterResourceQuotas())
	discoveryClient := cacheddiscovery.NewMemCacheClient(kubeClient.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	admissionInitializer, err := openshiftadmission.NewPluginInitializer(config.ImagePolicyConfig.ExternalRegistryHostnames, config.ImagePolicyConfig.InternalRegistryHostname, config.CloudProviderFile, kubeClientConfig, informers, genericConfig.Authorization.Authorizer, restMapper, clusterQuotaMappingController)
	if err != nil {
		return nil, err
	}

	admissionConfigFile, cleanup, err := openshiftadmission.ToAdmissionConfigFile(config.AdmissionConfig.PluginConfig)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	admissionOptions := genericapiserveroptions.NewAdmissionOptions()
	admissionOptions.Decorators = admission.Decorators{
		admission.DecoratorFunc(admissionmetrics.WithControllerMetrics),
		admission.DecoratorFunc(admissiontimeout.AdmissionTimeout{Timeout: 13 * time.Second}.WithTimeout),
	}
	admissionOptions.DefaultOffPlugins = sets.String{}
	admissionOptions.RecommendedPluginOrder = openshiftadmission.OpenShiftAdmissionPlugins
	admissionOptions.Plugins = openshiftadmission.OriginAdmissionPlugins
	admissionOptions.EnablePlugins = config.AdmissionConfig.EnabledAdmissionPlugins
	admissionOptions.DisablePlugins = config.AdmissionConfig.DisabledAdmissionPlugins
	admissionOptions.ConfigFile = admissionConfigFile
	admissionOptions.ApplyTo(&genericConfig.Config, kubeInformers, kubeClientConfig, admissionInitializer)

	var externalRegistryHostname string
	if len(config.ImagePolicyConfig.ExternalRegistryHostnames) > 0 {
		externalRegistryHostname = config.ImagePolicyConfig.ExternalRegistryHostnames[0]
	}
	registryHostnameRetriever, err := registryhostname.DefaultRegistryHostnameRetriever(kubeClientConfig, externalRegistryHostname, config.ImagePolicyConfig.InternalRegistryHostname)
	if err != nil {
		return nil, err
	}

	var caData []byte
	if len(config.ImagePolicyConfig.AdditionalTrustedCA) != 0 {
		klog.V(2).Infof("Image import using additional CA path: %s", config.ImagePolicyConfig.AdditionalTrustedCA)
		var err error
		caData, err = ioutil.ReadFile(config.ImagePolicyConfig.AdditionalTrustedCA)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA bundle %s for image importing: %v", config.ImagePolicyConfig.AdditionalTrustedCA, err)
		}
	}

	subjectLocator := NewSubjectLocator(informers.GetKubernetesInformers().Rbac().V1())
	projectAuthorizationCache := NewProjectAuthorizationCache(
		subjectLocator,
		informers.GetKubernetesInformers().Core().V1().Namespaces(),
		informers.GetKubernetesInformers().Rbac().V1(),
	)

	routeAllocator, err := configprocessing.RouteAllocator(config.RoutingConfig.Subdomain)
	if err != nil {
		return nil, err
	}

	ruleResolver := NewRuleResolver(informers.kubernetesInformers.Rbac().V1())

	ret := &OpenshiftAPIConfig{
		GenericConfig: genericConfig,
		ExtraConfig: OpenshiftAPIExtraConfig{
			InformerStart:                      informers.Start,
			KubeAPIServerClientConfig:          kubeClientConfig,
			KubeInformers:                      kubeInformers, // TODO remove this and use the one from the genericconfig
			QuotaInformers:                     informers.quotaInformers,
			SecurityInformers:                  informers.securityInformers,
			RuleResolver:                       ruleResolver,
			SubjectLocator:                     subjectLocator,
			RegistryHostnameRetriever:          registryHostnameRetriever,
			AllowedRegistriesForImport:         config.ImagePolicyConfig.AllowedRegistriesForImport,
			MaxImagesBulkImportedPerRepository: config.ImagePolicyConfig.MaxImagesBulkImportedPerRepository,
			AdditionalTrustedCA:                caData,
			RouteAllocator:                     routeAllocator,
			ProjectAuthorizationCache:          projectAuthorizationCache,
			ProjectCache:                       projectCache,
			ProjectRequestTemplate:             config.ProjectConfig.ProjectRequestTemplate,
			ProjectRequestMessage:              config.ProjectConfig.ProjectRequestMessage,
			ClusterQuotaMappingController:      clusterQuotaMappingController,
			RESTMapper:                         restMapper,
			ServiceAccountMethod:               string(config.ServiceAccountOAuthGrantMethod),
		},
	}

	return ret, ret.ExtraConfig.Validate()
}

func OpenshiftHandlerChain(apiHandler http.Handler, genericConfig *genericapiserver.Config) http.Handler {
	// this is the normal kube handler chain
	handler := genericapiserver.DefaultBuildHandlerChain(apiHandler, genericConfig)

	handler = configprocessing.WithCacheControl(handler, "no-store") // protected endpoints should not be cached

	return handler
}
