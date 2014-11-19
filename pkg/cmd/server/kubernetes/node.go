package kubernetes

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet"
	kconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/config"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/proxy"
	pconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/proxy/config"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/exec"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/iptables"
	"github.com/coreos/go-etcd/etcd"
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	cadvisor "github.com/google/cadvisor/client"

	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"

	"github.com/openshift/origin/pkg/service"
)

// NodePort is the default Kubelet port for serving information about the node.
const NodePort = 10250

// NodeConfig represents the required parameters to start the OpenShift node
// through Kubernetes. All fields are required.
type NodeConfig struct {
	// The address to bind to
	BindHost string
	// The name of this node that will be used to identify the node in the master.
	// This value must match the value provided to the master on startup.
	NodeHost string
	// The host that the master can be reached at (not in use yet)
	MasterHost string
	// The directory that volumes will be stored under
	VolumeDir string

	// The image used as the Kubelet network namespace and volume container.
	NetworkContainerImage string

	// A client to connect to etcd
	EtcdClient *etcd.Client
	// A client to connect to Docker
	DockerClient *docker.Client
}

// EnsureDocker attempts to connect to the Docker daemon defined by the helper,
// and if it is unable to it will print a warning.
func (c *NodeConfig) EnsureDocker(docker *dockerutil.Helper) {
	dockerClient, dockerAddr := docker.GetClientOrExit()
	if err := dockerClient.Ping(); err != nil {
		glog.Errorf("WARNING: Docker could not be reached at %s.  Docker must be installed and running to start containers.\n%v", dockerAddr, err)
	} else {
		glog.Infof("Connecting to Docker at %s", dockerAddr)
	}
	c.DockerClient = dockerClient
}

// EnsureVolumeDir attempts to convert the provided volume directory argument to
// an absolute path and create the directory if it does not exist. Will exit if
// an error is encountered.
func (c *NodeConfig) EnsureVolumeDir() {
	rootDirectory, err := filepath.Abs(c.VolumeDir)
	if err != nil {
		glog.Fatalf("Error converting volume directory to an absolute path: %v", err)
	}

	if _, err := os.Stat(rootDirectory); os.IsNotExist(err) {
		if mkdirErr := os.MkdirAll(rootDirectory, 0750); mkdirErr != nil {
			glog.Fatalf("Couldn't create kubelet volume root directory '%s': %s", rootDirectory, mkdirErr)
		}
	}
	c.VolumeDir = rootDirectory
}

// RunKubelet starts the Kubelet.
func (c *NodeConfig) RunKubelet() {
	// initialize Kubelet
	cfg := kconfig.NewPodConfig(kconfig.PodConfigNotificationSnapshotAndUpdates)
	kconfig.NewSourceEtcd(kconfig.EtcdKeyForHost(c.NodeHost), c.EtcdClient, cfg.Channel("etcd"))
	k := kubelet.NewMainKubelet(
		c.NodeHost,
		c.DockerClient,
		c.EtcdClient,
		c.VolumeDir,
		c.NetworkContainerImage,
		30*time.Second,
		0.0,
		10,
		0,
		5)
	// k := kubelet.NewIntegrationTestKubelet(c.NodeHost, c.VolumeDir, c.DockerClient)
	go util.Forever(func() { k.Run(cfg.Updates()) }, 0)

	// this parameter must be true, otherwise buildLogs won't work
	enableDebuggingHandlers := true
	go util.Forever(func() {
		glog.Infof("Started Kubelet for node %s, server at %s:%d", c.NodeHost, c.BindHost, NodePort)
		kubelet.ListenAndServeKubeletServer(k, cfg.Channel("http"), net.ParseIP(c.BindHost), uint(NodePort), enableDebuggingHandlers)
	}, 0)

	// this mirrors 1fc92bef53fdd1bc70f623c0693736c763cff45f
	// I don't fully understand what a cadvisor is, but it seems that we're supposed to run it separately from the rest of the kubelet
	go func() {
		defer util.HandleCrash()
		// TODO: Monitor this connection, reconnect if needed?
		glog.V(1).Infof("Trying to create cadvisor client.")
		// cAdvisor should be running on the local machine
		cadvisorClient, err := cadvisor.NewClient("http://" + c.NodeHost + ":4194")
		if err != nil {
			glog.Errorf("Error on creating cadvisor client: %v", err)
			return
		}
		glog.V(1).Infof("Successfully created cadvisor client.")
		// this binds the cadvisor to the kubelet for later references
		k.SetCadvisorClient(cadvisorClient)
	}()

}

// RunProxy starts the proxy
func (c *NodeConfig) RunProxy() {
	// initialize kube proxy
	serviceConfig := pconfig.NewServiceConfig()
	endpointsConfig := pconfig.NewEndpointsConfig()
	pconfig.NewConfigSourceEtcd(c.EtcdClient,
		serviceConfig.Channel("etcd"),
		endpointsConfig.Channel("etcd"))
	loadBalancer := proxy.NewLoadBalancerRR()
	endpointsConfig.RegisterHandler(loadBalancer)

	var proxier pconfig.ServiceConfigHandler
	proxier = proxy.NewProxier(loadBalancer, net.ParseIP(c.BindHost), iptables.New(exec.New()))
	if proxier == nil || reflect.ValueOf(proxier).IsNil() { // explicitly declared interfaces aren't plain nil, you must reflect inside to see if it's really nil or not
		glog.Errorf("WARNING: Could not modify iptables.  iptables must be mutable by this process to use services.  Do you have root permissions?")
		proxier = &service.FailingServiceConfigProxy{}
	}
	serviceConfig.RegisterHandler(proxier)

	glog.Infof("Started Kubernetes Proxy on %s", c.BindHost)
}
