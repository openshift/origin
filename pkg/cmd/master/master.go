package master

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/controller"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet"
	kconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/config"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/proxy"
	pconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/proxy/config"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler/factory"
	etcdconfig "github.com/coreos/etcd/config"
	"github.com/coreos/etcd/etcd"
	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/golang/glog"
	cadvisor "github.com/google/cadvisor/client"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/assets"
	"github.com/openshift/origin/pkg/build"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildetcd "github.com/openshift/origin/pkg/build/registry/etcd"
	"github.com/openshift/origin/pkg/build/strategy"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/github"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/deploy"
	deployregistry "github.com/openshift/origin/pkg/deploy/registry/deploy"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployetcd "github.com/openshift/origin/pkg/deploy/registry/etcd"
	imageetcd "github.com/openshift/origin/pkg/image/registry/etcd"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorymapping"
	routeetcd "github.com/openshift/origin/pkg/route/registry/etcd"
	routeregistry "github.com/openshift/origin/pkg/route/registry/route"
	"github.com/openshift/origin/pkg/template"
	"github.com/openshift/origin/pkg/version"

	// Register versioned api types
	_ "github.com/openshift/origin/pkg/config/api/v1beta1"
	_ "github.com/openshift/origin/pkg/image/api/v1beta1"
	_ "github.com/openshift/origin/pkg/route/api/v1beta1"
	_ "github.com/openshift/origin/pkg/template/api/v1beta1"
)

func NewCommandStartAllInOne(name string) *cobra.Command {
	dockerHelper := docker.NewHelper()
	cfg := &config{Docker: *dockerHelper}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch in all-in-one mode",
		Run: func(c *cobra.Command, args []string) {
			cfg.masterHost = env("OPENSHIFT_MASTER", "127.0.0.1")
			cfg.bindAddr = env("OPENSHIFT_BIND_ADDR", "127.0.0.1")
			cfg.nodeHosts = []string{"127.0.0.1"}
			cfg.networkContainerImage = env("KUBERNETES_NETWORK_CONTAINER_IMAGE", kubelet.NetworkContainerImage)

			if len(os.Getenv("OPENSHIFT_MASTER")) > 0 {
				if cfg.masterHost == cfg.bindAddr {
					cfg.nodeHosts = []string{}
					cfg.ListenAddr = cfg.masterHost + ":8080"
					glog.Infof("Starting master with cfg %v", cfg)
					cfg.startMaster()
				} else {
					glog.Infof("Starting node with cfg %v", cfg)
					cfg.startNode()
				}
			} else {
				cfg.startAllInOne()
			}
		},
	}

	flag := cmd.Flags()
	flag.StringVar(&cfg.ListenAddr, "listenAddr", "127.0.0.1:8080", "The server listen address.")
	flag.StringVar(&cfg.VolumeDir, "volumeDir", "openshift.local.volumes", "The volume storage directory.")
	flag.StringVar(&cfg.EtcdDir, "etcdDir", "openshift.local.etcd", "The etcd data directory.")

	dockerHelper.InstallFlags(flag)

	return cmd
}

// config contains all options that apply to a running command
type config struct {
	ListenAddr string
	VolumeDir  string
	EtcdDir    string
	Docker     docker.Helper

	masterHost string
	nodeHosts  []string
	bindAddr   string

	networkContainerImage string

	storageVersion string
}

// newEtcdHelper returns an EtcdHelper for the provided arguments or an error if the version
// is incorrect.
func (c *config) newEtcdHelper() (helper tools.EtcdHelper, err error) {
	client, _ := c.getEtcdClient()
	version := c.storageVersion
	if version == "" {
		version = latest.Version
	}
	interfaces, err := latest.InterfacesFor(version)
	if err != nil {
		return helper, err
	}
	return tools.EtcdHelper{client, interfaces.Codec, interfaces.ResourceVersioner}, nil
}

func (c *config) getKubeClient() *kubeclient.Client {
	kubeClient, err := kubeclient.New(&kubeclient.Config{Host: c.ListenAddr, Version: klatest.Version})
	if err != nil {
		glog.Fatalf("Unable to configure client - bad URL: %v", err)
	}
	return kubeClient
}

func (c *config) getOsClient() *osclient.Client {
	osClient, err := osclient.New(&kubeclient.Config{Host: c.ListenAddr, Version: latest.Version})
	if err != nil {
		glog.Fatalf("Unable to configure client - bad URL: %v", err)
	}
	return osClient
}

func (c *config) getEtcdClient() (*etcdclient.Client, []string) {
	etcdServers := []string{"http://" + c.masterHost + ":4001"}
	etcdClient := etcdclient.NewClient(etcdServers)

	for i := 0; ; i += 1 {
		_, err := etcdClient.Get("/", false, false)
		if err == nil || tools.IsEtcdNotFound(err) {
			break
		}
		if i > 100 {
			glog.Fatal("Could not reach etcd: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	return etcdClient, etcdServers
}

func (c *config) startAllInOne() {
	c.runEtcd()
	c.runApiserver()
	c.runAssetServer()
	c.runKubelet()
	c.runProxy()
	c.runScheduler()
	c.runReplicationController()
	c.runBuildController()
	c.runDeploymentController()

	select {}
}

func (c *config) startMaster() {
	c.runEtcd()
	c.runApiserver()
	c.runScheduler()
	c.runReplicationController()
	c.runBuildController()
	c.runDeploymentController()

	select {}
}

func (c *config) startNode() {
	c.runProxy()
	c.runKubelet()

	select {}
}

func (c *config) runApiserver() {
	minionPort := 10250
	osAddr := c.ListenAddr

	kubePrefix := "/api/v1beta1"
	kube2Prefix := "/api/v1beta2"
	osPrefix := "/osapi/v1beta1"

	kubeClient := c.getKubeClient()
	osClient := c.getOsClient()
	_, etcdServers := c.getEtcdClient()
	etcdHelper, err := c.newEtcdHelper()
	if err != nil {
		glog.Errorf("Error setting up server storage: %v", err)
	}
	ketcdHelper, err := master.NewEtcdHelper(etcdServers, klatest.Version)
	if err != nil {
		glog.Errorf("Error setting up Kubernetes server storage: %v", err)
	}

	buildRegistry := buildetcd.New(etcdHelper)
	imageRegistry := imageetcd.New(etcdHelper)
	deployEtcd := deployetcd.New(etcdHelper)
	routeEtcd := routeetcd.New(etcdHelper)

	// initialize OpenShift API
	storage := map[string]apiserver.RESTStorage{
		"builds":                  buildregistry.NewREST(buildRegistry),
		"buildConfigs":            buildconfigregistry.NewREST(buildRegistry),
		"images":                  image.NewREST(imageRegistry),
		"imageRepositories":       imagerepository.NewREST(imageRegistry),
		"imageRepositoryMappings": imagerepositorymapping.NewREST(imageRegistry, imageRegistry),
		"deployments":             deployregistry.NewREST(deployEtcd),
		"deploymentConfigs":       deployconfigregistry.NewREST(deployEtcd),
		"templateConfigs":         template.NewStorage(),
		"routes":                  routeregistry.NewREST(routeEtcd),
	}

	osMux := http.NewServeMux()

	// initialize webhooks
	whPrefix := osPrefix + "/buildConfigHooks/"
	osMux.Handle(whPrefix, http.StripPrefix(whPrefix,
		webhook.NewController(osClient, map[string]webhook.Plugin{
			"github": github.New(),
		})))

	// initialize Kubernetes API
	podInfoGetter := &kubeclient.HTTPPodInfoGetter{
		Client: http.DefaultClient,
		Port:   uint(minionPort),
	}
	masterConfig := &master.Config{
		Client:             kubeClient,
		EtcdHelper:         ketcdHelper,
		HealthCheckMinions: true,
		Minions:            c.nodeHosts,
		PodInfoGetter:      podInfoGetter,
	}
	m := master.New(masterConfig)

	apiserver.NewAPIGroup(m.API_v1beta1()).InstallREST(osMux, kubePrefix)
	apiserver.NewAPIGroup(m.API_v1beta2()).InstallREST(osMux, kube2Prefix)
	apiserver.NewAPIGroup(storage, v1beta1.Codec, osPrefix, latest.SelfLinker).InstallREST(osMux, osPrefix)
	apiserver.InstallSupport(osMux)

	osApi := &http.Server{
		Addr:           osAddr,
		Handler:        apiserver.RecoverPanics(osMux),
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		glog.Infof("Started Kubernetes API at http://%s%s", osAddr, kubePrefix)
		glog.Infof("Started Kubernetes API at http://%s%s", osAddr, kube2Prefix)
		glog.Infof("Started OpenShift API at http://%s%s", osAddr, osPrefix)
		glog.Fatal(osApi.ListenAndServe())
	}, 0)
}

func (c *config) runAssetServer() {
	// TODO prefix should be able to be overridden at the command line
	// move this out to a helper / config
	prefix := fmt.Sprintf("/assets/%s/", version.Get().GitCommit)
	// TODO configurable listen address
	addr := c.masterHost + ":8091"
	// TODO - For now redirect requests to the root to the commit-based index.html URL
	// Next step is to have the root page served without redirecting.  May require build
	// changes or altering index.html while serving.
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		urlStr := fmt.Sprintf("%sindex.html", prefix)
		http.Redirect(w, req, urlStr, http.StatusTemporaryRedirect)
	}))

	mux.Handle(prefix, http.StripPrefix(prefix, http.FileServer(
		&assetfs.AssetFS{assets.Asset, assets.AssetDir, ""})))

	osAssets := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		glog.Infof("Started OpenShift static asset server at http://%s", addr)
		glog.Fatal(osAssets.ListenAndServe())
	}, 0)
}

func (c *config) runKubelet() {
	rootDirectory := path.Clean(c.VolumeDir)
	minionHost := c.bindAddr
	minionPort := 10250

	cadvisorClient, err := cadvisor.NewClient("http://" + c.masterHost + ":4194")
	if err != nil {
		glog.Errorf("Error on creating cadvisor client: %v", err)
	}

	dockerClient, dockerAddr := c.Docker.GetClientOrExit()
	if err := dockerClient.Ping(); err != nil {
		glog.Errorf("WARNING: Docker could not be reached at %s.  Docker must be installed and running to start containers.\n%v", dockerAddr, err)
	} else {
		glog.Infof("Connecting to Docker at %s", dockerAddr)
	}

	etcdClient, _ := c.getEtcdClient()

	// initialize Kubelet
	if _, err := os.Stat(rootDirectory); os.IsNotExist(err) {
		if mkdirErr := os.MkdirAll(rootDirectory, 0750); mkdirErr != nil {
			glog.Fatalf("Couldn't create kubelet volume root directory '%s': %s", rootDirectory, mkdirErr)
		}
	}

	cfg := kconfig.NewPodConfig(kconfig.PodConfigNotificationSnapshotAndUpdates)
	kconfig.NewSourceEtcd(kconfig.EtcdKeyForHost(minionHost), etcdClient, cfg.Channel("etcd"))
	k := kubelet.NewMainKubelet(
		minionHost,
		dockerClient,
		cadvisorClient,
		etcdClient,
		rootDirectory,
		c.networkContainerImage,
		30*time.Second,
		0.0,
		10)
	go util.Forever(func() { k.Run(cfg.Updates()) }, 0)
	go util.Forever(func() {
		kubelet.ListenAndServeKubeletServer(k, cfg.Channel("http"), minionHost, uint(minionPort))
	}, 0)
}

func (c *config) runProxy() {
	etcdClient, _ := c.getEtcdClient()

	// initialize kube proxy
	serviceConfig := pconfig.NewServiceConfig()
	endpointsConfig := pconfig.NewEndpointsConfig()
	pconfig.NewConfigSourceEtcd(etcdClient,
		serviceConfig.Channel("etcd"),
		endpointsConfig.Channel("etcd"))
	loadBalancer := proxy.NewLoadBalancerRR()
	proxier := proxy.NewProxier(loadBalancer, c.bindAddr)
	serviceConfig.RegisterHandler(proxier)
	endpointsConfig.RegisterHandler(loadBalancer)
	glog.Infof("Started Kubernetes Proxy")
}

func (c *config) runReplicationController() {
	kubeClient := c.getKubeClient()

	// initialize replication manager
	controllerManager := controller.NewReplicationManager(kubeClient)
	controllerManager.Run(10 * time.Second)
	glog.Infof("Started Kubernetes Replication Manager")
}

func (c *config) runEtcd() {
	etcdAddr := c.bindAddr + ":4001"
	etcdConfig := etcdconfig.New()
	etcdConfig.Addr = etcdAddr
	etcdConfig.BindAddr = etcdAddr
	etcdConfig.DataDir = c.EtcdDir
	etcdConfig.Name = "openshift.local"

	// initialize etcd
	etcdServer := etcd.New(etcdConfig)
	go util.Forever(func() {
		glog.Infof("Started etcd at http://%s", etcdAddr)
		etcdServer.Run()
	}, 0)
}

func (c *config) runScheduler() {
	kubeClient := c.getKubeClient()

	// initialize scheduler
	configFactory := &factory.ConfigFactory{Client: kubeClient}
	config := configFactory.Create()
	s := scheduler.New(config)
	s.Run()
	glog.Infof("Started Kubernetes Scheduler")
}

func (c *config) runBuildController() {
	kubeClient := c.getKubeClient()
	osClient := c.getOsClient()

	// initialize build controller
	dockerBuilderImage := env("OPENSHIFT_DOCKER_BUILDER_IMAGE", "openshift/docker-builder")
	stiBuilderImage := env("OPENSHIFT_STI_BUILDER_IMAGE", "openshift/sti-builder")

	buildStrategies := map[buildapi.BuildType]build.BuildJobStrategy{
		buildapi.DockerBuildType: strategy.NewDockerBuildStrategy(dockerBuilderImage),
		buildapi.STIBuildType:    strategy.NewSTIBuildStrategy(stiBuilderImage, strategy.STITempDirectoryCreator),
	}

	buildController := build.NewBuildController(kubeClient, osClient, buildStrategies, 1200)
	buildController.Run(10 * time.Second)
}

func (c *config) runDeploymentController() {
	env := []api.EnvVar{
		api.EnvVar{Name: "KUBERNETES_MASTER", Value: "http://" + c.ListenAddr},
	}
	kubeClient := c.getKubeClient()
	osClient := c.getOsClient()

	deployController := deploy.NewDeploymentController(kubeClient, osClient, env)
	deployController.Run(10 * time.Second)
}

func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	} else {
		return val
	}
}
