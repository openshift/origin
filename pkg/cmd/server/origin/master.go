package origin

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"gopkg.in/natefinch/lumberjack.v2"

	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/internalversion"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	auditpolicy "k8s.io/apiserver/pkg/audit/policy"
	apifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	apiserver "k8s.io/apiserver/pkg/server"
	apiserverfilters "k8s.io/apiserver/pkg/server/filters"
	auditlog "k8s.io/apiserver/plugin/pkg/audit/log"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	kubeapiserver "k8s.io/kubernetes/pkg/master"
	kcorestorage "k8s.io/kubernetes/pkg/registry/core/rest"

	assetapiserver "github.com/openshift/origin/pkg/assets/apiserver"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	serverhandlers "github.com/openshift/origin/pkg/cmd/server/handlers"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/plug"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	routeplugin "github.com/openshift/origin/pkg/route/allocation/simple"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	sccstorage "github.com/openshift/origin/pkg/security/registry/securitycontextconstraints/etcd"
	"github.com/openshift/origin/pkg/user/cache"
)

const (
	openShiftOAuthAPIPrefix      = "/oauth"
	openShiftLoginPrefix         = "/login"
	openShiftOAuthCallbackPrefix = "/oauth2callback"
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

func (c *MasterConfig) withKubeAPI(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig kubeapiserver.Config) (apiserver.DelegationTarget, error) {
	var err error
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

func (c *MasterConfig) newAssetServerHandler() (http.Handler, error) {
	if !c.WebConsoleEnabled() || c.WebConsoleStandalone() {
		return http.NotFoundHandler(), nil
	}

	config, err := NewAssetServerConfigFromMasterConfig(c.Options)
	if err != nil {
		return nil, err
	}
	assetServer, err := config.Complete().New(apiserver.EmptyDelegate)
	if err != nil {
		return nil, err
	}
	return assetServer.GenericAPIServer.PrepareRun().GenericAPIServer.Handler.FullHandlerChain, nil
}

func (c *MasterConfig) newOAuthServerHandler() (http.Handler, map[string]apiserver.PostStartHookFunc, error) {
	if c.Options.OAuthConfig == nil {
		return http.NotFoundHandler(), nil, nil
	}

	config, err := NewOAuthServerConfigFromMasterConfig(c)
	if err != nil {
		return nil, nil, err
	}
	oauthServer, err := config.Complete().New(apiserver.EmptyDelegate)
	if err != nil {
		return nil, nil, err
	}
	return oauthServer.GenericAPIServer.PrepareRun().GenericAPIServer.Handler.FullHandlerChain,
		map[string]apiserver.PostStartHookFunc{
			"oauth.openshift.io-EnsureBootstrapOAuthClients": config.EnsureBootstrapOAuthClients,
		},
		nil
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
func (c *MasterConfig) Run(kubeAPIServerConfig *kubeapiserver.Config, controllerPlug plug.Plug, stopCh <-chan struct{}) error {
	var err error
	var apiExtensionsInformers apiextensionsinformers.SharedInformerFactory
	var delegateAPIServer apiserver.DelegationTarget
	var extraPostStartHooks map[string]apiserver.PostStartHookFunc

	kubeAPIServerConfig.GenericConfig.BuildHandlerChainFunc, extraPostStartHooks, err = c.buildHandlerChain()
	if err != nil {
		return err
	}

	delegateAPIServer = apiserver.EmptyDelegate
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
	delegateAPIServer, err = c.withKubeAPI(delegateAPIServer, *kubeAPIServerConfig)
	if err != nil {
		return err
	}
	aggregatedAPIServer, err := c.withAggregator(delegateAPIServer, *kubeAPIServerConfig.GenericConfig, apiExtensionsInformers)
	if err != nil {
		return err
	}

	// Start the audit backend before any request comes in. This means we cannot turn it into a
	// post start hook because without calling Backend.Run the Backend.ProcessEvents call might block.
	if c.AuditBackend != nil {
		if err := c.AuditBackend.Run(stopCh); err != nil {
			return fmt.Errorf("failed to run the audit backend: %v", err)
		}
	}

	// add post-start hooks
	aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie("template.openshift.io-sharednamespace", c.ensureOpenShiftSharedResourcesNamespace)
	aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie("authorization.openshift.io-bootstrapclusterroles", bootstrappolicy.Policy().EnsureRBACPolicy())
	for name, fn := range extraPostStartHooks {
		aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie(name, fn)
	}

	go aggregatedAPIServer.GenericAPIServer.PrepareRun().Run(stopCh)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	return cmdutil.WaitForSuccessfulDial(true, aggregatedAPIServer.GenericAPIServer.SecureServingInfo.BindNetwork, aggregatedAPIServer.GenericAPIServer.SecureServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

func (c *MasterConfig) buildHandlerChain() (func(apiHandler http.Handler, kc *apiserver.Config) http.Handler, map[string]apiserver.PostStartHookFunc, error) {
	assetServerHandler, err := c.newAssetServerHandler()
	if err != nil {
		return nil, nil, err
	}
	oauthServerHandler, extraPostStartHooks, err := c.newOAuthServerHandler()
	if err != nil {
		return nil, nil, err
	}

	return func(apiHandler http.Handler, genericConfig *apiserver.Config) http.Handler {
			// these are after the kube handler
			handler := c.versionSkewFilter(apiHandler, genericConfig.RequestContextMapper)
			handler = namespacingFilter(handler, genericConfig.RequestContextMapper)

			// these are all equivalent to the kube handler chain
			///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
			handler = serverhandlers.AuthorizationFilter(handler, c.Authorizer, c.AuthorizationAttributeBuilder, genericConfig.RequestContextMapper)
			handler = serverhandlers.ImpersonationFilter(handler, c.Authorizer, cache.NewGroupCache(c.UserInformers.User().InternalVersion().Groups()), genericConfig.RequestContextMapper)
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
				c.AuditBackend = auditlog.NewBackend(writer)
				auditPolicyChecker := auditpolicy.NewChecker(&auditinternal.Policy{
					// This is for backwards compatibility maintaining the old visibility, ie. just
					// raw overview of the requests comming in.
					Rules: []auditinternal.PolicyRule{{Level: auditinternal.LevelMetadata}},
				})
				handler = apifilters.WithAudit(handler, genericConfig.RequestContextMapper, c.AuditBackend, auditPolicyChecker, genericConfig.LongRunningFunc)
			}
			handler = serverhandlers.AuthenticationHandlerFilter(handler, c.Authenticator, genericConfig.RequestContextMapper)
			handler = apiserverfilters.WithCORS(handler, c.Options.CORSAllowedOrigins, nil, nil, nil, "true")
			handler = apiserverfilters.WithTimeoutForNonLongRunningRequests(handler, genericConfig.RequestContextMapper, genericConfig.LongRunningFunc)
			// TODO: MaxRequestsInFlight should be subdivided by intent, type of behavior, and speed of
			// execution - updates vs reads, long reads vs short reads, fat reads vs skinny reads.
			// NOTE: read vs. write is implemented in Kube 1.6+
			handler = apiserverfilters.WithMaxInFlightLimit(handler, genericConfig.MaxRequestsInFlight, genericConfig.MaxMutatingRequestsInFlight, genericConfig.RequestContextMapper, genericConfig.LongRunningFunc)
			handler = apifilters.WithRequestInfo(handler, apiserver.NewRequestInfoResolver(genericConfig), genericConfig.RequestContextMapper)
			handler = apirequest.WithRequestContext(handler, genericConfig.RequestContextMapper)
			handler = apiserverfilters.WithPanicRecovery(handler)
			///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

			// these handlers are all before the normal kube chain
			handler = cacheControlFilter(handler, "no-store") // protected endpoints should not be cached

			if c.WebConsoleEnabled() {
				handler = assetapiserver.WithAssetServerRedirect(handler, c.Options.AssetConfig.PublicURL)
			}
			// these handlers are actually separate API servers which have their own handler chains.
			// our server embeds these
			handler = c.withConsoleRedirection(handler, assetServerHandler, c.Options.AssetConfig)
			handler = c.withOAuthRedirection(handler, oauthServerHandler)

			return handler
		},
		extraPostStartHooks,
		nil
}

func (c *MasterConfig) withConsoleRedirection(handler, assetServerHandler http.Handler, assetConfig *configapi.AssetConfig) http.Handler {
	if assetConfig == nil {
		return handler
	}
	if !c.WebConsoleEnabled() || c.WebConsoleStandalone() {
		return handler
	}

	publicURL, err := url.Parse(assetConfig.PublicURL)
	if err != nil {
		// fails validation before here
		glog.Fatal(err)
	}
	// path always ends in a slash or the
	prefix := publicURL.Path
	lastIndex := len(publicURL.Path) - 1
	if publicURL.Path[lastIndex] == '/' {
		prefix = publicURL.Path[0:lastIndex]
	}

	glog.Infof("Starting Web Console %s", assetConfig.PublicURL)
	return WithPatternPrefixHandler(handler, assetServerHandler, prefix)
}

func (c *MasterConfig) withOAuthRedirection(handler, oauthServerHandler http.Handler) http.Handler {
	if c.Options.OAuthConfig == nil {
		return handler
	}

	glog.Infof("Starting OAuth2 API at %s", oauthutil.OpenShiftOAuthAPIPrefix)
	return WithPatternPrefixHandler(handler, oauthServerHandler, openShiftOAuthAPIPrefix, openShiftLoginPrefix, openShiftOAuthCallbackPrefix)
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

func WithPatternPrefixHandler(handler http.Handler, patternHandler http.Handler, prefixes ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, p := range prefixes {
			if strings.HasPrefix(req.URL.Path, p) {
				patternHandler.ServeHTTP(w, req)
				return
			}
		}
		handler.ServeHTTP(w, req)
	})
}
