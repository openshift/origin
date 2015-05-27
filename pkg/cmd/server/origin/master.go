package origin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"bitbucket.org/ww/goautoneg"

	etcdclient "github.com/coreos/go-etcd/etcd"
	restful "github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kmaster "github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/serviceaccount"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/api/v1beta3"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
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
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configchangecontroller "github.com/openshift/origin/pkg/deploy/controller/configchange"
	deployerpodcontroller "github.com/openshift/origin/pkg/deploy/controller/deployerpod"
	deploycontroller "github.com/openshift/origin/pkg/deploy/controller/deployment"
	deployconfigcontroller "github.com/openshift/origin/pkg/deploy/controller/deploymentconfig"
	imagechangecontroller "github.com/openshift/origin/pkg/deploy/controller/imagechange"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/generator"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployconfigetcd "github.com/openshift/origin/pkg/deploy/registry/deployconfig/etcd"
	deployrollback "github.com/openshift/origin/pkg/deploy/rollback"
	"github.com/openshift/origin/pkg/dns"
	imagecontroller "github.com/openshift/origin/pkg/image/controller"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorymapping"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorytag"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	"github.com/openshift/origin/pkg/image/registry/imagestreammapping"
	"github.com/openshift/origin/pkg/image/registry/imagestreamtag"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	authorizetokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken/etcd"
	clientetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclient/etcd"
	clientauthetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization/etcd"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	projectcontroller "github.com/openshift/origin/pkg/project/controller"
	projectproxy "github.com/openshift/origin/pkg/project/registry/project/proxy"
	projectrequeststorage "github.com/openshift/origin/pkg/project/registry/projectrequest/delegated"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	routeetcd "github.com/openshift/origin/pkg/route/registry/etcd"
	routeregistry "github.com/openshift/origin/pkg/route/registry/route"
	clusternetworketcd "github.com/openshift/origin/pkg/sdn/registry/clusternetwork/etcd"
	hostsubnetetcd "github.com/openshift/origin/pkg/sdn/registry/hostsubnet/etcd"
	"github.com/openshift/origin/pkg/service"
	templateregistry "github.com/openshift/origin/pkg/template/registry"
	templateetcd "github.com/openshift/origin/pkg/template/registry/etcd"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
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
	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/plugins/osdn"
	routeplugin "github.com/openshift/origin/plugins/route/allocation/simple"
)

const (
	OpenShiftAPIPrefix        = "/osapi" // TODO: make configurable
	KubernetesAPIPrefix       = "/api"   // TODO: make configurable
	OpenShiftAPIV1Beta1       = "v1beta1"
	OpenShiftAPIV1Beta3       = "v1beta3"
	OpenShiftAPIV1            = "v1"
	OpenShiftAPIPrefixV1Beta1 = OpenShiftAPIPrefix + "/" + OpenShiftAPIV1Beta1
	OpenShiftAPIPrefixV1Beta3 = OpenShiftAPIPrefix + "/" + OpenShiftAPIV1Beta3
	OpenShiftAPIPrefixV1      = "/oapi" + "/" + OpenShiftAPIV1
	OpenShiftRouteSubdomain   = "router.default.local"
	swaggerAPIPrefix          = "/swaggerapi/"
)

var (
	excludedV1Beta3Types = util.NewStringSet(
		"templateConfigs", "deployments", "buildLogs",
		"imageRepositories", "imageRepositories/status", "imageRepositoryMappings", "imageRepositoryTags",
	)
	excludedV1Types = excludedV1Beta3Types
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

func (c *MasterConfig) InstallProtectedAPI(container *restful.Container) []string {
	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(c.KubeClient().Services(api.NamespaceDefault).Get)
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
	imageStreamMappingRegistry := imagestreammapping.NewRegistry(imageStreamMappingStorage)
	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamTagRegistry := imagestreamtag.NewRegistry(imageStreamTagStorage)
	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamImageRegistry := imagestreamimage.NewRegistry(imageStreamImageStorage)

	imageRepositoryStorage, imageRepositoryStatusStorage := imagerepository.NewREST(imageStreamRegistry)
	imageRepositoryMappingStorage := imagerepositorymapping.NewREST(imageStreamMappingRegistry)
	imageRepositoryTagStorage := imagerepositorytag.NewREST(imageStreamTagRegistry)

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
	}
	buildClone, buildConfigInstantiate := buildgenerator.NewREST(buildGenerator)

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
	projectRequestStorage := projectrequeststorage.NewREST(c.Options.ProjectConfig.ProjectRequestMessage, namespace, templateName, c.PrivilegedLoopbackOpenShiftClient)

	bcClient, _ := c.BuildControllerClients()
	buildConfigWebHooks := buildconfigregistry.NewWebHookREST(
		buildConfigRegistry,
		buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient),
		map[string]webhook.Plugin{
			"generic": generic.New(),
			"github":  github.New(),
		},
	)

	// initialize OpenShift API
	storage := map[string]rest.Storage{
		"builds":                   buildStorage,
		"builds/clone":             buildClone,
		"buildConfigs":             buildConfigStorage,
		"buildConfigs/webhooks":    buildConfigWebHooks,
		"buildConfigs/instantiate": buildConfigInstantiate,
		"buildLogs":                buildlogregistry.NewREST(buildRegistry, c.BuildLogClient(), kubeletClient),
		"builds/log":               buildlogregistry.NewREST(buildRegistry, c.BuildLogClient(), kubeletClient),

		"images":                   imageStorage,
		"imageStreams":             imageStreamStorage,
		"imageStreams/status":      imageStreamStatusStorage,
		"imageStreamImages":        imageStreamImageStorage,
		"imageStreamMappings":      imageStreamMappingStorage,
		"imageStreamTags":          imageStreamTagStorage,
		"imageRepositories":        imageRepositoryStorage,
		"imageRepositories/status": imageRepositoryStatusStorage,
		"imageRepositoryMappings":  imageRepositoryMappingStorage,
		"imageRepositoryTags":      imageRepositoryTagStorage,

		"deploymentConfigs":         deployConfigStorage,
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, latest.Codec),
		"deploymentConfigRollbacks": deployrollback.NewREST(deployRollbackClient, latest.Codec),

		"processedTemplates": templateregistry.NewREST(false),
		"templates":          templateetcd.NewREST(c.EtcdHelper),
		// DEPRECATED: remove with v1beta1
		"templateConfigs": templateregistry.NewREST(true),

		"routes": routeregistry.NewREST(routeEtcd, routeAllocator),

		"projects":        projectStorage,
		"projectRequests": projectRequestStorage,

		"hostSubnets":     hostSubnetStorage,
		"clusterNetworks": clusterNetworkStorage,

		"users":                userStorage,
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

	if err := c.api_v1beta1(storage).InstallREST(container); err != nil {
		glog.Fatalf("Unable to initialize v1beta1 API: %v", err)
	}

	if err := c.api_v1beta3(storage).InstallREST(container); err != nil {
		glog.Fatalf("Unable to initialize v1beta3 API: %v", err)
	}

	if err := c.api_v1(storage).InstallREST(container); err != nil {
		glog.Fatalf("Unable to initialize v1 API: %v", err)
	}

	var root *restful.WebService
	for _, svc := range container.RegisteredWebServices() {
		switch svc.RootPath() {
		case "/":
			root = svc
		case OpenShiftAPIPrefixV1Beta1:
			svc.Doc("OpenShift REST API, version v1beta1").ApiVersion("v1beta1")
		case OpenShiftAPIPrefixV1Beta3:
			svc.Doc("OpenShift REST API, version v1beta3").ApiVersion("v1beta3")
		case OpenShiftAPIPrefixV1:
			svc.Doc("OpenShift REST API, version v1").ApiVersion("v1")
		}
	}
	if root == nil {
		root = new(restful.WebService)
		container.Add(root)
	}
	initAPIVersionRoute(root, "v1beta1", "v1beta3", "v1")

	return []string{
		fmt.Sprintf("Started OpenShift API at %%s%s (deprecated)", OpenShiftAPIPrefixV1Beta1),
		fmt.Sprintf("Started OpenShift API at %%s%s", OpenShiftAPIPrefixV1Beta3),
		fmt.Sprintf("Started OpenShift API at %%s%s (experimental)", OpenShiftAPIPrefixV1),
	}
}

func (c *MasterConfig) InstallUnprotectedAPI(container *restful.Container) []string {
	bcClient, _ := c.BuildControllerClients()
	bcGetterUpdater := buildclient.NewOSClientBuildConfigClient(bcClient)
	handler := webhook.NewController(
		bcGetterUpdater,
		buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient),
		map[string]webhook.Plugin{
			"generic": generic.New(),
			"github":  github.New(),
		})

	// TODO: deprecated, remove when v1beta1 is dropped
	prefix := OpenShiftAPIPrefixV1Beta1 + "/buildConfigHooks/"
	container.Handle(prefix, http.StripPrefix(prefix, handler))
	return []string{}
}

//initAPIVersionRoute initializes the osapi endpoint to behave similar to the upstream api endpoint
func initAPIVersionRoute(root *restful.WebService, versions ...string) {
	versionHandler := apiserver.APIVersionHandler(versions...)
	root.Route(root.GET(OpenShiftAPIPrefix).To(versionHandler).
		Doc("list supported server API versions").
		Produces(restful.MIME_JSON).
		Consumes(restful.MIME_JSON))
}

// If we know the location of the asset server, redirect to it when / is requested
// and the Accept header supports text/html
func assetServerRedirect(handler http.Handler, assetPublicURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			accepts := goautoneg.ParseAccept(req.Header.Get("Accept"))
			for _, accept := range accepts {
				if accept.Type == "text" && accept.SubType == "html" {
					http.Redirect(w, req, assetPublicURL, http.StatusFound)
					return
				}
			}
		}
		// Dispatch to the next handler
		handler.ServeHTTP(w, req)
	})
}

// TODO We would like to use the IndexHandler from k8s but we do not yet have a
// MuxHelper to track all registered paths
func indexAPIPaths(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			// TODO once we have a MuxHelper we will not need to hardcode this list of paths
			object := api.RootPaths{Paths: []string{
				"/api",
				"/api/v1beta1",
				"/api/v1beta3",
				"/api/v1beta3",
				"/healthz",
				"/healthz/ping",
				"/logs/",
				"/metrics",
				"/osapi",
				"/osapi/v1beta1",
				"/osapi/v1beta3",
				"/oapi/v1",
				"/swaggerapi/",
			}}
			// TODO it would be nice if apiserver.writeRawJSON was not private
			output, err := json.MarshalIndent(object, "", "  ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(output)
		} else {
			// Dispatch to the next handler
			handler.ServeHTTP(w, req)
		}
	})
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

	handler = indexAPIPaths(handler)

	open.Handle("/", handler)

	// install swagger
	swaggerConfig := swagger.Config{
		WebServicesUrl: c.Options.MasterPublicURL,
		WebServices:    append(safe.RegisteredWebServices(), open.RegisteredWebServices()...),
		ApiPath:        swaggerAPIPrefix,
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

	if c.Options.AssetConfig != nil {
		handler = assetServerRedirect(handler, c.Options.AssetConfig.PublicURL)
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
	c.ensureOpenShiftSharedResourcesNamespace()
	go util.Forever(func() {
		c.ensureOpenShiftSharedResourcesNamespace()
	}, 10*time.Second)
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

// api_v1beta1 returns the resources and codec for API version v1beta1.
func (c *MasterConfig) api_v1beta1(all map[string]rest.Storage) *apiserver.APIGroupVersion {
	storage := make(map[string]rest.Storage)
	for k, v := range all {
		storage[strings.ToLower(k)] = v
		storage[k] = v
	}
	version := c.defaultAPIGroupVersion()
	version.Storage = storage
	version.Version = OpenShiftAPIV1Beta1
	version.Codec = v1beta1.Codec
	return version
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

// ensureOpenShiftSharedResourcesNamespace is called as part of global policy initialization to ensure shared namespace exists
func (c *MasterConfig) ensureOpenShiftSharedResourcesNamespace() {
	namespace, err := c.KubeClient().Namespaces().Get(c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace)
	if err != nil {
		namespace = &kapi.Namespace{
			ObjectMeta: kapi.ObjectMeta{Name: c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace},
			Spec: kapi.NamespaceSpec{
				Finalizers: []kapi.FinalizerName{projectapi.FinalizerProject},
			},
		}
		kapi.FillObjectMetaSystemFields(api.NewContext(), &namespace.ObjectMeta)
		_, err = c.KubeClient().Namespaces().Create(namespace)
		if err != nil {
			glog.Errorf("Error creating namespace: %v due to %v\n", namespace, err)
		}
	}
}

// ensureComponentAuthorizationRules initializes the cluster policies
func (c *MasterConfig) ensureComponentAuthorizationRules() {
	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterpolicystorage.NewStorage(c.EtcdHelper))
	ctx := kapi.WithNamespace(kapi.NewContext(), "")

	if _, err := clusterPolicyRegistry.GetClusterPolicy(ctx, authorizationapi.PolicyName); kapierror.IsNotFound(err) {
		glog.Infof("No cluster policy found.  Creating bootstrap policy based on: %v", c.Options.PolicyConfig.BootstrapPolicyFile)

		if err := admin.OverwriteBootstrapPolicy(c.EtcdHelper, c.Options.PolicyConfig.BootstrapPolicyFile, admin.CreateBootstrapPolicyFileFullCommand, true, ioutil.Discard); err != nil {
			glog.Errorf("Error creating bootstrap policy: %v", err)
		}

	} else {
		glog.V(2).Infof("Ignoring bootstrap policy file because cluster policy found")
	}
}

func (c *MasterConfig) authorizationFilter(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attributes, err := c.AuthorizationAttributeBuilder.GetAttributes(req)
		if err != nil {
			forbidden(err.Error(), "", w, req)
			return
		}
		if attributes == nil {
			forbidden("No attributes", "", w, req)
			return
		}

		ctx, exists := c.RequestContextMapper.Get(req)
		if !exists {
			forbidden("context not found", attributes.GetAPIVersion(), w, req)
			return
		}

		allowed, reason, err := c.Authorizer.Authorize(ctx, attributes)
		if err != nil {
			forbidden(err.Error(), attributes.GetAPIVersion(), w, req)
			return
		}
		if !allowed {
			forbidden(reason, attributes.GetAPIVersion(), w, req)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

// forbidden renders a simple forbidden error
func forbidden(reason, apiVersion string, w http.ResponseWriter, req *http.Request) {
	// the api version can be empty for two basic reasons:
	// 1. malformed API request
	// 2. not an API request at all
	// In these cases, just assume the latest version that will work better than nothing
	if len(apiVersion) == 0 {
		apiVersion = klatest.Version
	}

	// Reason is an opaque string that describes why access is allowed or forbidden (forbidden by the time we reach here).
	// We don't have direct access to kind or name (not that those apply either in the general case)
	// We create a NewForbidden to stay close the API, but then we override the message to get a serialization
	// that makes sense when a human reads it.
	forbiddenError, _ := kapierror.NewForbidden("", "", errors.New("")).(*kapierror.StatusError)
	forbiddenError.ErrStatus.Message = reason

	// Not all API versions in valid API requests will have a matching codec in kubernetes.  If we can't find one,
	// just default to the latest kube codec.
	codec := klatest.Codec
	if requestedCodec, err := klatest.InterfacesFor(apiVersion); err == nil {
		codec = requestedCodec
	}

	formatted := &bytes.Buffer{}
	output, err := codec.Encode(&forbiddenError.ErrStatus)
	if err != nil {
		fmt.Fprintf(formatted, "%s", forbiddenError.Error())
	} else {
		_ = json.Indent(formatted, output, "", "  ")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write(formatted.Bytes())
}

// RunProjectAuthorizationCache starts the project authorization cache
func (c *MasterConfig) RunProjectAuthorizationCache() {
	// TODO: look at exposing a configuration option in future to control how often we run this loop
	period := 1 * time.Second
	c.ProjectAuthorizationCache.Run(period)
}

// RunOriginNamespaceController starts the controller that takes part in namespace termination of openshift content
func (c *MasterConfig) RunOriginNamespaceController() {
	osclient, kclient := c.OriginNamespaceControllerClients()
	factory := projectcontroller.NamespaceControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
	}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunServiceAccountsController() {
	if len(c.Options.ServiceAccountConfig.ManagedNames) == 0 {
		glog.Infof("Skipped starting Service Account Manager, no managed names specified")
		return
	}
	options := serviceaccount.DefaultServiceAccountsControllerOptions()
	options.Names = util.NewStringSet(c.Options.ServiceAccountConfig.ManagedNames...)
	serviceaccount.NewServiceAccountsController(c.KubeClient(), options).Run()
	glog.Infof("Started Service Account Manager")
}

func (c *MasterConfig) RunServiceAccountTokensController() {
	if len(c.Options.ServiceAccountConfig.PrivateKeyFile) == 0 {
		glog.Infof("Skipped starting Service Account Token Controller, no private key specified")
		return
	}

	privateKey, err := serviceaccount.ReadPrivateKey(c.Options.ServiceAccountConfig.PrivateKeyFile)
	if err != nil {
		glog.Fatalf("Error reading signing key for service account token controller: %v", err)
	}
	options := serviceaccount.DefaultTokenControllerOptions(serviceaccount.JWTTokenGenerator(privateKey))

	serviceaccount.NewTokensController(c.KubeClient(), options).Run()
	glog.Infof("Started Service Account Token Manager")
}

// RunPolicyCache starts the policy cache
func (c *MasterConfig) RunPolicyCache() {
	c.PolicyCache.Run()
}

// RunAssetServer starts the asset server for the OpenShift UI.
func (c *MasterConfig) RunAssetServer() {

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

// RunProjectCache populates project cache, used by scheduler and project admission controller.
func (c *MasterConfig) RunProjectCache() {
	glog.Infof("Using default project node label selector: %s", c.Options.ProjectConfig.DefaultNodeSelector)
	projectcache.RunProjectCache(c.PrivilegedLoopbackKubernetesClient, c.Options.ProjectConfig.DefaultNodeSelector)
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
		SourceBuildStrategy: &buildstrategy.SourceBuildStrategy{
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
	deleteController := factory.CreateDeleteController()
	deleteController.Run()
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
	deletecontroller := factory.CreateDeleteController()
	deletecontroller.Run()
}

// RunBuildImageChangeTriggerController starts the build image change trigger controller process.
func (c *MasterConfig) RunBuildImageChangeTriggerController() {
	bcClient, _ := c.BuildControllerClients()
	bcUpdater := buildclient.NewOSClientBuildConfigClient(bcClient)
	bcInstantiator := buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient)
	factory := buildcontrollerfactory.ImageChangeControllerFactory{Client: bcClient, BuildConfigInstantiator: bcInstantiator, BuildConfigUpdater: bcUpdater}
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
		KubeClient:    kclient,
		Codec:         latest.Codec,
		Environment:   env,
		DeployerImage: c.ImageFor("deployer"),
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

func (c *MasterConfig) RunDeploymentCancellationController() {
	kclient := c.DeploymentCancellationControllerClient()
	factory := deployconfigcontroller.DeploymentConfigControllerFactory{
		KubeClient: kclient,
		Codec:      latest.Codec,
	}
	controller := factory.Create()
	controller.Run()
}

// SDN controller runs openshift-sdn if the said network plugin is provided
func (c *MasterConfig) RunSDNController() {
	if c.Options.NetworkConfig.NetworkPluginName == osdn.NetworkPluginName() {
		osdn.Master(*c.SdnClient(), *c.KubeClient(), c.Options.NetworkConfig.ClusterNetworkCIDR, c.Options.NetworkConfig.HostSubnetLength)
	}
}

// RouteAllocator returns a route allocation controller.
func (c *MasterConfig) RouteAllocator() *routeallocationcontroller.RouteAllocationController {
	factory := routeallocationcontroller.RouteAllocationControllerFactory{
		OSClient:   c.PrivilegedLoopbackOpenShiftClient,
		KubeClient: c.KubeClient(),
	}

	subdomain := env("OPENSHIFT_ROUTE_SUBDOMAIN", OpenShiftRouteSubdomain)

	plugin, err := routeplugin.NewSimpleAllocationPlugin(subdomain)
	if err != nil {
		glog.Fatalf("Route plugin initialization failed: %v", err)
	}

	return factory.Create(plugin)
}

func (c *MasterConfig) RunImageImportController() {
	osclient := c.ImageImportControllerClient()
	factory := imagecontroller.ImportControllerFactory{
		Client: osclient,
	}
	controller := factory.Create()
	controller.Run()
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
