package origin

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/golang/glog"
	"gopkg.in/natefinch/lumberjack.v2"

	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/internalversion"
	apifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	apiserver "k8s.io/apiserver/pkg/server"
	apiserverfilters "k8s.io/apiserver/pkg/server/filters"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	kubeapiserver "k8s.io/kubernetes/pkg/master"
	kcorestorage "k8s.io/kubernetes/pkg/registry/core/rest"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	serverhandlers "github.com/openshift/origin/pkg/cmd/server/handlers"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/plug"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	openservicebrokerserver "github.com/openshift/origin/pkg/openservicebroker/server"
	routeplugin "github.com/openshift/origin/pkg/route/allocation/simple"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	sccstorage "github.com/openshift/origin/pkg/security/registry/securitycontextconstraints/etcd"
)

func (c *MasterConfig) newOpenshiftAPIConfig(kubeAPIServerConfig apiserver.Config) (*OpenshiftAPIConfig, error) {
	// sccStorage must use the upstream RESTOptionsGetter to be in the correct location
	// this probably creates a duplicate cache, but there are not very many SCCs, so live with it to avoid further linkage
	sccStorage := sccstorage.NewREST(kubeAPIServerConfig.RESTOptionsGetter)

	// make a shallow copy to let us twiddle a few things
	// most of the config actually remains the same.  We only need to mess with a couple items
	genericConfig := kubeAPIServerConfig
	// TODO try to stop special casing these.  We should all agree on them.
	genericConfig.AdmissionControl = c.AdmissionControl
	genericConfig.RESTOptionsGetter = c.RESTOptionsGetter
	genericConfig.Authenticator = c.Authenticator
	genericConfig.Authorizer = c.Authorizer
	genericConfig.RequestContextMapper = c.RequestContextMapper

	ret := &OpenshiftAPIConfig{
		GenericConfig: &genericConfig,

		KubeClientExternal:                 c.PrivilegedLoopbackKubernetesClientsetExternal,
		KubeClientInternal:                 c.PrivilegedLoopbackKubernetesClientsetInternal,
		KubeletClientConfig:                c.KubeletClientConfig,
		KubeInternalInformers:              c.InternalKubeInformers,
		AuthorizationInformers:             c.AuthorizationInformers,
		QuotaInformers:                     c.QuotaInformers,
		SecurityInformers:                  c.SecurityInformers,
		DeprecatedOpenshiftClient:          c.PrivilegedLoopbackOpenShiftClient,
		RuleResolver:                       c.RuleResolver,
		SubjectLocator:                     c.SubjectLocator,
		LimitVerifier:                      c.LimitVerifier,
		RegistryNameFn:                     c.RegistryNameFn,
		AllowedRegistriesForImport:         c.Options.ImagePolicyConfig.AllowedRegistriesForImport,
		MaxImagesBulkImportedPerRepository: c.Options.ImagePolicyConfig.MaxImagesBulkImportedPerRepository,
		RouteAllocator:                     c.RouteAllocator(),
		ProjectAuthorizationCache:          c.ProjectAuthorizationCache,
		ProjectCache:                       c.ProjectCache,
		ProjectRequestTemplate:             c.Options.ProjectConfig.ProjectRequestTemplate,
		ProjectRequestMessage:              c.Options.ProjectConfig.ProjectRequestMessage,
		EnableBuilds:                       configapi.IsBuildEnabled(&c.Options),
		EnableTemplateServiceBroker:        c.Options.TemplateServiceBrokerConfig != nil,
		ClusterQuotaMappingController:      c.ClusterQuotaMappingController,
		SCCStorage:                         sccStorage,
	}
	if c.Options.OAuthConfig != nil {
		ret.ServiceAccountMethod = c.Options.OAuthConfig.GrantConfig.ServiceAccountMethod
	}

	return ret, ret.Validate()
}

func (c *MasterConfig) newOpenshiftNonAPIConfig(kubeAPIServerConfig apiserver.Config, controllerPlug plug.Plug) *OpenshiftNonAPIConfig {
	ret := &OpenshiftNonAPIConfig{
		GenericConfig:  &kubeAPIServerConfig,
		ControllerPlug: controllerPlug,
		EnableOAuth:    c.Options.OAuthConfig != nil,
	}
	if c.Options.OAuthConfig != nil {
		ret.MasterPublicURL = c.Options.OAuthConfig.MasterPublicURL
	}

	return ret
}

func (c *MasterConfig) newTemplateServiceBrokerConfig(kubeAPIServerConfig apiserver.Config) *openservicebrokerserver.TemplateServiceBrokerConfig {
	ret := &openservicebrokerserver.TemplateServiceBrokerConfig{
		GenericConfig:      &kubeAPIServerConfig,
		KubeClientInternal: c.PrivilegedLoopbackKubernetesClientsetInternal,
		TemplateInformers:  c.TemplateInformers,
		TemplateNamespaces: c.Options.TemplateServiceBrokerConfig.TemplateNamespaces,
	}

	return ret
}

func (c *MasterConfig) withTemplateServiceBroker(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig apiserver.Config) (apiserver.DelegationTarget, error) {
	if c.Options.TemplateServiceBrokerConfig == nil {
		return delegateAPIServer, nil
	}
	tsbConfig := c.newTemplateServiceBrokerConfig(kubeAPIServerConfig)
	tsbServer, err := tsbConfig.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, err
	}

	return tsbServer.GenericAPIServer, nil
}

func (c *MasterConfig) withAPIExtensions(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig apiserver.Config) (apiserver.DelegationTarget, apiextensionsinformers.SharedInformerFactory, error) {
	kubeAPIServerOptions, err := kubernetes.BuildKubeAPIserverOptions(c.Options)
	if err != nil {
		return nil, nil, err
	}

	apiExtensionsConfig, err := createAPIExtensionsConfig(kubeAPIServerConfig, kubeAPIServerOptions.Etcd)
	if err != nil {
		return nil, nil, err
	}
	apiExtensionsServer, err := createAPIExtensionsServer(apiExtensionsConfig, delegateAPIServer)
	if err != nil {
		return nil, nil, err
	}
	return apiExtensionsServer.GenericAPIServer, apiExtensionsServer.Informers, nil
}

func (c *MasterConfig) withNonAPIRoutes(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig apiserver.Config, controllerPlug plug.Plug) (apiserver.DelegationTarget, error) {
	openshiftNonAPIConfig := c.newOpenshiftNonAPIConfig(kubeAPIServerConfig, controllerPlug)
	openshiftNonAPIServer, err := openshiftNonAPIConfig.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	return openshiftNonAPIServer.GenericAPIServer, nil
}

func (c *MasterConfig) withOpenshiftAPI(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig apiserver.Config) (apiserver.DelegationTarget, error) {
	openshiftAPIServerConfig, err := c.newOpenshiftAPIConfig(kubeAPIServerConfig)
	if err != nil {
		return nil, err
	}
	// We need to add an openshift type to the kube's core storage until at least 3.8.  This does that by using a patch we carry.
	kcorestorage.LegacyStorageMutatorFn = sccstorage.AddSCC(openshiftAPIServerConfig.SCCStorage)

	openshiftAPIServer, err := openshiftAPIServerConfig.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	// this sets up the openapi endpoints
	preparedOpenshiftAPIServer := openshiftAPIServer.GenericAPIServer.PrepareRun()

	return preparedOpenshiftAPIServer.GenericAPIServer, nil
}

func (c *MasterConfig) withKubeAPI(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig kubeapiserver.Config, assetConfig *AssetConfig) (apiserver.DelegationTarget, error) {
	var err error
	// TODO move out of this function to somewhere we build the kubeAPIServerConfig
	kubeAPIServerConfig.GenericConfig.BuildHandlerChainFunc, err = c.buildHandlerChain(assetConfig)
	if err != nil {
		return nil, err
	}
	kubeAPIServer, err := kubeAPIServerConfig.Complete().New(delegateAPIServer, nil /*this is only used for tpr migration and we don't have any to migrate*/)
	if err != nil {
		return nil, err
	}
	// this sets up the openapi endpoints
	preparedKubeAPIServer := kubeAPIServer.GenericAPIServer.PrepareRun()

	return preparedKubeAPIServer.GenericAPIServer, nil
}

func (c *MasterConfig) withAggregator(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig apiserver.Config, apiExtensionsInformers apiextensionsinformers.SharedInformerFactory) (*aggregatorapiserver.APIAggregator, error) {
	aggregatorConfig, err := c.createAggregatorConfig(kubeAPIServerConfig)
	if err != nil {
		return nil, err
	}
	aggregatorServer, err := createAggregatorServer(aggregatorConfig, delegateAPIServer, c.InternalKubeInformers, apiExtensionsInformers)
	if err != nil {
		// we don't need special handling for innerStopCh because the aggregator server doesn't create any go routines
		return nil, err
	}

	return aggregatorServer, nil
}

// Run launches the OpenShift master by creating a kubernetes master, installing
// OpenShift APIs into it and then running it.
func (c *MasterConfig) Run(kubeAPIServerConfig *kubeapiserver.Config, assetConfig *AssetConfig, controllerPlug plug.Plug, stopCh <-chan struct{}) error {
	var err error
	var apiExtensionsInformers apiextensionsinformers.SharedInformerFactory
	var delegateAPIServer apiserver.DelegationTarget

	delegateAPIServer = apiserver.EmptyDelegate
	delegateAPIServer, err = c.withTemplateServiceBroker(delegateAPIServer, *kubeAPIServerConfig.GenericConfig)
	if err != nil {
		return err
	}
	delegateAPIServer, apiExtensionsInformers, err = c.withAPIExtensions(delegateAPIServer, *kubeAPIServerConfig.GenericConfig)
	if err != nil {
		return err
	}
	delegateAPIServer, err = c.withNonAPIRoutes(delegateAPIServer, *kubeAPIServerConfig.GenericConfig, controllerPlug)
	if err != nil {
		return err
	}
	delegateAPIServer, err = c.withOpenshiftAPI(delegateAPIServer, *kubeAPIServerConfig.GenericConfig)
	if err != nil {
		return err
	}
	delegateAPIServer, err = c.withKubeAPI(delegateAPIServer, *kubeAPIServerConfig, assetConfig)
	if err != nil {
		return err
	}
	aggregatedAPIServer, err := c.withAggregator(delegateAPIServer, *kubeAPIServerConfig.GenericConfig, apiExtensionsInformers)
	if err != nil {
		return err
	}

	go aggregatedAPIServer.GenericAPIServer.PrepareRun().Run(stopCh)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	return cmdutil.WaitForSuccessfulDial(c.TLS, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

func (c *MasterConfig) buildHandlerChain(assetConfig *AssetConfig) (func(http.Handler, *apiserver.Config) (secure http.Handler), error) {
	if c.Options.OAuthConfig != nil {
		glog.Infof("Starting OAuth2 API at %s", oauthutil.OpenShiftOAuthAPIPrefix)
	}

	if assetConfig != nil {
		publicURL, err := url.Parse(assetConfig.Options.PublicURL)
		if err != nil {
			return nil, err
		}
		glog.Infof("Starting Web Console %s", publicURL.Path)
	}

	// TODO(sttts): resync with upstream handler chain and re-use upstream filters as much as possible
	return func(apiHandler http.Handler, kc *apiserver.Config) (secure http.Handler) {
		contextMapper := c.getRequestContextMapper()

		handler := c.versionSkewFilter(apiHandler, contextMapper)
		handler = serverhandlers.AuthorizationFilter(handler, c.Authorizer, c.AuthorizationAttributeBuilder, contextMapper)
		handler = serverhandlers.ImpersonationFilter(handler, c.Authorizer, c.GroupCache, contextMapper)

		// audit handler must comes before the impersonationFilter to read the original user
		if c.Options.AuditConfig.Enabled {
			var writer io.Writer
			if len(c.Options.AuditConfig.AuditFilePath) > 0 {
				writer = &lumberjack.Logger{
					Filename:   c.Options.AuditConfig.AuditFilePath,
					MaxAge:     c.Options.AuditConfig.MaximumFileRetentionDays,
					MaxBackups: c.Options.AuditConfig.MaximumRetainedFiles,
					MaxSize:    c.Options.AuditConfig.MaximumFileSizeMegabytes,
				}
			} else {
				// backwards compatible writer to regular log
				writer = cmdutil.NewGLogWriterV(0)
			}
			handler = apifilters.WithLegacyAudit(handler, contextMapper, writer)
		}
		handler = serverhandlers.AuthenticationHandlerFilter(handler, c.Authenticator, contextMapper)
		handler = namespacingFilter(handler, contextMapper)
		handler = cacheControlFilter(handler, "no-store") // protected endpoints should not be cached

		if c.Options.OAuthConfig != nil {
			authConfig, err := BuildAuthConfig(c)
			if err != nil {
				glog.Fatalf("Failed to setup OAuth2: %v", err)
			}
			handler, err = authConfig.WithOAuth(handler)
			if err != nil {
				glog.Fatalf("Failed to setup OAuth2: %v", err)
			}
		}

		handler, err := assetConfig.WithAssets(handler)
		if err != nil {
			glog.Fatalf("Failed to setup serving of assets: %v", err)
		}

		// skip authz/n for the index handler
		handler = WithPatternsHandler(handler, apiHandler, "/", "")

		if c.WebConsoleEnabled() {
			handler = WithAssetServerRedirect(handler, c.Options.AssetConfig.PublicURL)
		}

		handler = apiserverfilters.WithCORS(handler, c.Options.CORSAllowedOrigins, nil, nil, nil, "true")
		handler = apiserverfilters.WithTimeoutForNonLongRunningRequests(handler, contextMapper, kc.LongRunningFunc)
		// TODO: MaxRequestsInFlight should be subdivided by intent, type of behavior, and speed of
		// execution - updates vs reads, long reads vs short reads, fat reads vs skinny reads.
		// NOTE: read vs. write is implemented in Kube 1.6+
		handler = apiserverfilters.WithMaxInFlightLimit(handler, kc.MaxRequestsInFlight, kc.MaxMutatingRequestsInFlight, contextMapper, kc.LongRunningFunc)
		handler = apifilters.WithRequestInfo(handler, apiserver.NewRequestInfoResolver(kc), contextMapper)
		handler = apirequest.WithRequestContext(handler, contextMapper)
		handler = apiserverfilters.WithPanicRecovery(handler)

		return handler
	}, nil
}

// InitializeObjects ensures objects in Kubernetes and etcd are properly populated.
// Requires a Kube client to be established and that etcd be started.
func (c *MasterConfig) InitializeObjects() {
	// Create required policy rules if needed
	c.ensureComponentAuthorizationRules()
	// Ensure the default SCCs are created
	c.ensureDefaultSecurityContextConstraints()
	// Bind default roles for service accounts in the default namespace if needed
	c.ensureDefaultNamespaceServiceAccountRoles()
	// Create the infra namespace
	c.ensureOpenShiftInfraNamespace()
	// Create the shared resource namespace
	c.ensureOpenShiftSharedResourcesNamespace()
}

// getRequestContextMapper returns a mapper from requests to contexts, initializing it if needed
func (c *MasterConfig) getRequestContextMapper() apirequest.RequestContextMapper {
	if c.RequestContextMapper == nil {
		c.RequestContextMapper = apirequest.NewRequestContextMapper()
	}
	return c.RequestContextMapper
}

// RouteAllocator returns a route allocation controller.
func (c *MasterConfig) RouteAllocator() *routeallocationcontroller.RouteAllocationController {
	osclient, kclient := c.RouteAllocatorClients()
	factory := routeallocationcontroller.RouteAllocationControllerFactory{
		OSClient:   osclient,
		KubeClient: kclient,
	}

	plugin, err := routeplugin.NewSimpleAllocationPlugin(c.Options.RoutingConfig.Subdomain)
	if err != nil {
		glog.Fatalf("Route plugin initialization failed: %v", err)
	}

	return factory.Create(plugin)
}

// env returns an environment variable, or the defaultValue if it is not set.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

func WithPatternsHandler(handler http.Handler, patternHandler http.Handler, patterns ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, p := range patterns {
			if req.URL.Path == p {
				patternHandler.ServeHTTP(w, req)
				return
			}
		}
		handler.ServeHTTP(w, req)
	})
}
