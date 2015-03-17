package origin

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/elazarl/go-bindata-assetfs"
	restful "github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kmaster "github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/assets"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildlogregistry "github.com/openshift/origin/pkg/build/registry/buildlog"
	buildetcd "github.com/openshift/origin/pkg/build/registry/etcd"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/generic"
	"github.com/openshift/origin/pkg/build/webhook/github"
	osclient "github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configchangecontroller "github.com/openshift/origin/pkg/deploy/controller/configchange"
	deployerpodcontroller "github.com/openshift/origin/pkg/deploy/controller/deployerpod"
	deploycontroller "github.com/openshift/origin/pkg/deploy/controller/deployment"
	deployconfigcontroller "github.com/openshift/origin/pkg/deploy/controller/deploymentconfig"
	imagechangecontroller "github.com/openshift/origin/pkg/deploy/controller/imagechange"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/generator"
	deployregistry "github.com/openshift/origin/pkg/deploy/registry/deploy"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployetcd "github.com/openshift/origin/pkg/deploy/registry/etcd"
	deployrollback "github.com/openshift/origin/pkg/deploy/rollback"
	"github.com/openshift/origin/pkg/dns"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	imagerepositoryetcd "github.com/openshift/origin/pkg/image/registry/imagerepository/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorymapping"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorytag"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/accesstoken"
	authorizetokenregistry "github.com/openshift/origin/pkg/oauth/registry/authorizetoken"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/client"
	clientauthorizationregistry "github.com/openshift/origin/pkg/oauth/registry/clientauthorization"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	projectregistry "github.com/openshift/origin/pkg/project/registry/project"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	routeetcd "github.com/openshift/origin/pkg/route/registry/etcd"
	routeregistry "github.com/openshift/origin/pkg/route/registry/route"
	"github.com/openshift/origin/pkg/service"
	templateregistry "github.com/openshift/origin/pkg/template/registry"
	templateetcd "github.com/openshift/origin/pkg/template/registry/etcd"
	"github.com/openshift/origin/pkg/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
	"github.com/openshift/origin/pkg/version"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationetcd "github.com/openshift/origin/pkg/authorization/registry/etcd"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	resourceaccessreviewregistry "github.com/openshift/origin/pkg/authorization/registry/resourceaccessreview"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	subjectaccessreviewregistry "github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	routeplugin "github.com/openshift/origin/plugins/route/allocation/simple"
)

const (
	OpenShiftAPIPrefix        = "/osapi"
	OpenShiftAPIV1Beta1       = "v1beta1"
	OpenShiftAPIPrefixV1Beta1 = OpenShiftAPIPrefix + "/" + OpenShiftAPIV1Beta1
	OpenShiftRouteSubdomain   = "router.default.local"
	swaggerAPIPrefix          = "/swaggerapi/"
)

// APIInstaller installs additional API components into this server
type APIInstaller interface {
	// Returns an array of strings describing what was installed
	InstallAPI(*restful.Container) []string
}

// APIInstallFunc is a function for installing APIs
type APIInstallFunc func(*restful.Container) []string

// InstallAPI implements APIInstaller
func (fn APIInstallFunc) InstallAPI(container *restful.Container) []string {
	return fn(container)
}

// KubeClient returns the kubernetes client object
func (c *MasterConfig) KubeClient() *kclient.Client {
	return c.KubernetesClient
}

// PolicyClient returns the policy client object
// It must have the following capabilities:
//  list, watch all policyBindings in all namespaces
//  list, watch all policies in all namespaces
//  create resourceAccessReviews in all namespaces
func (c *MasterConfig) PolicyClient() *osclient.Client {
	return c.OSClient
}

// DeploymentClient returns the deployment client object
func (c *MasterConfig) DeploymentClient() *kclient.Client {
	return c.KubernetesClient
}

// DNSServerClient returns the DNS server client object
// It must have the following capabilities:
//   list, watch all services in all namespaces
func (c *MasterConfig) DNSServerClient() *kclient.Client {
	return c.KubernetesClient
}

// BuildLogClient returns the build log client object
func (c *MasterConfig) BuildLogClient() *kclient.Client {
	return c.KubernetesClient
}

// WebHookClient returns the webhook client object
func (c *MasterConfig) WebHookClient() *osclient.Client {
	return c.OSClient
}

// BuildControllerClients returns the build controller client objects
func (c *MasterConfig) BuildControllerClients() (*osclient.Client, *kclient.Client) {
	return c.OSClient, c.KubernetesClient
}

// ImageChangeControllerClient returns the openshift client object
func (c *MasterConfig) ImageChangeControllerClient() *osclient.Client {
	return c.OSClient
}

// DeploymentControllerClients returns the deployment controller client object
func (c *MasterConfig) DeploymentControllerClients() (*osclient.Client, *kclient.Client) {
	return c.OSClient, c.KubernetesClient
}

// DeployerClientConfig returns the client configuration a Deployer instance launched in a pod
// should use when making API calls.
func (c *MasterConfig) DeployerClientConfig() *kclient.Config {
	return &c.DeployerOSClientConfig
}

func (c *MasterConfig) DeploymentConfigControllerClients() (*osclient.Client, *kclient.Client) {
	return c.OSClient, c.KubernetesClient
}
func (c *MasterConfig) DeploymentConfigChangeControllerClients() (*osclient.Client, *kclient.Client) {
	return c.OSClient, c.KubernetesClient
}
func (c *MasterConfig) DeploymentImageChangeControllerClient() *osclient.Client {
	return c.OSClient
}

func (c *MasterConfig) InstallProtectedAPI(container *restful.Container) []string {
	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(c.KubeClient().Services(api.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		glog.Fatalf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	buildEtcd := buildetcd.New(c.EtcdHelper)
	deployEtcd := deployetcd.New(c.EtcdHelper)
	routeEtcd := routeetcd.New(c.EtcdHelper)
	userEtcd := useretcd.New(c.EtcdHelper, user.NewDefaultUserInitStrategy())
	oauthEtcd := oauthetcd.New(c.EtcdHelper)
	authorizationEtcd := authorizationetcd.New(c.EtcdHelper)

	imageStorage := imageetcd.NewREST(c.EtcdHelper)
	imageRegistry := image.NewRegistry(imageStorage)
	imageRepositoryStorage := imagerepositoryetcd.NewREST(c.EtcdHelper, imagerepository.DefaultRegistryFunc(defaultRegistryFunc))
	imageRepositoryRegistry := imagerepository.NewRegistry(imageRepositoryStorage)
	imageRepositoryMappingStorage := imagerepositorymapping.NewREST(imageRegistry, imageRepositoryRegistry)
	imageRepositoryTagStorage := imagerepositorytag.NewREST(imageRegistry, imageRepositoryRegistry)
	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageRepositoryRegistry)
	routeAllocator := c.RouteAllocator()

	// TODO: with sharding, this needs to be changed
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		Client: deployconfiggenerator.Client{
			DCFn:   deployEtcd.GetDeploymentConfig,
			IRFn:   imageRepositoryRegistry.GetImageRepository,
			LIRFn2: imageRepositoryRegistry.ListImageRepositories,
		},
		Codec: latest.Codec,
	}
	_, kclient := c.DeploymentConfigControllerClients()
	deployRollback := &deployrollback.RollbackGenerator{}
	deployRollbackClient := deployrollback.Client{
		DCFn: deployEtcd.GetDeploymentConfig,
		RCFn: clientDeploymentInterface{kclient}.GetDeployment,
		GRFn: deployRollback.GenerateRollback,
	}

	// initialize OpenShift API
	storage := map[string]apiserver.RESTStorage{
		"builds":       buildregistry.NewREST(buildEtcd),
		"buildConfigs": buildconfigregistry.NewREST(buildEtcd),
		"buildLogs":    buildlogregistry.NewREST(buildEtcd, c.BuildLogClient()),

		"images":                  imageStorage,
		"imageStreams":            imageRepositoryStorage,
		"imageStreamImages":       imageStreamImageStorage,
		"imageStreamMappings":     imageRepositoryMappingStorage,
		"imageStreamTags":         imageRepositoryTagStorage,
		"imageRepositories":       imageRepositoryStorage,
		"imageRepositoryMappings": imageRepositoryMappingStorage,
		"imageRepositoryTags":     imageRepositoryTagStorage,

		"deployments":               deployregistry.NewREST(deployEtcd),
		"deploymentConfigs":         deployconfigregistry.NewREST(deployEtcd),
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, v1beta1.Codec),
		"deploymentConfigRollbacks": deployrollback.NewREST(deployRollbackClient, latest.Codec),

		"templateConfigs": templateregistry.NewREST(),
		"templates":       templateetcd.NewREST(c.EtcdHelper),

		"routes": routeregistry.NewREST(routeEtcd, routeAllocator),

		"projects": projectregistry.NewREST(kclient.Namespaces(), c.ProjectAuthorizationCache),

		"userIdentityMappings": useridentitymapping.NewREST(userEtcd),
		"users":                userregistry.NewREST(userEtcd),

		"oAuthAuthorizeTokens":      authorizetokenregistry.NewREST(oauthEtcd),
		"oAuthAccessTokens":         accesstokenregistry.NewREST(oauthEtcd),
		"oAuthClients":              clientregistry.NewREST(oauthEtcd),
		"oAuthClientAuthorizations": clientauthorizationregistry.NewREST(oauthEtcd),

		"policies":              policyregistry.NewREST(authorizationEtcd),
		"policyBindings":        policybindingregistry.NewREST(authorizationEtcd),
		"roles":                 roleregistry.NewREST(roleregistry.NewVirtualRegistry(authorizationEtcd)),
		"roleBindings":          rolebindingregistry.NewREST(rolebindingregistry.NewVirtualRegistry(authorizationEtcd, authorizationEtcd, c.Options.MasterAuthorizationNamespace)),
		"resourceAccessReviews": resourceaccessreviewregistry.NewREST(c.Authorizer),
		"subjectAccessReviews":  subjectaccessreviewregistry.NewREST(c.Authorizer),
	}

	admissionControl := admit.NewAlwaysAdmit()

	if err := apiserver.NewAPIGroupVersion(storage, v1beta1.Codec, OpenShiftAPIPrefix, OpenShiftAPIV1Beta1, latest.SelfLinker, admissionControl, c.getRequestContextMapper(), latest.RESTMapper).InstallREST(container, OpenShiftAPIPrefix, "v1beta1"); err != nil {
		glog.Fatalf("Unable to initialize API: %v", err)
	}

	var root *restful.WebService
	for _, svc := range container.RegisteredWebServices() {
		switch svc.RootPath() {
		case "/":
			root = svc
		case OpenShiftAPIPrefixV1Beta1:
			svc.Doc("OpenShift REST API, version v1beta1").ApiVersion("v1beta1")
		}
	}
	if root == nil {
		root = new(restful.WebService)
		container.Add(root)
	}
	initAPIVersionRoute(root, "v1beta1")

	return []string{
		fmt.Sprintf("Started OpenShift API at %%s%s", OpenShiftAPIPrefixV1Beta1),
	}
}

func (c *MasterConfig) InstallUnprotectedAPI(container *restful.Container) []string {
	bcClient, _ := c.BuildControllerClients()
	handler := webhook.NewController(
		buildclient.NewOSClientBuildConfigClient(bcClient),
		buildclient.NewOSClientBuildClient(bcClient),
		bcClient.ImageRepositories(kapi.NamespaceAll).(osclient.ImageRepositoryNamespaceGetter),
		map[string]webhook.Plugin{
			"generic": generic.New(),
			"github":  github.New(),
		})

	// TODO: go-restfulize this
	prefix := OpenShiftAPIPrefixV1Beta1 + "/buildConfigHooks/"
	handler = http.StripPrefix(prefix, handler)
	container.Handle(prefix, handler)
	return []string{}
}

//initAPIVersionRoute initializes the osapi endpoint to behave similar to the upstream api endpoint
func initAPIVersionRoute(root *restful.WebService, version string) {
	versionHandler := apiserver.APIVersionHandler(version)
	root.Route(root.GET(OpenShiftAPIPrefix).To(versionHandler).
		Doc("list supported server API versions").
		Produces(restful.MIME_JSON).
		Consumes(restful.MIME_JSON))
}

// Run launches the OpenShift master. It takes optional installers that may install additional endpoints into the server.
// All endpoints get configured CORS behavior
// Protected installers' endpoints are protected by API authentication and authorization.
// Unprotected installers' endpoints do not have any additional protection added.
func (c *MasterConfig) Run(protected []APIInstaller, unprotected []APIInstaller) {
	var extra []string

	safe := kmaster.NewHandlerContainer(http.NewServeMux())
	open := kmaster.NewHandlerContainer(http.NewServeMux())

	// enforce authentication on protected endpoints
	protected = append(protected, APIInstallFunc(c.InstallProtectedAPI))
	for _, i := range protected {
		extra = append(extra, i.InstallAPI(safe)...)
	}
	handler := c.authorizationFilter(safe)
	handler = authenticationHandlerFilter(handler, c.Authenticator, c.getRequestContextMapper())
	handler = namespacingFilter(handler, c.getRequestContextMapper())

	// unprotected resources
	unprotected = append(unprotected, APIInstallFunc(c.InstallUnprotectedAPI))
	for _, i := range unprotected {
		extra = append(extra, i.InstallAPI(open)...)
	}
	open.Handle("/", handler)

	// install swagger
	swaggerConfig := swagger.Config{
		WebServices: append(safe.RegisteredWebServices(), open.RegisteredWebServices()...),
		ApiPath:     swaggerAPIPrefix,
	}
	swagger.RegisterSwaggerService(swaggerConfig, open)
	extra = append(extra, fmt.Sprintf("Started Swagger Schema API at %%s%s", swaggerAPIPrefix))

	handler = open

	// add CORS support
	if origins := c.ensureCORSAllowedOrigins(); len(origins) != 0 {
		handler = apiserver.CORS(handler, origins, nil, nil, "true")
	}

	// Make the outermost filter the requestContextMapper to ensure all components share the same context
	if contextHandler, err := kapi.NewRequestContextFilter(c.getRequestContextMapper(), handler); err != nil {
		glog.Fatalf("Error setting up request context filter: %v", err)
	} else {
		handler = contextHandler
	}

	server := &http.Server{
		Addr:           c.Options.ServingInfo.BindAddress,
		Handler:        handler,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		for _, s := range extra {
			glog.Infof(s, c.Options.ServingInfo.BindAddress)
		}
		if c.TLS {
			server.TLSConfig = &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
				// Populate PeerCertificates in requests, but don't reject connections without certificates
				// This allows certificates to be validated by authenticators, while still allowing other auth types
				ClientAuth: tls.RequestClientCert,
				ClientCAs:  c.ClientCAs,
			}
			glog.Fatal(server.ListenAndServeTLS(c.Options.ServingInfo.ServerCert.CertFile, c.Options.ServingInfo.ServerCert.KeyFile))
		} else {
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(c.TLS, "tcp", c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	// Attempt to create the required policy rules now, and then stick in a forever loop to make sure they are always available
	c.ensureComponentAuthorizationRules()
	c.ensureMasterAuthorizationNamespace()
	go util.Forever(func() {
		c.ensureComponentAuthorizationRules()
		c.ensureMasterAuthorizationNamespace()
	}, 10*time.Second)
}

// getRequestContextMapper returns a mapper from requests to contexts, initializing it if needed
func (c *MasterConfig) getRequestContextMapper() kapi.RequestContextMapper {
	if c.RequestContextMapper == nil {
		c.RequestContextMapper = kapi.NewRequestContextMapper()
	}
	return c.RequestContextMapper
}

// ensureMasterAuthorizationNamespace is called as part of global policy initialization to ensure master namespace exists
func (c *MasterConfig) ensureMasterAuthorizationNamespace() {

	// ensure that master namespace actually exists
	namespace, err := c.KubeClient().Namespaces().Get(c.Options.MasterAuthorizationNamespace)
	if err != nil {
		namespace = &kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: c.Options.MasterAuthorizationNamespace}}
		kapi.FillObjectMetaSystemFields(api.NewContext(), &namespace.ObjectMeta)
		_, err = c.KubeClient().Namespaces().Create(namespace)
		if err != nil {
			glog.Errorf("Error creating namespace: %v due to %v\n", namespace, err)
		}
	}
}

// ensureComponentAuthorizationRules initializes the global policies
func (c *MasterConfig) ensureComponentAuthorizationRules() {
	registry := authorizationetcd.New(c.EtcdHelper)

	roleRegistry := roleregistry.NewVirtualRegistry(registry)
	for _, role := range bootstrappolicy.GetBootstrapRoles(c.Options.MasterAuthorizationNamespace, c.Options.OpenShiftSharedResourcesNamespace) {
		ctx := kapi.WithNamespace(kapi.NewContext(), role.Namespace)

		if _, err := roleRegistry.GetRole(ctx, role.Name); kapierror.IsNotFound(err) {
			if err := roleRegistry.CreateRole(ctx, &role); err != nil {
				glog.Errorf("Error creating role: %#v due to %v\n", role, err)
			}

		} else if err != nil {
			glog.Errorf("Error get role: %#v due to %v\n", role, err)
		}
	}

	roleBindingRegistry := rolebindingregistry.NewVirtualRegistry(registry, registry, c.Options.MasterAuthorizationNamespace)
	for _, roleBinding := range bootstrappolicy.GetBootstrapRoleBindings(c.Options.MasterAuthorizationNamespace, c.Options.OpenShiftSharedResourcesNamespace) {
		ctx := kapi.WithNamespace(kapi.NewContext(), roleBinding.Namespace)

		if _, err := roleBindingRegistry.GetRoleBinding(ctx, roleBinding.Name); kapierror.IsNotFound(err) {
			// if this is a binding for a non-master namespaced role.  That means that the policy binding must be provisioned
			if roleBinding.RoleRef.Namespace != c.Options.MasterAuthorizationNamespace {
				policyBindingName := roleBinding.RoleRef.Namespace
				if _, err := registry.GetPolicyBinding(ctx, policyBindingName); kapierror.IsNotFound(err) {
					policyBinding := &authorizationapi.PolicyBinding{
						ObjectMeta: kapi.ObjectMeta{
							Namespace: roleBinding.Namespace,
							Name:      policyBindingName,
						},

						PolicyRef: kapi.ObjectReference{
							Namespace: roleBinding.RoleRef.Namespace,
							Name:      authorizationapi.PolicyName,
						},
					}

					if err := registry.CreatePolicyBinding(ctx, policyBinding); err != nil {
						glog.Errorf("Error creating policyBinding: %#v due to %v\n", policyBinding, err)
					}

				} else if err != nil {
					glog.Errorf("Error getting policyBinding: %#v due to %v\n", policyBindingName, err)
				}
			}

			if err := roleBindingRegistry.CreateRoleBinding(ctx, &roleBinding, true); err != nil {
				glog.Errorf("Error creating roleBinding: %#v due to %v\n", roleBinding, err)
			}

		} else if err != nil {
			glog.Errorf("Error getting roleBinding: %#v due to %v\n", roleBinding, err)
		}
	}

}

func (c *MasterConfig) authorizationFilter(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attributes, err := c.AuthorizationAttributeBuilder.GetAttributes(req)
		if err != nil {
			forbidden(err.Error(), w, req)
			return
		}
		if attributes == nil {
			forbidden("No attributes", w, req)
			return
		}

		ctx, exists := c.RequestContextMapper.Get(req)
		if !exists {
			forbidden("context not found", w, req)
			return
		}

		allowed, reason, err := c.Authorizer.Authorize(ctx, attributes)
		if err != nil {
			forbidden(err.Error(), w, req)
			return
		}
		if !allowed {
			forbidden(reason, w, req)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

// forbidden renders a simple forbidden error
func forbidden(reason string, w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, "Forbidden: %q %s", req.RequestURI, reason)
}

// RunProjectAuthorizationCache starts the project authorization cache
func (c *MasterConfig) RunProjectAuthorizationCache() {
	// TODO: look at exposing a configuration option in future to control how often we run this loop
	period := 1 * time.Second
	c.ProjectAuthorizationCache.Run(period)
}

// RunPolicyCache starts the policy cache
func (c *MasterConfig) RunPolicyCache() {
	c.PolicyCache.Run()
}

// RunAssetServer starts the asset server for the OpenShift UI.
func (c *MasterConfig) RunAssetServer() {
	// TODO use	version.Get().GitCommit as an etag cache header
	mux := http.NewServeMux()

	masterURL, err := url.Parse(c.Options.AssetConfig.MasterPublicURL)
	if err != nil {
		glog.Fatalf("Error parsing master url: %v", err)
	}

	k8sURL, err := url.Parse(c.Options.AssetConfig.KubernetesPublicURL)
	if err != nil {
		glog.Fatalf("Error parsing kubernetes url: %v", err)
	}

	config := assets.WebConsoleConfig{
		MasterAddr:        masterURL.Host,
		MasterPrefix:      OpenShiftAPIPrefix,
		KubernetesAddr:    k8sURL.Host,
		KubernetesPrefix:  "/api",
		OAuthAuthorizeURI: OpenShiftOAuthAuthorizeURL(masterURL.String()),
		OAuthRedirectBase: c.Options.AssetConfig.PublicURL,
		OAuthClientID:     OpenShiftWebConsoleClientID,
		LogoutURI:         c.Options.AssetConfig.LogoutURI,
	}

	assets.RegisterMimeTypes()

	mux.Handle("/",
		// Gzip first so that inner handlers can react to the addition of the Vary header
		assets.GzipHandler(
			// Generated config.js can not be cached since it changes depending on startup options
			assets.GeneratedConfigHandler(
				config,
				// Cache control should happen after all Vary headers are added, but before
				// any asset related routing (HTML5ModeHandler and FileServer)
				assets.CacheControlHandler(
					version.Get().GitCommit,
					assets.HTML5ModeHandler(
						http.FileServer(
							&assetfs.AssetFS{
								assets.Asset,
								assets.AssetDir,
								"",
							},
						),
					),
				),
			),
		),
	)

	server := &http.Server{
		Addr:           c.Options.AssetConfig.ServingInfo.BindAddress,
		Handler:        mux,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		if c.TLS {
			server.TLSConfig = &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
			}
			glog.Infof("OpenShift UI listening at https://%s", c.Options.AssetConfig.ServingInfo.BindAddress)
			glog.Fatal(server.ListenAndServeTLS(c.Options.AssetConfig.ServingInfo.ServerCert.CertFile, c.Options.AssetConfig.ServingInfo.ServerCert.KeyFile))
		} else {
			glog.Infof("OpenShift UI listening at https://%s", c.Options.AssetConfig.ServingInfo.BindAddress)
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(c.TLS, "tcp", c.Options.AssetConfig.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	glog.Infof("OpenShift UI available at %s", c.Options.AssetConfig.PublicURL)
}

func (c *MasterConfig) RunDNSServer() {
	config, err := dns.NewServerDefaults()
	if err != nil {
		glog.Fatalf("Could not start DNS: %v", err)
	}
	config.DnsAddr = c.Options.DNSConfig.BindAddress

	_, port, err := net.SplitHostPort(c.Options.DNSConfig.BindAddress)
	if err != nil {
		glog.Fatalf("Could not start DNS: %v", err)
	}
	if port != "53" {
		glog.Warningf("Binding DNS on port %v instead of 53 (you may need to run as root and update your config), using %s which will not resolve from all locations", port, c.Options.DNSConfig.BindAddress)
	}

	if ok, err := cmdutil.TryListen(c.Options.DNSConfig.BindAddress); !ok {
		glog.Warningf("Could not start DNS: %v", err)
		return
	}

	go func() {
		err := dns.ListenAndServe(config, c.DNSServerClient(), c.EtcdHelper.Client.(*etcdclient.Client))
		glog.Fatalf("Could not start DNS: %v", err)
	}()

	cmdutil.WaitForSuccessfulDial(false, "tcp", c.Options.DNSConfig.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	glog.Infof("OpenShift DNS listening at %s", c.Options.DNSConfig.BindAddress)
}

// RunBuildController starts the build sync loop for builds and buildConfig processing.
func (c *MasterConfig) RunBuildController() {
	// initialize build controller
	dockerImage := c.ImageFor("docker-builder")
	stiImage := c.ImageFor("sti-builder")

	osclient, kclient := c.BuildControllerClients()
	factory := buildcontrollerfactory.BuildControllerFactory{
		OSClient:     osclient,
		KubeClient:   kclient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osclient),
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image: dockerImage,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: v1beta1.Codec,
		},
		STIBuildStrategy: &buildstrategy.STIBuildStrategy{
			Image:                stiImage,
			TempDirectoryCreator: buildstrategy.STITempDirectoryCreator,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: v1beta1.Codec,
		},
		CustomBuildStrategy: &buildstrategy.CustomBuildStrategy{
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: v1beta1.Codec,
		},
	}

	controller := factory.Create()
	controller.Run()
}

// RunBuildPodController starts the build/pod status sync loop for build status
func (c *MasterConfig) RunBuildPodController() {
	osclient, kclient := c.BuildControllerClients()
	factory := buildcontrollerfactory.BuildPodControllerFactory{
		OSClient:     osclient,
		KubeClient:   kclient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osclient),
	}
	controller := factory.Create()
	controller.Run()
}

// RunBuildImageChangeTriggerController starts the build image change trigger controller process.
func (c *MasterConfig) RunBuildImageChangeTriggerController() {
	bcClient, _ := c.BuildControllerClients()
	bcUpdater := buildclient.NewOSClientBuildConfigClient(bcClient)
	bCreator := buildclient.NewOSClientBuildClient(bcClient)
	factory := buildcontrollerfactory.ImageChangeControllerFactory{Client: bcClient, BuildCreator: bCreator, BuildConfigUpdater: bcUpdater}
	factory.Create().Run()
}

// RunDeploymentController starts the deployment controller process.
func (c *MasterConfig) RunDeploymentController() error {
	_, kclient := c.DeploymentControllerClients()

	_, kclientConfig, err := configapi.GetKubeClient(c.Options.MasterClients.OpenShiftLoopbackKubeConfig)
	if err != nil {
		return err
	}
	// TODO eliminate these environment variables once we figure out what they do
	env := []api.EnvVar{
		{Name: "KUBERNETES_MASTER", Value: kclientConfig.Host},
		{Name: "OPENSHIFT_MASTER", Value: kclientConfig.Host},
	}
	env = append(env, clientcmd.EnvVarsFromConfig(c.DeployerClientConfig())...)

	factory := deploycontroller.DeploymentControllerFactory{
		KubeClient:            kclient,
		Codec:                 latest.Codec,
		Environment:           env,
		RecreateStrategyImage: c.ImageFor("deployer"),
	}

	controller := factory.Create()
	controller.Run()

	return nil
}

// RunDeployerPodController starts the deployer pod controller process.
func (c *MasterConfig) RunDeployerPodController() {
	_, kclient := c.DeploymentControllerClients()
	factory := deployerpodcontroller.DeployerPodControllerFactory{
		KubeClient: kclient,
	}

	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentConfigController() {
	osclient, kclient := c.DeploymentConfigControllerClients()
	factory := deployconfigcontroller.DeploymentConfigControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      latest.Codec,
	}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentConfigChangeController() {
	osclient, kclient := c.DeploymentConfigChangeControllerClients()
	factory := configchangecontroller.DeploymentConfigChangeControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      latest.Codec,
	}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentImageChangeTriggerController() {
	osclient := c.DeploymentImageChangeControllerClient()
	factory := imagechangecontroller.ImageChangeControllerFactory{Client: osclient}
	controller := factory.Create()
	controller.Run()
}

// RouteAllocator returns a route allocation controller.
func (c *MasterConfig) RouteAllocator() *routeallocationcontroller.RouteAllocationController {
	factory := routeallocationcontroller.RouteAllocationControllerFactory{
		OSClient:   c.OSClient,
		KubeClient: c.KubeClient(),
	}

	subdomain := os.Getenv("OPENSHIFT_ROUTE_SUBDOMAIN")
	if len(subdomain) == 0 {
		subdomain = OpenShiftRouteSubdomain
	}

	plugin, err := routeplugin.NewSimpleAllocationPlugin(subdomain)
	if err != nil {
		glog.Fatalf("Route plugin initialization failed: %v", err)
	}

	return factory.Create(plugin)
}

// ensureCORSAllowedOrigins takes a string list of origins and attempts to covert them to CORS origin
// regexes, or exits if it cannot.
func (c *MasterConfig) ensureCORSAllowedOrigins() []*regexp.Regexp {
	if len(c.Options.CORSAllowedOrigins) == 0 {
		return []*regexp.Regexp{}
	}
	allowedOriginRegexps, err := util.CompileRegexps(util.StringList(c.Options.CORSAllowedOrigins))
	if err != nil {
		glog.Fatalf("Invalid --cors-allowed-origins: %v", err)
	}
	return allowedOriginRegexps
}

// NewEtcdHelper returns an EtcdHelper for the provided arguments or an error if the version
// is incorrect.
func NewEtcdHelper(version string, client *etcdclient.Client) (helper tools.EtcdHelper, err error) {
	if len(version) == 0 {
		version = latest.Version
	}
	interfaces, err := latest.InterfacesFor(version)
	if err != nil {
		return helper, err
	}
	return tools.EtcdHelper{client, interfaces.Codec, tools.RuntimeVersionAdapter{interfaces.MetadataAccessor}}, nil
}

// env returns an environment variable, or the defaultValue if it is not set.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

type clientDeploymentInterface struct {
	KubeClient kclient.Interface
}

func (c clientDeploymentInterface) GetDeployment(ctx api.Context, name string) (*api.ReplicationController, error) {
	return c.KubeClient.ReplicationControllers(api.NamespaceValue(ctx)).Get(name)
}

// namespacingFilter adds a filter that adds the namespace of the request to the context.  Not all requests will have namespaces,
// but any that do will have the appropriate value added.
func namespacingFilter(handler http.Handler, contextMapper kapi.RequestContextMapper) http.Handler {
	infoResolver := &apiserver.APIRequestInfoResolver{util.NewStringSet("api", "osapi"), latest.RESTMapper}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, ok := contextMapper.Get(req)
		if !ok {
			http.Error(w, "Unable to find request context", http.StatusInternalServerError)
			return
		}

		if _, exists := kapi.NamespaceFrom(ctx); !exists {
			if requestInfo, err := infoResolver.GetAPIRequestInfo(req); err == nil {
				// only set the namespace if the apiRequestInfo was resolved
				// keep in mind that GetAPIRequestInfo will fail on non-api requests, so don't fail the entire http request on that
				// kind of failure.

				// TODO reconsider special casing this.  Having the special case hereallow us to fully share the kube
				// APIRequestInfoResolver without any modification or customization.
				namespace := requestInfo.Namespace
				if (requestInfo.Resource == "projects") && (len(requestInfo.Name) > 0) {
					namespace = requestInfo.Name
				}

				ctx = kapi.WithNamespace(ctx, namespace)
				contextMapper.Update(req, ctx)
			}
		}

		handler.ServeHTTP(w, req)
	})
}
