package origin

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"

	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/internalversion"
	apiserver "k8s.io/apiserver/pkg/server"
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
	genericConfig.RESTOptionsGetter = c.RESTOptionsGetter

	ret := &OpenshiftAPIConfig{
		GenericConfig: &genericConfig,

		KubeClientInternal:                 c.PrivilegedLoopbackKubernetesClientsetInternal,
		KubeletClientConfig:                c.KubeletClientConfig,
		KubeInternalInformers:              c.InternalKubeInformers,
		QuotaInformers:                     c.QuotaInformers,
		SecurityInformers:                  c.SecurityInformers,
		RuleResolver:                       c.RuleResolver,
		SubjectLocator:                     c.SubjectLocator,
		LimitVerifier:                      c.LimitVerifier,
		RegistryHostnameRetriever:          c.RegistryHostnameRetriever,
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

func (c *MasterConfig) newAssetServerHandler(genericConfig *apiserver.Config) (http.Handler, error) {
	if !c.WebConsoleEnabled() || c.WebConsoleStandalone() {
		return http.NotFoundHandler(), nil
	}

	config, err := NewAssetServerConfigFromMasterConfig(c.Options)
	if err != nil {
		return nil, err
	}
	config.GenericConfig.AuditBackend = genericConfig.AuditBackend
	config.GenericConfig.AuditPolicyChecker = genericConfig.AuditPolicyChecker
	assetServer, err := config.Complete().New(apiserver.EmptyDelegate)
	if err != nil {
		return nil, err
	}
	return assetServer.GenericAPIServer.PrepareRun().GenericAPIServer.Handler.FullHandlerChain, nil
}

func (c *MasterConfig) newOAuthServerHandler(genericConfig *apiserver.Config) (http.Handler, map[string]apiserver.PostStartHookFunc, error) {
	if c.Options.OAuthConfig == nil {
		return http.NotFoundHandler(), nil, nil
	}

	config, err := NewOAuthServerConfigFromMasterConfig(c)
	if err != nil {
		return nil, nil, err
	}
	config.GenericConfig.AuditBackend = genericConfig.AuditBackend
	config.GenericConfig.AuditPolicyChecker = genericConfig.AuditPolicyChecker
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
func (c *MasterConfig) Run(controllerPlug plug.Plug, stopCh <-chan struct{}) error {
	var err error
	var apiExtensionsInformers apiextensionsinformers.SharedInformerFactory
	var delegateAPIServer apiserver.DelegationTarget
	var extraPostStartHooks map[string]apiserver.PostStartHookFunc

	c.kubeAPIServerConfig.GenericConfig.BuildHandlerChainFunc, extraPostStartHooks, err = c.buildHandlerChain(c.kubeAPIServerConfig.GenericConfig)
	if err != nil {
		return err
	}

	delegateAPIServer = apiserver.EmptyDelegate
	delegateAPIServer, apiExtensionsInformers, err = c.withAPIExtensions(delegateAPIServer, *c.kubeAPIServerConfig.GenericConfig)
	if err != nil {
		return err
	}
	delegateAPIServer, err = c.withNonAPIRoutes(delegateAPIServer, *c.kubeAPIServerConfig.GenericConfig, controllerPlug)
	if err != nil {
		return err
	}
	delegateAPIServer, err = c.withOpenshiftAPI(delegateAPIServer, *c.kubeAPIServerConfig.GenericConfig)
	if err != nil {
		return err
	}
	delegateAPIServer, err = c.withKubeAPI(delegateAPIServer, *c.kubeAPIServerConfig)
	if err != nil {
		return err
	}
	aggregatedAPIServer, err := c.withAggregator(delegateAPIServer, *c.kubeAPIServerConfig.GenericConfig, apiExtensionsInformers)
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
	for name, fn := range c.additionalPostStartHooks {
		aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie(name, fn)
	}
	for name, fn := range extraPostStartHooks {
		aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie(name, fn)
	}

	go aggregatedAPIServer.GenericAPIServer.PrepareRun().Run(stopCh)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	return cmdutil.WaitForSuccessfulDial(true, aggregatedAPIServer.GenericAPIServer.SecureServingInfo.BindNetwork, aggregatedAPIServer.GenericAPIServer.SecureServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

func (c *MasterConfig) buildHandlerChain(genericConfig *apiserver.Config) (func(apiHandler http.Handler, kc *apiserver.Config) http.Handler, map[string]apiserver.PostStartHookFunc, error) {
	assetServerHandler, err := c.newAssetServerHandler(genericConfig)
	if err != nil {
		return nil, nil, err
	}
	oauthServerHandler, extraPostStartHooks, err := c.newOAuthServerHandler(genericConfig)
	if err != nil {
		return nil, nil, err
	}

	return func(apiHandler http.Handler, genericConfig *apiserver.Config) http.Handler {
			// these are after the kube handler
			handler := c.versionSkewFilter(apiHandler, genericConfig.RequestContextMapper)
			handler = namespacingFilter(handler, genericConfig.RequestContextMapper)

			// this is the normal kube handler chain
			handler = apiserver.DefaultBuildHandlerChain(handler, genericConfig)

			// these handlers are all before the normal kube chain
			handler = serverhandlers.TranslateLegacyScopeImpersonation(handler)
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
	factory := routeallocationcontroller.RouteAllocationControllerFactory{
		KubeClient: c.PrivilegedLoopbackKubernetesClientsetInternal,
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
