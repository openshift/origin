package origin

import (
	"fmt"
	"io/ioutil"
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

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	routeplugin "github.com/openshift/origin/pkg/route/allocation/simple"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	sccstorage "github.com/openshift/origin/pkg/security/registry/securitycontextconstraints/etcd"
	"github.com/openshift/origin/pkg/util/httprequest"
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
		GenericConfig: &apiserver.RecommendedConfig{Config: genericConfig},
		ExtraConfig: OpenshiftAPIExtraConfig{
			KubeAPIServerClientConfig:          &c.PrivilegedLoopbackClientConfig,
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
			ClusterQuotaMappingController:      c.ClusterQuotaMappingController,
			SCCStorage:                         sccStorage,
		},
	}
	if c.Options.OAuthConfig != nil {
		ret.ExtraConfig.ServiceAccountMethod = c.Options.OAuthConfig.GrantConfig.ServiceAccountMethod
	}

	return ret, ret.ExtraConfig.Validate()
}

func (c *MasterConfig) newOpenshiftNonAPIConfig(kubeAPIServerConfig apiserver.Config) *OpenshiftNonAPIConfig {
	ret := &OpenshiftNonAPIConfig{
		GenericConfig: &apiserver.RecommendedConfig{
			Config:                kubeAPIServerConfig,
			SharedInformerFactory: c.ClientGoKubeInformers,
		},
		ExtraConfig: NonAPIExtraConfig{
			EnableOAuth: c.Options.OAuthConfig != nil,
		},
	}
	if c.Options.OAuthConfig != nil {
		ret.ExtraConfig.MasterPublicURL = c.Options.OAuthConfig.MasterPublicURL
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

func (c *MasterConfig) withNonAPIRoutes(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig apiserver.Config) (apiserver.DelegationTarget, error) {
	openshiftNonAPIConfig := c.newOpenshiftNonAPIConfig(kubeAPIServerConfig)
	openshiftNonAPIServer, err := openshiftNonAPIConfig.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	return openshiftNonAPIServer.GenericAPIServer, nil
}

func (c *MasterConfig) withOpenshiftAPI(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig apiserver.Config) (*apiserver.GenericAPIServer, error) {
	openshiftAPIServerConfig, err := c.newOpenshiftAPIConfig(kubeAPIServerConfig)
	if err != nil {
		return nil, err
	}
	// We need to add an openshift type to the kube's core storage until at least 3.8.  This does that by using a patch we carry.
	kcorestorage.LegacyStorageMutatorFn = sccstorage.AddSCC(openshiftAPIServerConfig.ExtraConfig.SCCStorage)

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
	kubeAPIServer, err := kubeAPIServerConfig.Complete(c.ClientGoKubeInformers).New(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	// this sets up the openapi endpoints
	preparedKubeAPIServer := kubeAPIServer.GenericAPIServer.PrepareRun()

	// this remains here and separate so that you can check both kube and openshift levels
	// TODO make this is a proxy at some point
	addOpenshiftVersionRoute(kubeAPIServer.GenericAPIServer.Handler.GoRestfulContainer, "/version/openshift")

	return preparedKubeAPIServer.GenericAPIServer, nil
}

func (c *MasterConfig) newWebConsoleProxy() (http.Handler, error) {
	caBundle, err := ioutil.ReadFile(c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile)
	if err != nil {
		return nil, err
	}
	proxyHandler, err := NewServiceProxyHandler("webconsole", "openshift-web-console", aggregatorapiserver.NewClusterIPServiceResolver(c.ClientGoKubeInformers.Core().V1().Services().Lister()), caBundle, "OpenShift web console")
	if err != nil {
		return nil, err
	}
	return proxyHandler, nil
}

func (c *MasterConfig) newOAuthServerHandler(genericConfig *apiserver.Config) (http.Handler, map[string]apiserver.PostStartHookFunc, error) {
	if c.Options.OAuthConfig == nil {
		return http.NotFoundHandler(), nil, nil
	}

	config, err := NewOAuthServerConfigFromMasterConfig(c, genericConfig.SecureServingInfo.Listener)
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
			"oauth.openshift.io-StartOAuthClientsBootstrapping": config.StartOAuthClientsBootstrapping,
		},
		nil
}

func (c *MasterConfig) withAggregator(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig apiserver.Config, apiExtensionsInformers apiextensionsinformers.SharedInformerFactory) (*aggregatorapiserver.APIAggregator, error) {
	aggregatorConfig, err := c.createAggregatorConfig(kubeAPIServerConfig)
	if err != nil {
		return nil, err
	}
	aggregatorServer, err := createAggregatorServer(aggregatorConfig, delegateAPIServer, apiExtensionsInformers)
	if err != nil {
		// we don't need special handling for innerStopCh because the aggregator server doesn't create any go routines
		return nil, err
	}

	return aggregatorServer, nil
}

// Run launches the OpenShift master by creating a kubernetes master, installing
// OpenShift APIs into it and then running it.
// TODO this method only exists to support the old openshift start path.  It should be removed a little ways into 3.10.
func (c *MasterConfig) Run(stopCh <-chan struct{}) error {
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
	delegateAPIServer, err = c.withNonAPIRoutes(delegateAPIServer, *c.kubeAPIServerConfig.GenericConfig)
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

	if GRPCThreadLimit > 0 {
		if err := aggregatedAPIServer.GenericAPIServer.AddHealthzChecks(NewGRPCStuckThreads()); err != nil {
			return err
		}
		// We start a separate gofunc that will panic for us because nothing is watching healthz at the moment.
		PanicOnGRPCStuckThreads(10*time.Second, stopCh)
	}

	// add post-start hooks
	aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie("authorization.openshift.io-bootstrapclusterroles", bootstrappolicy.Policy().EnsureRBACPolicy())
	aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie("authorization.openshift.io-ensureopenshift-infra", ensureOpenShiftInfraNamespace)
	aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie("quota.openshift.io-clusterquotamapping", c.startClusterQuotaMapping)
	for name, fn := range c.additionalPostStartHooks {
		aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie(name, fn)
	}
	for name, fn := range extraPostStartHooks {
		aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie(name, fn)
	}

	go aggregatedAPIServer.GenericAPIServer.PrepareRun().Run(stopCh)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	return cmdutil.WaitForSuccessfulDial(true, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

func (c *MasterConfig) RunKubeAPIServer(stopCh <-chan struct{}) error {
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
	delegateAPIServer, err = c.withNonAPIRoutes(delegateAPIServer, *c.kubeAPIServerConfig.GenericConfig)
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

	aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie("authorization.openshift.io-bootstrapclusterroles", bootstrappolicy.Policy().EnsureRBACPolicy())
	aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie("authorization.openshift.io-ensureopenshift-infra", ensureOpenShiftInfraNamespace)
	aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie("quota.openshift.io-clusterquotamapping", c.startClusterQuotaMapping)
	// add post-start hooks
	for name, fn := range c.additionalPostStartHooks {
		aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie(name, fn)
	}
	for name, fn := range extraPostStartHooks {
		aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie(name, fn)
	}

	go aggregatedAPIServer.GenericAPIServer.PrepareRun().Run(stopCh)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	return cmdutil.WaitForSuccessfulDial(true, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

func (c *MasterConfig) RunOpenShift(stopCh <-chan struct{}) error {
	// TODO rewrite the authenticator and authorizer here to use the webhooks.  I think we'll be able to manage this
	// using the existing client connections since they'll all point to the kube-apiserver, but some new separation may be required
	// to handle the distinction between loopback and kube-apiserver

	// the openshift apiserver shouldn't need to host these and they make us crashloop
	c.kubeAPIServerConfig.GenericConfig.EnableSwaggerUI = false
	c.kubeAPIServerConfig.GenericConfig.SwaggerConfig = nil
	c.kubeAPIServerConfig.GenericConfig.BuildHandlerChainFunc = openshiftHandlerChain

	openshiftAPIServer, err := c.withOpenshiftAPI(apiserver.EmptyDelegate, *c.kubeAPIServerConfig.GenericConfig)
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
	openshiftAPIServer.AddPostStartHookOrDie("quota.openshift.io-clusterquotamapping", c.startClusterQuotaMapping)
	for name, fn := range c.additionalPostStartHooks {
		openshiftAPIServer.AddPostStartHookOrDie(name, fn)
	}

	go openshiftAPIServer.PrepareRun().Run(stopCh)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	return cmdutil.WaitForSuccessfulDial(true, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

func (c *MasterConfig) buildHandlerChain(genericConfig *apiserver.Config) (func(apiHandler http.Handler, kc *apiserver.Config) http.Handler, map[string]apiserver.PostStartHookFunc, error) {
	webconsolePublicURL := ""
	if c.Options.OAuthConfig != nil {
		webconsolePublicURL = c.Options.OAuthConfig.AssetPublicURL
	}
	webconsoleProxyHandler, err := c.newWebConsoleProxy()
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

			// this is the normal kube handler chain
			handler = apiserver.DefaultBuildHandlerChain(handler, genericConfig)

			// these handlers are all before the normal kube chain
			handler = translateLegacyScopeImpersonation(handler)
			handler = withCacheControl(handler, "no-store") // protected endpoints should not be cached

			// redirects from / to /console if you're using a browser
			handler = withAssetServerRedirect(handler, webconsolePublicURL)

			// these handlers are actually separate API servers which have their own handler chains.
			// our server embeds these
			handler = c.withConsoleRedirection(handler, webconsoleProxyHandler, webconsolePublicURL)
			handler = c.withOAuthRedirection(handler, oauthServerHandler)

			return handler
		},
		extraPostStartHooks,
		nil
}

func openshiftHandlerChain(apiHandler http.Handler, genericConfig *apiserver.Config) http.Handler {
	// this is the normal kube handler chain
	handler := apiserver.DefaultBuildHandlerChain(apiHandler, genericConfig)

	handler = withCacheControl(handler, "no-store") // protected endpoints should not be cached

	return handler
}

// If we know the location of the asset server, redirect to it when / is requested
// and the Accept header supports text/html
func withAssetServerRedirect(handler http.Handler, webconsolePublicURL string) http.Handler {
	if len(webconsolePublicURL) == 0 {
		return handler
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			if httprequest.PrefersHTML(req) {
				http.Redirect(w, req, webconsolePublicURL, http.StatusFound)
			}
		}
		// Dispatch to the next handler
		handler.ServeHTTP(w, req)
	})
}

func (c *MasterConfig) withConsoleRedirection(handler, assetServerHandler http.Handler, webconsolePublicURL string) http.Handler {
	if len(webconsolePublicURL) == 0 {
		return handler
	}

	publicURL, err := url.Parse(webconsolePublicURL)
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

func (c *MasterConfig) startClusterQuotaMapping(context apiserver.PostStartHookContext) error {
	go c.ClusterQuotaMappingController.Run(5, context.StopCh)
	return nil
}
