package origin

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kmaster "github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/authorizer"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/assets"
	"github.com/openshift/origin/pkg/auth/authenticator/token/bearertoken"
	authcontext "github.com/openshift/origin/pkg/auth/context"
	authfilter "github.com/openshift/origin/pkg/auth/handlers"
	buildapi "github.com/openshift/origin/pkg/build/api"
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
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploycontrollerfactory "github.com/openshift/origin/pkg/deploy/controller/factory"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/generator"
	deployregistry "github.com/openshift/origin/pkg/deploy/registry/deploy"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployetcd "github.com/openshift/origin/pkg/deploy/registry/etcd"
	deployrollback "github.com/openshift/origin/pkg/deploy/rollback"
	imageetcd "github.com/openshift/origin/pkg/image/registry/etcd"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorymapping"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorytag"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/accesstoken"
	authorizetokenregistry "github.com/openshift/origin/pkg/oauth/registry/authorizetoken"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/client"
	clientauthorizationregistry "github.com/openshift/origin/pkg/oauth/registry/clientauthorization"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	projectetcd "github.com/openshift/origin/pkg/project/registry/etcd"
	projectregistry "github.com/openshift/origin/pkg/project/registry/project"
	routeetcd "github.com/openshift/origin/pkg/route/registry/etcd"
	routeregistry "github.com/openshift/origin/pkg/route/registry/route"
	templateregistry "github.com/openshift/origin/pkg/template/registry"
	"github.com/openshift/origin/pkg/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
	"github.com/openshift/origin/pkg/version"
)

const (
	OpenShiftAPIPrefix        = "/osapi"
	OpenShiftAPIPrefixV1Beta1 = OpenShiftAPIPrefix + "/v1beta1"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	BindAddr       string
	MasterAddr     string
	AssetAddr      string
	KubernetesAddr string
	// external clients may need to access APIs at different addresses than internal components do
	MasterPublicAddr     string
	KubernetesPublicAddr string

	TLS bool

	CORSAllowedOrigins    []*regexp.Regexp
	RequireAuthentication bool

	EtcdHelper tools.EtcdHelper

	Authorizer       authorizer.Authorizer
	AdmissionControl admission.Interface

	MasterCertFile string
	MasterKeyFile  string
	AssetCertFile  string
	AssetKeyFile   string

	// kubeClient is the client used to call Kubernetes APIs from system components, built from KubeClientConfig.
	// It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically for that component.
	kubeClient *kclient.Client
	// KubeClientConfig is the client configuration used to call Kubernetes APIs from system components.
	// To apply different access control to a system component, create a client config specifically for that component.
	KubeClientConfig kclient.Config

	// osClient is the client used to call OpenShift APIs from system components, built from OSClientConfig.
	// It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically for that component.
	osClient *osclient.Client
	// OSClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	OSClientConfig kclient.Config

	// DeployerOSClientConfig is the client configuration used to call OpenShift APIs from launched deployer pods
	DeployerOSClientConfig kclient.Config
}

// APIInstaller installs additional API components into this server
type APIInstaller interface {
	// Returns an array of strings describing what was installed
	InstallAPI(*restful.Container) []string
}

func (c *MasterConfig) BuildClients() {
	kubeClient, err := kclient.New(&c.KubeClientConfig)
	if err != nil {
		glog.Fatalf("Unable to configure client: %v", err)
	}
	c.kubeClient = kubeClient

	osclient, err := osclient.New(&c.OSClientConfig)
	if err != nil {
		glog.Fatalf("Unable to configure client: %v", err)
	}
	c.osClient = osclient
}

func (c *MasterConfig) KubeClient() *kclient.Client {
	return c.kubeClient
}
func (c *MasterConfig) DeploymentClient() *kclient.Client {
	return c.kubeClient
}
func (c *MasterConfig) BuildLogClient() *kclient.Client {
	return c.kubeClient
}
func (c *MasterConfig) WebHookClient() *osclient.Client {
	return c.osClient
}
func (c *MasterConfig) BuildControllerClients() (*osclient.Client, *kclient.Client) {
	return c.osClient, c.kubeClient
}
func (c *MasterConfig) ImageChangeControllerClient() *osclient.Client {
	return c.osClient
}
func (c *MasterConfig) DeploymentControllerClients() (*osclient.Client, *kclient.Client) {
	return c.osClient, c.kubeClient
}

// DeployerClientConfig returns the client configuration a Deployer instance launched in a pod
// should use when making API calls.
func (c *MasterConfig) DeployerClientConfig() *kclient.Config {
	return &c.DeployerOSClientConfig
}

func (c *MasterConfig) DeploymentConfigControllerClients() (*osclient.Client, *kclient.Client) {
	return c.osClient, c.kubeClient
}
func (c *MasterConfig) DeploymentConfigChangeControllerClients() (*osclient.Client, *kclient.Client) {
	return c.osClient, c.kubeClient
}
func (c *MasterConfig) DeploymentImageChangeControllerClient() *osclient.Client {
	return c.osClient
}

// EnsureCORSAllowedOrigins takes a string list of origins and attempts to covert them to CORS origin
// regexes, or exits if it cannot.
func (c *MasterConfig) EnsureCORSAllowedOrigins(origins []string) {
	if len(origins) > 0 {
		allowedOriginRegexps, err := util.CompileRegexps(util.StringList(origins))
		if err != nil {
			glog.Fatalf("Invalid CORS allowed origin, --corsAllowedOrigins flag was set to %v - %v", strings.Join(origins, ","), err)
		}
		c.CORSAllowedOrigins = allowedOriginRegexps
	}
}

func (c *MasterConfig) InstallAPI(container *restful.Container) []string {
	buildEtcd := buildetcd.New(c.EtcdHelper)
	imageEtcd := imageetcd.New(c.EtcdHelper)
	deployEtcd := deployetcd.New(c.EtcdHelper)
	routeEtcd := routeetcd.New(c.EtcdHelper)
	projectEtcd := projectetcd.New(c.EtcdHelper)
	userEtcd := useretcd.New(c.EtcdHelper, user.NewDefaultUserInitStrategy())
	oauthEtcd := oauthetcd.New(c.EtcdHelper)

	osclient, kclient := c.DeploymentConfigControllerClients()
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		DeploymentInterface:       &oldClientDeploymentInterface{kclient},
		DeploymentConfigInterface: deployEtcd,
		ImageRepositoryInterface:  imageEtcd,
		Codec: latest.Codec,
	}

	deployRollbackGenerator := &deployrollback.RollbackGenerator{}
	rollbackDeploymentGetter := &clientDeploymentInterface{kclient}
	rollbackDeploymentConfigGetter := &clientDeploymentConfigInterface{osclient}

	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "")

	// initialize OpenShift API
	storage := map[string]apiserver.RESTStorage{
		"builds":       buildregistry.NewREST(buildEtcd),
		"buildConfigs": buildconfigregistry.NewREST(buildEtcd),
		"buildLogs":    buildlogregistry.NewREST(buildEtcd, c.BuildLogClient()),

		"images":                  image.NewREST(imageEtcd),
		"imageRepositories":       imagerepository.NewREST(imageEtcd, defaultRegistry),
		"imageRepositoryMappings": imagerepositorymapping.NewREST(imageEtcd, imageEtcd),
		"imageRepositoryTags":     imagerepositorytag.NewREST(imageEtcd, imageEtcd),

		"deployments":               deployregistry.NewREST(deployEtcd),
		"deploymentConfigs":         deployconfigregistry.NewREST(deployEtcd),
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, v1beta1.Codec),
		"deploymentConfigRollbacks": deployrollback.NewREST(deployRollbackGenerator, rollbackDeploymentGetter, rollbackDeploymentConfigGetter, latest.Codec),

		"templateConfigs": templateregistry.NewREST(),

		"routes": routeregistry.NewREST(routeEtcd),

		"projects": projectregistry.NewREST(projectEtcd),

		"userIdentityMappings": useridentitymapping.NewREST(userEtcd),
		"users":                userregistry.NewREST(userEtcd),

		"authorizeTokens":      authorizetokenregistry.NewREST(oauthEtcd),
		"accessTokens":         accesstokenregistry.NewREST(oauthEtcd),
		"clients":              clientregistry.NewREST(oauthEtcd),
		"clientAuthorizations": clientauthorizationregistry.NewREST(oauthEtcd),
	}

	whPrefix := OpenShiftAPIPrefixV1Beta1 + "/buildConfigHooks/"
	container.ServeMux.Handle(whPrefix, http.StripPrefix(whPrefix,
		webhook.NewController(ClientWebhookInterface{c.WebHookClient()}, map[string]webhook.Plugin{
			"generic": generic.New(),
			"github":  github.New(),
		})))

	admissionControl := admit.NewAlwaysAdmit()

	apiserver.NewAPIGroupVersion(storage, v1beta1.Codec, OpenShiftAPIPrefixV1Beta1, latest.SelfLinker, admissionControl).InstallREST(container, OpenShiftAPIPrefix, "v1beta1")

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
	versionHandler := apiserver.APIVersionHandler("v1beta1")
	root.Route(root.GET(OpenShiftAPIPrefix).To(versionHandler).Doc("list supported server API versions"))

	return []string{
		fmt.Sprintf("Started OpenShift API at %%s%s", OpenShiftAPIPrefixV1Beta1),
	}
}

// RunAPI launches the OpenShift master. It takes optional API installers that
// may install additional endpoints into the server.
func (c *MasterConfig) RunAPI(installers ...APIInstaller) {
	container := kmaster.NewHandlerContainer(http.NewServeMux())

	var extra []string
	for _, i := range installers {
		extra = append(extra, i.InstallAPI(container)...)
	}

	handler := http.Handler(container)
	if c.RequireAuthentication {
		handler = c.wireAuthenticationHandling(container.ServeMux, handler)
	}

	if len(c.CORSAllowedOrigins) > 0 {
		handler = apiserver.CORS(handler, c.CORSAllowedOrigins, nil, nil, "true")
	}

	server := &http.Server{
		Addr:           c.BindAddr,
		Handler:        handler,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		for _, s := range extra {
			glog.Infof(s, c.MasterAddr)
		}
		if c.TLS {
			server.TLSConfig = &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
				// Populate PeerCertificates in requests, but don't reject connections without certificates
				// This allows certificates to be validated by authenticators, while still allowing other auth types
				ClientAuth: tls.RequestClientCert,
			}
			glog.Fatal(server.ListenAndServeTLS(c.MasterCertFile, c.MasterKeyFile))
		} else {
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial("tcp", c.BindAddr, 100*time.Millisecond, 100*time.Millisecond, 100)
}

// wireAuthenticationHandling creates and binds all the objects that we only care about if authentication is turned on.  It's pulled out
// just to make the RunAPI method easier to read.  These resources include the requestsToUsers map that allows callers to know which user
// is requesting an operation, the handler wrapper that protects the passed handler behind a handler that requires valid authentication
// information on the request, and an endpoint that only functions properly with an authenticated user.
func (c *MasterConfig) wireAuthenticationHandling(osMux *http.ServeMux, handler http.Handler) http.Handler {
	// this tracks requests back to users for authorization.  The same instance must be shared between writers and readers
	requestsToUsers := authcontext.NewRequestContextMap()

	// wrapHandlerWithAuthentication binds a handler that will correlate the users and requests
	handler = c.wrapHandlerWithAuthentication(handler, requestsToUsers)

	// this requires the requests and users to be present
	userContextMap := userregistry.ContextFunc(func(req *http.Request) (userregistry.Info, bool) {
		obj, found := requestsToUsers.Get(req)
		if user, ok := obj.(userregistry.Info); found && ok {
			return user, true
		}
		return nil, false
	})
	// TODO: this is flawed, needs to be able to identify the right endpoints
	thisUserEndpoint := OpenShiftAPIPrefixV1Beta1 + "/users/~"
	userregistry.InstallThisUser(osMux, thisUserEndpoint, userContextMap, handler)

	return handler
}

// wrapHandlerWithAuthentication takes a handler and protects it behind a handler that tests to make sure that a user is authenticated.
// if the request does have value auth information, then the request is allowed through the passed handler.  If the request does not have
// valid auth information, then the request is passed to a failure handler.  Until we get authentication for system componenets, the
// failure handler logs and passes through.
func (c *MasterConfig) wrapHandlerWithAuthentication(handler http.Handler, requestsToUsers *authcontext.RequestContextMap) http.Handler {
	// wrap with authenticated token check
	tokenAuthenticator, err := GetTokenAuthenticator(c.EtcdHelper)
	if err != nil {
		glog.Fatalf("Error creating TokenAuthenticator: %v.  The oauth server cannot start!", err)
	}
	return authfilter.NewRequestAuthenticator(
		requestsToUsers,
		bearertoken.New(tokenAuthenticator),
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// TODO: make this failure handler actually fail once internal components can get auth tokens to do their job
			// w.WriteHeader(http.StatusUnauthorized)
			// return

			// For now, just let us know and continue on your merry way
			glog.V(2).Infof("Token authentication failed when accessing: %v", req.URL)
			handler.ServeHTTP(w, req)
		}),
		handler)
}

// RunAssetServer starts the asset server for the OpenShift UI.
func (c *MasterConfig) RunAssetServer() {
	// TODO use	version.Get().GitCommit as an etag cache header
	mux := http.NewServeMux()

	masterURL, err := url.Parse(c.MasterPublicAddr)
	if err != nil {
		glog.Fatalf("Error parsing master url: %v", err)
	}

	k8sURL, err := url.Parse(c.KubernetesPublicAddr)
	if err != nil {
		glog.Fatalf("Error parsing kubernetes url: %v", err)
	}

	mux.Handle("/",
		// Gzip first so that inner handlers can react to the addition of the Vary header
		assets.GzipHandler(
			// Cache control should happen after all Vary headers are added, but before
			// any asset related routing (HTML5ModeHandler and FileServer)
			assets.CacheControlHandler(
				version.Get().GitCommit,
				assets.GeneratedConfigHandler(
					masterURL.Host,
					OpenShiftAPIPrefix,
					k8sURL.Host,
					"/api",
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
		Addr:           c.AssetAddr,
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
				// Populate PeerCertificates in requests, but don't reject connections without certificates
				// This allows certificates to be validated by authenticators, while still allowing other auth types
				ClientAuth: tls.RequestClientCert,
			}
			glog.Infof("Started OpenShift static asset server at https://%s", c.AssetAddr)
			glog.Fatal(server.ListenAndServeTLS(c.AssetCertFile, c.AssetKeyFile))
		} else {
			glog.Infof("Started OpenShift static asset server at http://%s", c.AssetAddr)
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial("tcp", c.AssetAddr, 100*time.Millisecond, 100*time.Millisecond, 100)
}

// RunBuildController starts the build sync loop for builds and buildConfig processing.
func (c *MasterConfig) RunBuildController() {
	// initialize build controller
	dockerImage := env("OPENSHIFT_DOCKER_BUILDER_IMAGE", "openshift/origin-docker-builder")
	stiImage := env("OPENSHIFT_STI_BUILDER_IMAGE", "openshift/origin-sti-builder")
	useLocalImages := env("USE_LOCAL_IMAGES", "true") == "true"

	osclient, kclient := c.BuildControllerClients()
	factory := buildcontrollerfactory.BuildControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image:          dockerImage,
			UseLocalImages: useLocalImages,
		},
		STIBuildStrategy: &buildstrategy.STIBuildStrategy{
			Image:                stiImage,
			TempDirectoryCreator: buildstrategy.STITempDirectoryCreator,
			UseLocalImages:       useLocalImages,
		},
		CustomBuildStrategy: &buildstrategy.CustomBuildStrategy{
			UseLocalImages: useLocalImages,
		},
	}

	controller := factory.Create()
	controller.Run()
}

// RunDeploymentController starts the build image change trigger controller process.
func (c *MasterConfig) RunBuildImageChangeTriggerController() {
	factory := buildcontrollerfactory.ImageChangeControllerFactory{Client: c.ImageChangeControllerClient()}
	factory.Create().Run()
}

// RunDeploymentController starts the deployment controller process.
func (c *MasterConfig) RunDeploymentController() {
	osclient, kclient := c.DeploymentControllerClients()
	factory := deploycontrollerfactory.DeploymentControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      latest.Codec,
		Environment: []api.EnvVar{
			{Name: "KUBERNETES_MASTER", Value: c.MasterAddr},
			{Name: "OPENSHIFT_MASTER", Value: c.MasterAddr},
		},
		UseLocalImages:        env("USE_LOCAL_IMAGES", "true") == "true",
		RecreateStrategyImage: env("OPENSHIFT_DEPLOY_RECREATE_IMAGE", "openshift/origin-deployer"),
	}

	envvars := clientcmd.EnvVarsFromConfig(c.DeployerClientConfig())
	factory.Environment = append(factory.Environment, envvars...)

	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentConfigController() {
	osclient, kclient := c.DeploymentConfigControllerClients()
	factory := deploycontrollerfactory.DeploymentConfigControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      latest.Codec,
	}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentConfigChangeController() {
	osclient, kclient := c.DeploymentConfigChangeControllerClients()
	factory := deploycontrollerfactory.DeploymentConfigChangeControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      latest.Codec,
	}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentImageChangeTriggerController() {
	osclient := c.DeploymentImageChangeControllerClient()
	factory := deploycontrollerfactory.ImageChangeControllerFactory{Client: osclient}
	controller := factory.Create()
	controller.Run()
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

// ClientWebhookInterface is a webhookBuildInterface which delegates to the OpenShift client interfaces
type ClientWebhookInterface struct {
	Client osclient.Interface
}

// CreateBuild creates build using OpenShift client.
func (c ClientWebhookInterface) CreateBuild(namespace string, build *buildapi.Build) (*buildapi.Build, error) {
	return c.Client.Builds(namespace).Create(build)
}

// GetBuildConfig returns buildConfig using OpenShift client.
func (c ClientWebhookInterface) GetBuildConfig(namespace, name string) (*buildapi.BuildConfig, error) {
	return c.Client.BuildConfigs(namespace).Get(name)
}

type oldClientDeploymentInterface struct {
	KubeClient kclient.Interface
}

func (c *oldClientDeploymentInterface) GetDeployment(ctx api.Context, name string) (*api.ReplicationController, error) {
	return c.KubeClient.ReplicationControllers(api.Namespace(ctx)).Get(name)
}

type clientDeploymentInterface struct {
	KubeClient kclient.Interface
}

func (c *clientDeploymentInterface) GetDeployment(namespace, name string) (*api.ReplicationController, error) {
	return c.KubeClient.ReplicationControllers(namespace).Get(name)
}

type clientDeploymentConfigInterface struct {
	Client osclient.Interface
}

func (c *clientDeploymentConfigInterface) GetDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error) {
	return c.Client.DeploymentConfigs(namespace).Get(name)
}
