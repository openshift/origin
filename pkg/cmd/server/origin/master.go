package origin

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/assets"
	"github.com/openshift/origin/pkg/build"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildlogregistry "github.com/openshift/origin/pkg/build/registry/buildlog"
	buildetcd "github.com/openshift/origin/pkg/build/registry/etcd"
	"github.com/openshift/origin/pkg/build/strategy"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/github"
	osclient "github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	deploycontrollerfactory "github.com/openshift/origin/pkg/deploy/controller/factory"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/generator"
	deployregistry "github.com/openshift/origin/pkg/deploy/registry/deploy"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployetcd "github.com/openshift/origin/pkg/deploy/registry/etcd"
	imageetcd "github.com/openshift/origin/pkg/image/registry/etcd"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorymapping"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/accesstoken"
	authorizetokenregistry "github.com/openshift/origin/pkg/oauth/registry/authorizetoken"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/client"
	clientauthorizationregistry "github.com/openshift/origin/pkg/oauth/registry/clientauthorization"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	projectetcd "github.com/openshift/origin/pkg/project/registry/etcd"
	projectregistry "github.com/openshift/origin/pkg/project/registry/project"
	routeetcd "github.com/openshift/origin/pkg/route/registry/etcd"
	routeregistry "github.com/openshift/origin/pkg/route/registry/route"
	"github.com/openshift/origin/pkg/template"
	"github.com/openshift/origin/pkg/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
	"github.com/openshift/origin/pkg/version"

	// Register versioned api types
	_ "github.com/openshift/origin/pkg/config/api/v1beta1"
	_ "github.com/openshift/origin/pkg/image/api/v1beta1"
	_ "github.com/openshift/origin/pkg/project/api/v1beta1"
	_ "github.com/openshift/origin/pkg/route/api/v1beta1"
	_ "github.com/openshift/origin/pkg/template/api/v1beta1"
)

const (
	OpenShiftAPIPrefixV1Beta1 = "/osapi/v1beta1"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	BindAddr   string
	MasterAddr string
	AssetAddr  string

	CORSAllowedOrigins []*regexp.Regexp

	EtcdHelper tools.EtcdHelper

	KubeClient *kubeclient.Client
	OSClient   *osclient.Client
}

// APIInstaller installs additional API components into this server
type APIInstaller interface {
	// Returns an array of strings describing what was installed
	InstallAPI(cmdutil.Mux) []string
}

// EnsureKubernetesClient creates a Kubernetes client or exits if the client cannot be created.
func (c *MasterConfig) EnsureKubernetesClient() {
	kubeClient, err := kubeclient.New(&kubeclient.Config{Host: c.MasterAddr, Version: klatest.Version})
	if err != nil {
		glog.Fatalf("Unable to configure client: %v", err)
	}
	c.KubeClient = kubeClient
}

// EnsureOpenShiftClient creates an OpenShift client or exits if the client cannot be created.
func (c *MasterConfig) EnsureOpenShiftClient() {
	osClient, err := osclient.New(&kubeclient.Config{Host: c.MasterAddr, Version: latest.Version})
	if err != nil {
		glog.Fatalf("Unable to configure client: %v", err)
	}
	c.OSClient = osClient
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

// RunAPI launches the OpenShift master. It takes an optional API installer that
// may install additional endpoints into the server.
func (c *MasterConfig) RunAPI(installers ...APIInstaller) {
	buildEtcd := buildetcd.New(c.EtcdHelper)
	imageEtcd := imageetcd.New(c.EtcdHelper)
	deployEtcd := deployetcd.New(c.EtcdHelper)
	routeEtcd := routeetcd.New(c.EtcdHelper)
	projectEtcd := projectetcd.New(c.EtcdHelper)
	userEtcd := useretcd.New(c.EtcdHelper, user.NewDefaultUserInitStrategy())
	oauthEtcd := oauthetcd.New(c.EtcdHelper)

	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		DeploymentInterface:       deployEtcd,
		DeploymentConfigInterface: deployEtcd,
		ImageRepositoryInterface:  imageEtcd,
	}

	// initialize OpenShift API
	storage := map[string]apiserver.RESTStorage{
		"builds":       buildregistry.NewREST(buildEtcd),
		"buildConfigs": buildconfigregistry.NewREST(buildEtcd),
		"buildLogs":    buildlogregistry.NewREST(buildEtcd, c.KubeClient, "/proxy/minion"),

		"images":                  image.NewREST(imageEtcd),
		"imageRepositories":       imagerepository.NewREST(imageEtcd),
		"imageRepositoryMappings": imagerepositorymapping.NewREST(imageEtcd, imageEtcd),

		"deployments":               deployregistry.NewREST(deployEtcd),
		"deploymentConfigs":         deployconfigregistry.NewREST(deployEtcd),
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, v1beta1.Codec),

		"templateConfigs": template.NewStorage(),

		"routes": routeregistry.NewREST(routeEtcd),

		"projects": projectregistry.NewREST(projectEtcd),

		"userIdentityMappings": useridentitymapping.NewREST(userEtcd),
		"users":                userregistry.NewREST(userEtcd),

		"authorizeTokens":      authorizetokenregistry.NewREST(oauthEtcd),
		"accessTokens":         accesstokenregistry.NewREST(oauthEtcd),
		"clients":              clientregistry.NewREST(oauthEtcd),
		"clientAuthorizations": clientauthorizationregistry.NewREST(oauthEtcd),
	}

	osMux := http.NewServeMux()

	whPrefix := OpenShiftAPIPrefixV1Beta1 + "/buildConfigHooks/"
	osMux.Handle(whPrefix, http.StripPrefix(whPrefix,
		webhook.NewController(c.OSClient, map[string]webhook.Plugin{
			"github": github.New(),
		})))

	var extra []string
	for _, i := range installers {
		extra = append(extra, i.InstallAPI(osMux)...)
	}
	apiserver.NewAPIGroup(storage, v1beta1.Codec, OpenShiftAPIPrefixV1Beta1, latest.SelfLinker).InstallREST(osMux, OpenShiftAPIPrefixV1Beta1)
	apiserver.InstallSupport(osMux)

	handler := http.Handler(osMux)
	if len(c.CORSAllowedOrigins) > 0 {
		handler = apiserver.CORS(handler, c.CORSAllowedOrigins, nil, nil, "true")
	}

	handler = apiserver.RecoverPanics(handler)

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
		glog.Infof("Started OpenShift API at %s%s", c.MasterAddr, OpenShiftAPIPrefixV1Beta1)
		glog.Fatal(server.ListenAndServe())
	}, 0)
}

// RunAssetServer starts the asset server for the OpenShift UI.
func (c *MasterConfig) RunAssetServer() {
	// TODO prefix should be able to be overridden at the command line
	// move this out to a helper / config
	prefix := fmt.Sprintf("/assets/%s/", version.Get().GitCommit)

	mux := http.NewServeMux()

	// TODO - For now redirect requests to the root to the commit-based index.html URL
	// Next step is to have the root page served without redirecting.  May require build
	// changes or altering index.html while serving.
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		urlStr := fmt.Sprintf("%sindex.html", prefix)
		http.Redirect(w, req, urlStr, http.StatusTemporaryRedirect)
	}))

	mux.Handle(prefix, http.StripPrefix(prefix, http.FileServer(
		&assetfs.AssetFS{assets.Asset, assets.AssetDir, ""})))

	server := &http.Server{
		Addr:           c.AssetAddr,
		Handler:        mux,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		glog.Infof("Started OpenShift static asset server at http://%s", c.AssetAddr)
		glog.Fatal(server.ListenAndServe())
	}, 0)
}

// RunBuildController starts the build sync loop for builds and buildConfig processing.
func (c *MasterConfig) RunBuildController() {
	// initialize build controller
	dockerBuilderImage := env("OPENSHIFT_DOCKER_BUILDER_IMAGE", "openshift/docker-builder")
	stiBuilderImage := env("OPENSHIFT_STI_BUILDER_IMAGE", "openshift/sti-builder")

	buildStrategies := map[buildapi.BuildType]build.BuildJobStrategy{
		buildapi.DockerBuildType: strategy.NewDockerBuildStrategy(dockerBuilderImage),
		buildapi.STIBuildType:    strategy.NewSTIBuildStrategy(stiBuilderImage, strategy.STITempDirectoryCreator),
	}

	buildController := build.NewBuildController(c.KubeClient, c.OSClient, buildStrategies, 1200)
	buildController.Run(10 * time.Second)
}

// RunDeploymentController starts the deployment controller process.
func (c *MasterConfig) RunCustomPodDeploymentController() {
	factory := deploycontrollerfactory.CustomPodDeploymentControllerFactory{
		Client:     c.OSClient,
		KubeClient: c.KubeClient,
		Environment: []api.EnvVar{
			api.EnvVar{Name: "KUBERNETES_MASTER", Value: c.MasterAddr},
		},
	}

	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunBasicDeploymentController() {
	factory := deploycontrollerfactory.BasicDeploymentControllerFactory{
		Client:     c.OSClient,
		KubeClient: c.KubeClient,
	}

	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentConfigController() {
	factory := deploycontrollerfactory.DeploymentConfigControllerFactory{c.OSClient}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentConfigChangeController() {
	factory := deploycontrollerfactory.DeploymentConfigChangeControllerFactory{c.OSClient}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentImageChangeTriggerController() {
	factory := deploycontrollerfactory.ImageChangeControllerFactory{c.OSClient}
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
	return tools.EtcdHelper{client, interfaces.Codec, interfaces.ResourceVersioner}, nil
}

// env returns an environment variable, or the defaultValue if it is not set.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	} else {
		return val
	}
}
