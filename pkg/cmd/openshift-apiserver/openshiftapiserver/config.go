package openshiftapiserver

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	swagger "github.com/emicklei/go-restful-swagger12"
	"github.com/golang/glog"

	"k8s.io/apiserver/pkg/admission"
	admissionmetrics "k8s.io/apiserver/pkg/admission/metrics"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericapiserveroptions "k8s.io/apiserver/pkg/server/options"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/origin/pkg/admission/namespaceconditions"
	"github.com/openshift/origin/pkg/api/legacy"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	originadmission "github.com/openshift/origin/pkg/cmd/server/origin/admission"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
	sccstorage "github.com/openshift/origin/pkg/security/apiserver/registry/securitycontextconstraints/etcd"
	usercache "github.com/openshift/origin/pkg/user/cache"
	"github.com/openshift/origin/pkg/version"
)

func NewOpenshiftAPIConfig(newConfig *configapi.OpenshiftAPIServerConfig) (*OpenshiftAPIConfig, error) {
	kubeClientConfig, err := configapi.GetClientConfig(newConfig.MasterClients.OpenShiftLoopbackKubeConfig, newConfig.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return nil, err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, 10*time.Minute)

	openshiftVersion := version.Get()

	backend, policyChecker, err := configprocessing.GetAuditConfig(newConfig.AuditConfig)
	if err != nil {
		return nil, err
	}
	restOptsGetter, err := NewRESTOptionsGetter(newConfig.APIServerArguments, newConfig.EtcdClientInfo, newConfig.StoragePrefix)
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

	genericConfig.CorsAllowedOriginList = newConfig.CORSAllowedOrigins
	genericConfig.Version = &openshiftVersion
	// we don't use legacy audit anymore
	genericConfig.LegacyAuditWriter = nil
	genericConfig.AuditBackend = backend
	genericConfig.AuditPolicyChecker = policyChecker
	genericConfig.ExternalAddress = "apiserver.openshift-apiserver.svc"
	genericConfig.BuildHandlerChainFunc = OpenshiftHandlerChain
	genericConfig.LegacyAPIGroupPrefixes = configprocessing.LegacyAPIGroupPrefixes
	genericConfig.RequestInfoResolver = configprocessing.OpenshiftRequestInfoResolver()
	genericConfig.OpenAPIConfig = configprocessing.DefaultOpenAPIConfig(nil)
	genericConfig.SwaggerConfig = defaultSwaggerConfig()
	genericConfig.RESTOptionsGetter = restOptsGetter
	// previously overwritten.  I don't know why
	genericConfig.RequestTimeout = time.Duration(60) * time.Second
	genericConfig.MinRequestTimeout = newConfig.ServingInfo.RequestTimeoutSeconds
	genericConfig.MaxRequestsInFlight = newConfig.ServingInfo.MaxRequestsInFlight
	genericConfig.MaxMutatingRequestsInFlight = newConfig.ServingInfo.MaxRequestsInFlight / 2
	genericConfig.LongRunningFunc = configprocessing.IsLongRunningRequest

	// I'm just hoping this works.  I don't think we use it.
	//MergedResourceConfig *serverstore.ResourceConfig

	servingOptions, err := configprocessing.ToServingOptions(newConfig.ServingInfo)
	if err != nil {
		return nil, err
	}
	if err := servingOptions.ApplyTo(&genericConfig.Config); err != nil {
		return nil, err
	}
	authenticationOptions := genericapiserveroptions.NewDelegatingAuthenticationOptions()
	authenticationOptions.RemoteKubeConfigFile = newConfig.MasterClients.OpenShiftLoopbackKubeConfig
	if err := authenticationOptions.ApplyTo(&genericConfig.Authentication, genericConfig.SecureServing, genericConfig.OpenAPIConfig); err != nil {
		return nil, err
	}
	authorizationOptions := genericapiserveroptions.NewDelegatingAuthorizationOptions()
	authorizationOptions.RemoteKubeConfigFile = newConfig.MasterClients.OpenShiftLoopbackKubeConfig
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
	projectCache, err := NewProjectCache(informers.internalKubernetesInformers.Core().InternalVersion().Namespaces(), kubeClientConfig, newConfig.ProjectConfig.DefaultNodeSelector)
	if err != nil {
		return nil, err
	}
	clusterQuotaMappingController := NewClusterQuotaMappingController(informers.internalKubernetesInformers.Core().InternalVersion().Namespaces(), informers.quotaInformers.Quota().InternalVersion().ClusterResourceQuotas())
	discoveryClient := cacheddiscovery.NewMemCacheClient(kubeClient.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	cloudConfigFile, err := configprocessing.GetCloudProviderConfigFile(newConfig.APIServerArguments)
	if err != nil {
		return nil, err
	}
	admissionInitializer, err := originadmission.NewPluginInitializer(newConfig.ImagePolicyConfig.ExternalRegistryHostname, newConfig.ImagePolicyConfig.InternalRegistryHostname, cloudConfigFile, newConfig.JenkinsPipelineConfig, kubeClientConfig, informers, genericConfig.Authorization.Authorizer, projectCache, restMapper, clusterQuotaMappingController)
	if err != nil {
		return nil, err
	}
	namespaceLabelDecorator := namespaceconditions.NamespaceLabelConditions{
		NamespaceClient: kubeClient.CoreV1(),
		NamespaceLister: informers.GetKubernetesInformers().Core().V1().Namespaces().Lister(),

		SkipLevelZeroNames: originadmission.SkipRunLevelZeroPlugins,
		SkipLevelOneNames:  originadmission.SkipRunLevelOnePlugins,
	}
	admissionDecorators := admission.Decorators{
		admission.DecoratorFunc(namespaceLabelDecorator.WithNamespaceLabelConditions),
		admission.DecoratorFunc(admissionmetrics.WithControllerMetrics),
	}
	genericConfig.AdmissionControl, err = originadmission.NewAdmissionChains([]string{}, newConfig.AdmissionPluginConfig, []string{}, admissionInitializer, admissionDecorators)
	if err != nil {
		return nil, err
	}

	registryHostnameRetriever, err := registryhostname.DefaultRegistryHostnameRetriever(kubeClientConfig, newConfig.ImagePolicyConfig.ExternalRegistryHostname, newConfig.ImagePolicyConfig.InternalRegistryHostname)
	if err != nil {
		return nil, err
	}
	imageLimitVerifier := ImageLimitVerifier(informers.internalKubernetesInformers.Core().InternalVersion().LimitRanges())

	// sccStorage must use the upstream RESTOptionsGetter to be in the correct location
	// this probably creates a duplicate cache, but there are not very many SCCs, so live with it to avoid further linkage
	sccStorage := sccstorage.NewREST(genericConfig.RESTOptionsGetter)

	var caData []byte
	if len(newConfig.ImagePolicyConfig.AdditionalTrustedCA) != 0 {
		glog.V(2).Infof("Image import using additional CA path: %s", newConfig.ImagePolicyConfig.AdditionalTrustedCA)
		var err error
		caData, err = ioutil.ReadFile(newConfig.ImagePolicyConfig.AdditionalTrustedCA)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA bundle %s for image importing: %v", newConfig.ImagePolicyConfig.AdditionalTrustedCA, err)
		}
	}

	subjectLocator := NewSubjectLocator(informers.GetKubernetesInformers().Rbac().V1())
	projectAuthorizationCache := NewProjectAuthorizationCache(
		subjectLocator,
		informers.GetInternalKubernetesInformers().Core().InternalVersion().Namespaces().Informer(),
		informers.GetKubernetesInformers().Rbac().V1(),
	)

	routeAllocator, err := configprocessing.RouteAllocator(newConfig.RoutingConfig)
	if err != nil {
		return nil, err
	}

	ruleResolver := NewRuleResolver(informers.kubernetesInformers.Rbac().V1())

	ret := &OpenshiftAPIConfig{
		GenericConfig: genericConfig,
		ExtraConfig: OpenshiftAPIExtraConfig{
			InformerStart:                      informers.Start,
			KubeAPIServerClientConfig:          kubeClientConfig,
			KubeInternalInformers:              informers.internalKubernetesInformers,
			KubeInformers:                      kubeInformers, // TODO remove this and use the one from the genericconfig
			QuotaInformers:                     informers.quotaInformers,
			SecurityInformers:                  informers.securityInformers,
			RuleResolver:                       ruleResolver,
			SubjectLocator:                     subjectLocator,
			LimitVerifier:                      imageLimitVerifier,
			RegistryHostnameRetriever:          registryHostnameRetriever,
			AllowedRegistriesForImport:         newConfig.ImagePolicyConfig.AllowedRegistriesForImport,
			MaxImagesBulkImportedPerRepository: newConfig.ImagePolicyConfig.MaxImagesBulkImportedPerRepository,
			AdditionalTrustedCA:                caData,
			RouteAllocator:                     routeAllocator,
			ProjectAuthorizationCache:          projectAuthorizationCache,
			ProjectCache:                       projectCache,
			ProjectRequestTemplate:             newConfig.ProjectConfig.ProjectRequestTemplate,
			ProjectRequestMessage:              newConfig.ProjectConfig.ProjectRequestMessage,
			ClusterQuotaMappingController:      clusterQuotaMappingController,
			RESTMapper:                         restMapper,
			SCCStorage:                         sccStorage,
			ServiceAccountMethod:               newConfig.ServiceAccountOAuthGrantMethod,
		},
	}

	return ret, ret.ExtraConfig.Validate()
}

var apiInfo = map[string]swagger.Info{
	legacy.RESTPrefix + "/" + legacy.GroupVersion.Version: {
		Title:       "OpenShift v1 REST API",
		Description: `The OpenShift API exposes operations for managing an enterprise Kubernetes cluster, including security and user management, application deployments, image and source builds, HTTP(s) routing, and project management.`,
	},
}

// customizeSwaggerDefinition applies selective patches to the swagger API docs
// TODO: move most of these upstream or to go-restful
func customizeSwaggerDefinition(apiList *swagger.ApiDeclarationList) {
	for path, info := range apiInfo {
		if dec, ok := apiList.At(path); ok {
			if len(info.Title) > 0 {
				dec.Info.Title = info.Title
			}
			if len(info.Description) > 0 {
				dec.Info.Description = info.Description
			}
			apiList.Put(path, dec)
		} else {
			glog.Warningf("No API exists for predefined swagger description %s", path)
		}
	}
	for _, version := range []string{legacy.RESTPrefix + "/" + legacy.GroupVersion.Version} {
		apiDeclaration, _ := apiList.At(version)
		models := &apiDeclaration.Models

		model, _ := models.At("runtime.RawExtension")
		model.Required = []string{}
		model.Properties = swagger.ModelPropertyList{}
		model.Description = "this may be any JSON object with a 'kind' and 'apiVersion' field; and is preserved unmodified by processing"
		models.Put("runtime.RawExtension", model)

		model, _ = models.At("patch.Object")
		model.Description = "represents an object patch, which may be any of: JSON patch (RFC 6902), JSON merge patch (RFC 7396), or the Kubernetes strategic merge patch"
		models.Put("patch.Object", model)

		apiDeclaration.Models = *models
		apiList.Put(version, apiDeclaration)
	}
}

func defaultSwaggerConfig() *swagger.Config {
	ret := genericapiserver.DefaultSwaggerConfig()
	ret.PostBuildHandler = customizeSwaggerDefinition
	return ret
}

func OpenshiftHandlerChain(apiHandler http.Handler, genericConfig *genericapiserver.Config) http.Handler {
	// this is the normal kube handler chain
	handler := genericapiserver.DefaultBuildHandlerChain(apiHandler, genericConfig)

	handler = configprocessing.WithCacheControl(handler, "no-store") // protected endpoints should not be cached

	return handler
}
