package origin

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/golang/glog"
	"gopkg.in/natefinch/lumberjack.v2"

	utilwait "k8s.io/apimachinery/pkg/util/wait"
	apifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	apiserver "k8s.io/apiserver/pkg/server"
	apiserverfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/healthz"
	genericmux "k8s.io/apiserver/pkg/server/mux"
	genericroutes "k8s.io/apiserver/pkg/server/routes"
	authzwebhook "k8s.io/apiserver/plugin/pkg/authorizer/webhook"
	clientgoclientset "k8s.io/client-go/kubernetes"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kubeapiserver "k8s.io/kubernetes/pkg/master"
	kcorestorage "k8s.io/kubernetes/pkg/registry/core/rest"

	"crypto/x509"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	serverauthenticator "github.com/openshift/origin/pkg/cmd/server/authenticator"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	serverhandlers "github.com/openshift/origin/pkg/cmd/server/handlers"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/plug"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
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
		GenericConfig:               &kubeAPIServerConfig,
		ControllerPlug:              controllerPlug,
		EnableOAuth:                 c.Options.OAuthConfig != nil,
		KubeClientInternal:          c.PrivilegedLoopbackKubernetesClientsetInternal,
		EnableTemplateServiceBroker: c.Options.TemplateServiceBrokerConfig != nil,
		TemplateInformers:           c.TemplateInformers,
	}
	if c.Options.OAuthConfig != nil {
		ret.MasterPublicURL = c.Options.OAuthConfig.MasterPublicURL
	}
	if c.Options.TemplateServiceBrokerConfig != nil {
		ret.TemplateNamespaces = c.Options.TemplateServiceBrokerConfig.TemplateNamespaces
	}

	return ret
}

// Run launches the OpenShift master by creating a kubernetes master, installing
// OpenShift APIs into it and then running it.
func (c *MasterConfig) Run(kubeAPIServerConfig *kubeapiserver.Config, assetConfig *AssetConfig, controllerPlug plug.Plug, stopCh <-chan struct{}) {
	kubeAPIServerOptions, err := kubernetes.BuildKubeAPIserverOptions(c.Options)
	if err != nil {
		glog.Fatalf("Failed: %v", err)
	}
	apiExtensionsConfig, err := createAPIExtensionsConfig(*kubeAPIServerConfig.GenericConfig, kubeAPIServerOptions.Etcd)
	if err != nil {
		glog.Fatalf("Failed: %v", err)
	}
	apiExtensionsServer, err := createAPIExtensionsServer(apiExtensionsConfig, apiserver.EmptyDelegate)
	if err != nil {
		glog.Fatalf("Failed: %v", err)
	}

	openshiftNonAPIConfig := c.newOpenshiftNonAPIConfig(*kubeAPIServerConfig.GenericConfig, controllerPlug)
	openshiftNonAPIServer, err := openshiftNonAPIConfig.Complete().New(apiExtensionsServer.GenericAPIServer, stopCh)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}

	openshiftAPIServerConfig, err := c.newOpenshiftAPIConfig(*kubeAPIServerConfig.GenericConfig)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}
	// TODO this is eventually where we end up, with the openshift server completely discrete from the kube one
	// but this only works *AFTER* we commit to the aggregator.  Right now the aggregator is optional, so we have
	// to install ourselves in the kubeapiserver
	// openshiftAPIServer, err := openshiftAPIServerConfig.Complete().New(openshiftNonAPIServer.GenericAPIServer, stopCh)
	// if err != nil {
	// 	glog.Fatalf("Failed to launch master: %v", err)
	// }
	// // this sets up the openapi endpoints
	// preparedOpenshiftAPIServer := openshiftAPIServer.GenericAPIServer.PrepareRun()

	// TODO move out of this function to somewhere we build the kubeAPIServerConfig
	kubeAPIServerConfig.GenericConfig.BuildHandlerChainFunc, err = c.buildHandlerChain(assetConfig)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}
	// We need to add an openshift type to the kube's core storage until at least 3.8.  This does that by using a patch we carry.
	kcorestorage.LegacyStorageMutatorFn = sccstorage.AddSCC(openshiftAPIServerConfig.SCCStorage)
	kubeAPIServer, err := kubeAPIServerConfig.Complete().New(openshiftNonAPIServer.GenericAPIServer, apiExtensionsConfig.CRDRESTOptionsGetter)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}
	// TODO this goes away in 3.7 after we commit to the aggregator always being on (even if its just in local mode).
	// this is installing the openshift APIs into the kubeapiserver
	// ok, this is a big side-effect.  Openshift APIs run a different admission chain (always have), but since
	// we're going through a "normal" API installation in the wrong server, we need to switch the admission chain
	// *only while we're installing these APIs*.  There are tests that make sure this works and doesn't drop
	// plugins and we'll remove it once we're aggregating
	kubeAPIServer.GenericAPIServer.SetAdmission(openshiftAPIServerConfig.GenericConfig.AdmissionControl)
	installAPIs(openshiftAPIServerConfig, kubeAPIServer.GenericAPIServer)
	kubeAPIServer.GenericAPIServer.SetAdmission(kubeAPIServerConfig.GenericConfig.AdmissionControl)

	// this sets up the openapi endpoints
	preparedKubeAPIServer := kubeAPIServer.GenericAPIServer.PrepareRun()

	aggregatorConfig, err := c.createAggregatorConfig(*kubeAPIServerConfig.GenericConfig)
	if err != nil {
		glog.Fatalf("Failed to create aggregator config: %v", err)
	}
	aggregatorServer, err := createAggregatorServer(aggregatorConfig, preparedKubeAPIServer.GenericAPIServer, c.InternalKubeInformers, apiExtensionsServer.Informers)
	if err != nil {
		// we don't need special handling for innerStopCh because the aggregator server doesn't create any go routines
		glog.Fatalf("Failed to create aggregator server: %v", err)
	}
	go aggregatorServer.GenericAPIServer.PrepareRun().Run(stopCh)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(c.TLS, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
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

// TODO refactor this out of this package and split apiserver and controllers for good!
func RunControllerServer(servingInfo configapi.HTTPServingInfo, kubeInternal kclientsetinternal.Interface) error {
	clientCAs, err := getClientCertCAPool(servingInfo)
	if err != nil {
		return err
	}

	mux := genericmux.NewPathRecorderMux("master-healthz")

	healthz.InstallHandler(mux, healthz.PingHealthz)
	initReadinessCheckRoute(mux, "/healthz/ready", func() bool { return true })
	genericroutes.Profiling{}.Install(mux)
	genericroutes.MetricsWithReset{}.Install(mux)

	// TODO: replace me with a service account for controller manager
	tokenReview := clientgoclientset.New(kubeInternal.Authentication().RESTClient()).AuthenticationV1beta1().TokenReviews()
	authn, err := serverauthenticator.NewRemoteAuthenticator(tokenReview, clientCAs, 5*time.Minute)
	if err != nil {
		return err
	}
	sarClient := clientgoclientset.New(kubeInternal.Authorization().RESTClient()).AuthorizationV1beta1().SubjectAccessReviews()
	remoteAuthz, err := authzwebhook.NewFromInterface(sarClient, 5*time.Minute, 5*time.Minute)
	if err != nil {
		return err
	}

	// requestInfoFactory for controllers only needs to be able to handle non-API endpoints
	requestInfoResolver := apiserver.NewRequestInfoResolver(&apiserver.Config{})
	// the request context mapper for controllers is always separate
	requestContextMapper := apirequest.NewRequestContextMapper()
	authorizationAttributeBuilder := authorizer.NewAuthorizationAttributeBuilder(requestContextMapper, requestInfoResolver)

	// we use direct bypass to allow readiness and health to work regardless of the master health
	authz := serverhandlers.NewBypassAuthorizer(remoteAuthz, "/healthz", "/healthz/ready")
	handler := serverhandlers.AuthorizationFilter(mux, authz, authorizationAttributeBuilder, requestContextMapper)
	handler = serverhandlers.AuthenticationHandlerFilter(handler, authn, requestContextMapper)
	handler = apiserverfilters.WithPanicRecovery(handler)
	handler = apifilters.WithRequestInfo(handler, requestInfoResolver, requestContextMapper)
	handler = apirequest.WithRequestContext(handler, requestContextMapper)

	serveControllers(servingInfo, handler)
	return nil
}

// serve starts serving the provided http.Handler using security settings derived from the MasterConfig
func serveControllers(servingInfo configapi.HTTPServingInfo, handler http.Handler) {
	timeout := servingInfo.RequestTimeoutSeconds
	if timeout == -1 {
		timeout = 0
	}

	server := &http.Server{
		Addr:           servingInfo.BindAddress,
		Handler:        handler,
		ReadTimeout:    time.Duration(timeout) * time.Second,
		WriteTimeout:   time.Duration(timeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	clientCAs, err := getClientCertCAPool(servingInfo)
	if err != nil {
		glog.Fatal(err)
	}

	go utilwait.Forever(func() {
		glog.Infof("Started health checks at %s", servingInfo.BindAddress)

		if configapi.UseTLS(servingInfo.ServingInfo) {
			extraCerts, err := configapi.GetNamedCertificateMap(servingInfo.NamedCertificates)
			if err != nil {
				glog.Fatal(err)
			}
			server.TLSConfig = crypto.SecureTLSConfig(&tls.Config{
				// Populate PeerCertificates in requests, but don't reject connections without certificates
				// This allows certificates to be validated by authenticators, while still allowing other auth types
				ClientAuth: tls.RequestClientCert,
				ClientCAs:  clientCAs,
				// Set SNI certificate func
				GetCertificate: cmdutil.GetCertificateFunc(extraCerts),
				MinVersion:     crypto.TLSVersionOrDie(servingInfo.MinTLSVersion),
				CipherSuites:   crypto.CipherSuitesOrDie(servingInfo.CipherSuites),
			})
			glog.Fatal(cmdutil.ListenAndServeTLS(server, servingInfo.BindNetwork, servingInfo.ServerCert.CertFile, servingInfo.ServerCert.KeyFile))
		} else {
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)
}

func getClientCertCAPool(servingInfo configapi.HTTPServingInfo) (*x509.CertPool, error) {
	if !configapi.UseTLS(servingInfo.ServingInfo) {
		return nil, nil
	}

	roots := x509.NewCertPool()
	// Add CAs for API
	certs, err := cmdutil.CertificatesFromFile(servingInfo.ClientCA)
	if err != nil {
		return nil, err
	}
	for _, root := range certs {
		roots.AddCert(root)
	}

	return roots, nil
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
