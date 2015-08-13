package origin

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/apiserver"
	kclient "k8s.io/kubernetes/pkg/client"
	kmaster "k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/api/v1beta3"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildetcd "github.com/openshift/origin/pkg/build/registry/build/etcd"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildconfigetcd "github.com/openshift/origin/pkg/build/registry/buildconfig/etcd"
	buildlogregistry "github.com/openshift/origin/pkg/build/registry/buildlog"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/generic"
	"github.com/openshift/origin/pkg/build/webhook/github"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/generator"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployconfigetcd "github.com/openshift/origin/pkg/deploy/registry/deployconfig/etcd"
	deployrollback "github.com/openshift/origin/pkg/deploy/registry/rollback"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	"github.com/openshift/origin/pkg/image/registry/imagestreammapping"
	"github.com/openshift/origin/pkg/image/registry/imagestreamtag"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	authorizetokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken/etcd"
	clientetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclient/etcd"
	clientauthetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization/etcd"
	projectproxy "github.com/openshift/origin/pkg/project/registry/project/proxy"
	projectrequeststorage "github.com/openshift/origin/pkg/project/registry/projectrequest/delegated"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	routeetcd "github.com/openshift/origin/pkg/route/registry/etcd"
	routeregistry "github.com/openshift/origin/pkg/route/registry/route"
	clusternetworketcd "github.com/openshift/origin/pkg/sdn/registry/clusternetwork/etcd"
	hostsubnetetcd "github.com/openshift/origin/pkg/sdn/registry/hostsubnet/etcd"
	netnamespaceetcd "github.com/openshift/origin/pkg/sdn/registry/netnamespace/etcd"
	"github.com/openshift/origin/pkg/service"
	templateregistry "github.com/openshift/origin/pkg/template/registry"
	templateetcd "github.com/openshift/origin/pkg/template/registry/etcd"
	groupetcd "github.com/openshift/origin/pkg/user/registry/group/etcd"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"

	buildclonestorage "github.com/openshift/origin/pkg/build/registry/clone/generator"
	buildinstantiatestorage "github.com/openshift/origin/pkg/build/registry/instantiate/generator"

	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicystorage "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy/etcd"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	clusterpolicybindingstorage "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding/etcd"
	clusterrolestorage "github.com/openshift/origin/pkg/authorization/registry/clusterrole/proxy"
	clusterrolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/clusterrolebinding/proxy"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policyetcd "github.com/openshift/origin/pkg/authorization/registry/policy/etcd"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	policybindingetcd "github.com/openshift/origin/pkg/authorization/registry/policybinding/etcd"
	resourceaccessreviewregistry "github.com/openshift/origin/pkg/authorization/registry/resourceaccessreview"
	rolestorage "github.com/openshift/origin/pkg/authorization/registry/role/policybased"
	rolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/rolebinding/policybased"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	routeplugin "github.com/openshift/origin/plugins/route/allocation/simple"
)

const (
	LegacyOpenShiftAPIPrefix  = "/osapi" // TODO: make configurable
	OpenShiftAPIPrefix        = "/oapi"  // TODO: make configurable
	KubernetesAPIPrefix       = "/api"   // TODO: make configurable
	OpenShiftAPIV1Beta3       = "v1beta3"
	OpenShiftAPIV1            = "v1"
	OpenShiftAPIPrefixV1Beta3 = LegacyOpenShiftAPIPrefix + "/" + OpenShiftAPIV1Beta3
	OpenShiftAPIPrefixV1      = OpenShiftAPIPrefix + "/" + OpenShiftAPIV1
	swaggerAPIPrefix          = "/swaggerapi/"
)

var (
	excludedV1Beta3Types = util.NewStringSet()
	excludedV1Types      = excludedV1Beta3Types

	// TODO: correctly solve identifying requests by type
	longRunningRE = regexp.MustCompile("watch|proxy|logs|exec|portforward")
)

// APIInstaller installs additional API components into this server
type APIInstaller interface {
	// InstallAPI returns an array of strings describing what was installed
	InstallAPI(*restful.Container) []string
}

// APIInstallFunc is a function for installing APIs
type APIInstallFunc func(*restful.Container) []string

// InstallAPI implements APIInstaller
func (fn APIInstallFunc) InstallAPI(container *restful.Container) []string {
	return fn(container)
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
	handler = cacheControlFilter(handler, "no-store") // protected endpoints should not be cached

	// unprotected resources
	unprotected = append(unprotected, APIInstallFunc(c.InstallUnprotectedAPI))
	for _, i := range unprotected {
		extra = append(extra, i.InstallAPI(open)...)
	}

	handler = indexAPIPaths(handler)

	open.Handle("/", handler)

	// install swagger
	swaggerConfig := swagger.Config{
		WebServicesUrl:   c.Options.MasterPublicURL,
		WebServices:      append(safe.RegisteredWebServices(), open.RegisteredWebServices()...),
		ApiPath:          swaggerAPIPrefix,
		PostBuildHandler: customizeSwaggerDefinition,
	}
	// log nothing from swagger
	swagger.LogInfo = func(format string, v ...interface{}) {}
	swagger.RegisterSwaggerService(swaggerConfig, open)
	extra = append(extra, fmt.Sprintf("Started Swagger Schema API at %%s%s", swaggerAPIPrefix))

	handler = open

	// add CORS support
	if origins := c.ensureCORSAllowedOrigins(); len(origins) != 0 {
		handler = apiserver.CORS(handler, origins, nil, nil, "true")
	}

	if c.WebConsoleEnabled() {
		handler = assetServerRedirect(handler, c.Options.AssetConfig.PublicURL)
	}

	// Make the outermost filter the requestContextMapper to ensure all components share the same context
	if contextHandler, err := kapi.NewRequestContextFilter(c.getRequestContextMapper(), handler); err != nil {
		glog.Fatalf("Error setting up request context filter: %v", err)
	} else {
		handler = contextHandler
	}

	// TODO: MaxRequestsInFlight should be subdivided by intent, type of behavior, and speed of
	// execution - updates vs reads, long reads vs short reads, fat reads vs skinny reads.
	if c.Options.ServingInfo.MaxRequestsInFlight > 0 {
		sem := make(chan bool, c.Options.ServingInfo.MaxRequestsInFlight)
		handler = apiserver.MaxInFlightLimit(sem, longRunningRE, handler)
	}

	timeout := c.Options.ServingInfo.RequestTimeoutSeconds
	if timeout == -1 {
		timeout = 0
	}

	server := &http.Server{
		Addr:           c.Options.ServingInfo.BindAddress,
		Handler:        handler,
		ReadTimeout:    time.Duration(timeout) * time.Second,
		WriteTimeout:   time.Duration(timeout) * time.Second,
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
			glog.Fatal(cmdutil.ListenAndServeTLS(server, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.ServerCert.CertFile, c.Options.ServingInfo.ServerCert.KeyFile))
		} else {
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(c.TLS, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

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

func (c *MasterConfig) InstallProtectedAPI(container *restful.Container) []string {
	// initialize OpenShift API
	storage := c.GetRestStorage()

	messages := []string{}
	legacyAPIVersions := []string{}
	currentAPIVersions := []string{}

	if configapi.HasOpenShiftAPILevel(c.Options, OpenShiftAPIV1Beta3) {
		if err := c.api_v1beta3(storage).InstallREST(container); err != nil {
			glog.Fatalf("Unable to initialize v1beta3 API: %v", err)
		}
		messages = append(messages, fmt.Sprintf("Started Origin API at %%s%s", OpenShiftAPIPrefixV1Beta3))
		legacyAPIVersions = append(legacyAPIVersions, OpenShiftAPIV1Beta3)
	}

	if configapi.HasOpenShiftAPILevel(c.Options, OpenShiftAPIV1) {
		if err := c.api_v1(storage).InstallREST(container); err != nil {
			glog.Fatalf("Unable to initialize v1 API: %v", err)
		}
		messages = append(messages, fmt.Sprintf("Started Origin API at %%s%s", OpenShiftAPIPrefixV1))
		currentAPIVersions = append(currentAPIVersions, OpenShiftAPIV1)
	}

	var root *restful.WebService
	for _, service := range container.RegisteredWebServices() {
		switch service.RootPath() {
		case "/":
			root = service
		case OpenShiftAPIPrefixV1Beta3:
			service.Doc("OpenShift REST API, version v1beta3").ApiVersion("v1beta3")
		case OpenShiftAPIPrefixV1:
			service.Doc("OpenShift REST API, version v1").ApiVersion("v1")
		}
	}

	if root == nil {
		root = new(restful.WebService)
		container.Add(root)
	}

	initControllerRoutes(root, "/controllers", c.Options.Controllers != configapi.ControllersDisabled, c.ControllerPlug)
	initAPIVersionRoute(root, LegacyOpenShiftAPIPrefix, legacyAPIVersions...)
	initAPIVersionRoute(root, OpenShiftAPIPrefix, currentAPIVersions...)
	initHealthCheckRoute(root, "healthz")
	initReadinessCheckRoute(root, "healthz/ready", c.ProjectAuthorizationCache.ReadyForAccess)

	return messages
}

func (c *MasterConfig) GetRestStorage() map[string]rest.Storage {
	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(c.KubeClient().Services(kapi.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		glog.Fatalf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	kubeletClient, err := kclient.NewKubeletClient(c.KubeletClientConfig)
	if err != nil {
		glog.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	buildStorage := buildetcd.NewStorage(c.EtcdHelper)
	buildRegistry := buildregistry.NewRegistry(buildStorage)

	buildConfigStorage := buildconfigetcd.NewStorage(c.EtcdHelper)
	buildConfigRegistry := buildconfigregistry.NewRegistry(buildConfigStorage)

	deployConfigStorage := deployconfigetcd.NewStorage(c.EtcdHelper)
	deployConfigRegistry := deployconfigregistry.NewRegistry(deployConfigStorage)

	routeEtcd := routeetcd.New(c.EtcdHelper)
	hostSubnetStorage := hostsubnetetcd.NewREST(c.EtcdHelper)
	netNamespaceStorage := netnamespaceetcd.NewREST(c.EtcdHelper)
	clusterNetworkStorage := clusternetworketcd.NewREST(c.EtcdHelper)

	userStorage := useretcd.NewREST(c.EtcdHelper)
	userRegistry := userregistry.NewRegistry(userStorage)
	identityStorage := identityetcd.NewREST(c.EtcdHelper)
	identityRegistry := identityregistry.NewRegistry(identityStorage)
	userIdentityMappingStorage := useridentitymapping.NewREST(userRegistry, identityRegistry)

	policyStorage := policyetcd.NewStorage(c.EtcdHelper)
	policyRegistry := policyregistry.NewRegistry(policyStorage)
	policyBindingStorage := policybindingetcd.NewStorage(c.EtcdHelper)
	policyBindingRegistry := policybindingregistry.NewRegistry(policyBindingStorage)

	clusterPolicyStorage := clusterpolicystorage.NewStorage(c.EtcdHelper)
	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterPolicyStorage)
	clusterPolicyBindingStorage := clusterpolicybindingstorage.NewStorage(c.EtcdHelper)
	clusterPolicyBindingRegistry := clusterpolicybindingregistry.NewRegistry(clusterPolicyBindingStorage)

	roleStorage := rolestorage.NewVirtualStorage(policyRegistry)
	roleBindingStorage := rolebindingstorage.NewVirtualStorage(policyRegistry, policyBindingRegistry, clusterPolicyRegistry, clusterPolicyBindingRegistry)
	clusterRoleStorage := clusterrolestorage.NewClusterRoleStorage(clusterPolicyRegistry)
	clusterRoleBindingStorage := clusterrolebindingstorage.NewClusterRoleBindingStorage(clusterPolicyRegistry, clusterPolicyBindingRegistry)

	subjectAccessReviewStorage := subjectaccessreview.NewREST(c.Authorizer)
	subjectAccessReviewRegistry := subjectaccessreview.NewRegistry(subjectAccessReviewStorage)

	imageStorage := imageetcd.NewREST(c.EtcdHelper)
	imageRegistry := image.NewRegistry(imageStorage)
	imageStreamStorage, imageStreamStatusStorage := imagestreametcd.NewREST(c.EtcdHelper, imagestream.DefaultRegistryFunc(defaultRegistryFunc), subjectAccessReviewRegistry)
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatusStorage)
	imageStreamMappingStorage := imagestreammapping.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamTagRegistry := imagestreamtag.NewRegistry(imageStreamTagStorage)
	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamImageRegistry := imagestreamimage.NewRegistry(imageStreamImageStorage)

	routeAllocator := c.RouteAllocator()

	buildGenerator := &buildgenerator.BuildGenerator{
		Client: buildgenerator.Client{
			GetBuildConfigFunc:      buildConfigRegistry.GetBuildConfig,
			UpdateBuildConfigFunc:   buildConfigRegistry.UpdateBuildConfig,
			GetBuildFunc:            buildRegistry.GetBuild,
			CreateBuildFunc:         buildRegistry.CreateBuild,
			GetImageStreamFunc:      imageStreamRegistry.GetImageStream,
			GetImageStreamImageFunc: imageStreamImageRegistry.GetImageStreamImage,
			GetImageStreamTagFunc:   imageStreamTagRegistry.GetImageStreamTag,
		},
		ServiceAccounts: c.KubeClient(),
		Secrets:         c.KubeClient(),
	}

	// TODO: with sharding, this needs to be changed
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		Client: deployconfiggenerator.Client{
			DCFn:   deployConfigRegistry.GetDeploymentConfig,
			ISFn:   imageStreamRegistry.GetImageStream,
			LISFn2: imageStreamRegistry.ListImageStreams,
		},
	}
	_, kclient := c.DeploymentConfigControllerClients()
	deployRollback := &deployrollback.RollbackGenerator{}
	deployRollbackClient := deployrollback.Client{
		DCFn: deployConfigRegistry.GetDeploymentConfig,
		RCFn: clientDeploymentInterface{kclient}.GetDeployment,
		GRFn: deployRollback.GenerateRollback,
	}

	projectStorage := projectproxy.NewREST(kclient.Namespaces(), c.ProjectAuthorizationCache)

	namespace, templateName, err := configapi.ParseNamespaceAndName(c.Options.ProjectConfig.ProjectRequestTemplate)
	if err != nil {
		glog.Errorf("Error parsing project request template value: %v", err)
		// we can continue on, the storage that gets created will be valid, it simply won't work properly.  There's no reason to kill the master
	}
	projectRequestStorage := projectrequeststorage.NewREST(c.Options.ProjectConfig.ProjectRequestMessage, namespace, templateName, c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient)

	bcClient := c.BuildConfigWebHookClient()
	buildConfigWebHooks := buildconfigregistry.NewWebHookREST(
		buildConfigRegistry,
		buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient),
		map[string]webhook.Plugin{
			"generic": generic.New(),
			"github":  github.New(),
		},
	)

	storage := map[string]rest.Storage{
		"images":              imageStorage,
		"imageStreams":        imageStreamStorage,
		"imageStreams/status": imageStreamStatusStorage,
		"imageStreamImages":   imageStreamImageStorage,
		"imageStreamMappings": imageStreamMappingStorage,
		"imageStreamTags":     imageStreamTagStorage,

		"deploymentConfigs":         deployConfigStorage,
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, c.EtcdHelper.Codec()),
		"deploymentConfigRollbacks": deployrollback.NewREST(deployRollbackClient, c.EtcdHelper.Codec()),

		"processedTemplates": templateregistry.NewREST(),
		"templates":          templateetcd.NewREST(c.EtcdHelper),

		"routes": routeregistry.NewREST(routeEtcd, routeAllocator),

		"projects":        projectStorage,
		"projectRequests": projectRequestStorage,

		"hostSubnets":     hostSubnetStorage,
		"netNamespaces":   netNamespaceStorage,
		"clusterNetworks": clusterNetworkStorage,

		"users":                userStorage,
		"groups":               groupetcd.NewREST(c.EtcdHelper),
		"identities":           identityStorage,
		"userIdentityMappings": userIdentityMappingStorage,

		"oAuthAuthorizeTokens":      authorizetokenetcd.NewREST(c.EtcdHelper),
		"oAuthAccessTokens":         accesstokenetcd.NewREST(c.EtcdHelper),
		"oAuthClients":              clientetcd.NewREST(c.EtcdHelper),
		"oAuthClientAuthorizations": clientauthetcd.NewREST(c.EtcdHelper),

		"resourceAccessReviews": resourceaccessreviewregistry.NewREST(c.Authorizer),
		"subjectAccessReviews":  subjectAccessReviewStorage,

		"policies":       policyStorage,
		"policyBindings": policyBindingStorage,
		"roles":          roleStorage,
		"roleBindings":   roleBindingStorage,

		"clusterPolicies":       clusterPolicyStorage,
		"clusterPolicyBindings": clusterPolicyBindingStorage,
		"clusterRoleBindings":   clusterRoleBindingStorage,
		"clusterRoles":          clusterRoleStorage,
	}

	if configapi.IsBuildEnabled(&c.Options) {
		storage["builds"] = buildStorage
		storage["buildConfigs"] = buildConfigStorage
		storage["buildConfigs/webhooks"] = buildConfigWebHooks
		storage["builds/clone"] = buildclonestorage.NewStorage(buildGenerator)
		storage["buildConfigs/instantiate"] = buildinstantiatestorage.NewStorage(buildGenerator)
		storage["builds/log"] = buildlogregistry.NewREST(buildRegistry, c.BuildLogClient(), kubeletClient)
	}

	return storage
}

func (c *MasterConfig) InstallUnprotectedAPI(container *restful.Container) []string {
	return []string{}
}

// initAPIVersionRoute initializes the osapi endpoint to behave similar to the upstream api endpoint
func initAPIVersionRoute(root *restful.WebService, prefix string, versions ...string) {
	if len(versions) == 0 {
		return
	}

	versionHandler := apiserver.APIVersionHandler(versions...)
	root.Route(root.GET(prefix).To(versionHandler).
		Doc("list supported server API versions").
		Produces(restful.MIME_JSON).
		Consumes(restful.MIME_JSON))
}

// initHealthCheckRoute initalizes an HTTP endpoint for health checking.
// OpenShift is deemed healthy if the API server can respond with an OK messages
func initHealthCheckRoute(root *restful.WebService, path string) {
	root.Route(root.GET(path).To(func(req *restful.Request, resp *restful.Response) {
		resp.ResponseWriter.WriteHeader(http.StatusOK)
		resp.ResponseWriter.Write([]byte("ok"))
	}).Doc("return the health state of the master").
		Returns(http.StatusOK, "if master is healthy", nil).
		Produces(restful.MIME_JSON))
}

// initReadinessCheckRoute initializes an HTTP endpoint for readiness checking
func initReadinessCheckRoute(root *restful.WebService, path string, readyFunc func() bool) {
	root.Route(root.GET(path).To(func(req *restful.Request, resp *restful.Response) {
		if readyFunc() {
			resp.ResponseWriter.WriteHeader(http.StatusOK)
			resp.ResponseWriter.Write([]byte("ok"))

		} else {
			resp.ResponseWriter.WriteHeader(http.StatusServiceUnavailable)
		}
	}).Doc("return the readiness state of the master").
		Returns(http.StatusOK, "if the master is ready", nil).
		Returns(http.StatusServiceUnavailable, "if the master is not ready", nil).
		Produces(restful.MIME_JSON))
}

func (c *MasterConfig) defaultAPIGroupVersion() *apiserver.APIGroupVersion {
	return &apiserver.APIGroupVersion{
		Root: OpenShiftAPIPrefix,

		Mapper: latest.RESTMapper,

		Creater:   kapi.Scheme,
		Typer:     kapi.Scheme,
		Convertor: kapi.Scheme,
		Linker:    latest.SelfLinker,

		Admit:   c.AdmissionControl,
		Context: c.getRequestContextMapper(),
	}
}

// api_v1beta3 returns the resources and codec for API version v1beta3.
func (c *MasterConfig) api_v1beta3(all map[string]rest.Storage) *apiserver.APIGroupVersion {
	storage := make(map[string]rest.Storage)
	for k, v := range all {
		if excludedV1Beta3Types.Has(k) {
			continue
		}
		storage[strings.ToLower(k)] = v
	}
	version := c.defaultAPIGroupVersion()
	version.Root = LegacyOpenShiftAPIPrefix
	version.Storage = storage
	version.Version = OpenShiftAPIV1Beta3
	version.Codec = v1beta3.Codec
	return version
}

// api_v1 returns the resources and codec for API version v1.
func (c *MasterConfig) api_v1(all map[string]rest.Storage) *apiserver.APIGroupVersion {
	storage := make(map[string]rest.Storage)
	for k, v := range all {
		if excludedV1Types.Has(k) {
			continue
		}
		storage[strings.ToLower(k)] = v
	}
	version := c.defaultAPIGroupVersion()
	version.Storage = storage
	version.Version = OpenShiftAPIV1
	version.Codec = v1.Codec
	return version
}

// getRequestContextMapper returns a mapper from requests to contexts, initializing it if needed
func (c *MasterConfig) getRequestContextMapper() kapi.RequestContextMapper {
	if c.RequestContextMapper == nil {
		c.RequestContextMapper = kapi.NewRequestContextMapper()
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

type clientDeploymentInterface struct {
	KubeClient kclient.Interface
}

// GetDeployment returns the deployment with the provided context and name
func (c clientDeploymentInterface) GetDeployment(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
	return c.KubeClient.ReplicationControllers(kapi.NamespaceValue(ctx)).Get(name)
}
