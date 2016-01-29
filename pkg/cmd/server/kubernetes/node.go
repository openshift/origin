package kubernetes

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/kubelet/cadvisor"
	"k8s.io/kubernetes/pkg/kubelet/cm"
	"k8s.io/kubernetes/pkg/kubelet/dockertools"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
	proxy "k8s.io/kubernetes/pkg/proxy/iptables"
	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/iptables"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
)

type commandExecutor interface {
	LookPath(executable string) (string, error)
	Run(command string, args ...string) error
}

type defaultCommandExecutor struct{}

func (ce defaultCommandExecutor) LookPath(executable string) (string, error) {
	return exec.LookPath(executable)
}

func (ce defaultCommandExecutor) Run(command string, args ...string) error {
	c := exec.Command(command, args...)
	return c.Run()
}

const minimumDockerAPIVersionWithPullByID = "1.18"

// EnsureDocker attempts to connect to the Docker daemon defined by the helper,
// and if it is unable to it will print a warning.
func (c *NodeConfig) EnsureDocker(docker *dockerutil.Helper) {
	dockerClient, dockerAddr := docker.GetClientOrExit()
	if err := dockerClient.Ping(); err != nil {
		c.HandleDockerError(fmt.Sprintf("Docker could not be reached at %s.  Docker must be installed and running to start containers.\n%v", dockerAddr, err))
		return
	}

	glog.Infof("Connecting to Docker at %s", dockerAddr)

	env, err := dockerClient.Version()
	if err != nil {
		c.HandleDockerError(fmt.Sprintf("Unable to check for Docker server version.\n%v", err))
		return
	}

	serverVersionString := env.Get("ApiVersion")
	serverVersion, err := dockerclient.NewAPIVersion(serverVersionString)
	if err != nil {
		c.HandleDockerError(fmt.Sprintf("Unable to determine Docker server version from %q.\n%v", serverVersionString, err))
		return
	}

	minimumPullByIDVersion, err := dockerclient.NewAPIVersion(minimumDockerAPIVersionWithPullByID)
	if err != nil {
		c.HandleDockerError(fmt.Sprintf("Unable to check for Docker server version.\n%v", err))
		return
	}

	if serverVersion.LessThan(minimumPullByIDVersion) {
		c.HandleDockerError(fmt.Sprintf("Docker 1.6 or later (server API version 1.18 or later) required."))
		return
	}

	c.DockerClient = dockerClient
}

// HandleDockerError handles an an error from the docker daemon
func (c *NodeConfig) HandleDockerError(message string) {
	if !c.AllowDisabledDocker {
		glog.Fatalf("ERROR: %s", message)
	}
	glog.Errorf("WARNING: %s", message)
	c.DockerClient = &dockertools.FakeDockerClient{VersionInfo: dockerclient.Env([]string{"ApiVersion=1.18"})}
}

// EnsureVolumeDir attempts to convert the provided volume directory argument to
// an absolute path and create the directory if it does not exist. Will exit if
// an error is encountered.
func (c *NodeConfig) EnsureVolumeDir() {
	if volumeDir, err := c.initializeVolumeDir(&defaultCommandExecutor{}, c.VolumeDir); err != nil {
		glog.Fatal(err)
	} else {
		c.VolumeDir = volumeDir
	}
}

func (c *NodeConfig) initializeVolumeDir(ce commandExecutor, path string) (string, error) {
	rootDirectory, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("Error converting volume directory to an absolute path: %v", err)
	}

	if _, err := os.Stat(rootDirectory); os.IsNotExist(err) {
		if err := os.MkdirAll(rootDirectory, 0750); err != nil {
			return "", fmt.Errorf("Couldn't create kubelet volume root directory '%s': %s", rootDirectory, err)
		}
	}
	// always try to chcon, in case the volume dir existed prior to the node starting
	if chconPath, err := ce.LookPath("chcon"); err != nil {
		glog.V(2).Infof("Couldn't locate 'chcon' to set the kubelet volume root directory SELinux context: %s", err)
	} else {
		if err := ce.Run(chconPath, "-t", "svirt_sandbox_file_t", rootDirectory); err != nil {
			glog.Warningf("Error running 'chcon' to set the kubelet volume root directory SELinux context: %s", err)
		}
	}
	return rootDirectory, nil
}

// RunKubelet starts the Kubelet.
func (c *NodeConfig) RunKubelet() {
	if c.KubeletConfig.ClusterDNS == nil {
		if service, err := c.Client.Services(kapi.NamespaceDefault).Get("kubernetes"); err == nil {
			if includesServicePort(service.Spec.Ports, 53, "dns") {
				// Use master service if service includes "dns" port 53.
				c.KubeletConfig.ClusterDNS = net.ParseIP(service.Spec.ClusterIP)
			}
		}
	}
	if c.KubeletConfig.ClusterDNS == nil {
		if endpoint, err := c.Client.Endpoints(kapi.NamespaceDefault).Get("kubernetes"); err == nil {
			if endpointIP, ok := firstEndpointIPWithNamedPort(endpoint, 53, "dns"); ok {
				// Use first endpoint if endpoint includes "dns" port 53.
				c.KubeletConfig.ClusterDNS = net.ParseIP(endpointIP)
			} else if endpointIP, ok := firstEndpointIP(endpoint, 53); ok {
				// Test and use first endpoint if endpoint includes any port 53.
				if err := cmdutil.WaitForSuccessfulDial(false, "tcp", fmt.Sprintf("%s:%d", endpointIP, 53), 50*time.Millisecond, 0, 2); err == nil {
					c.KubeletConfig.ClusterDNS = net.ParseIP(endpointIP)
				}
			}
		}
	}

	c.KubeletConfig.DockerClient = c.DockerClient
	// updated by NodeConfig.EnsureVolumeDir
	c.KubeletConfig.RootDirectory = c.VolumeDir

	// hook for overriding the cadvisor interface for integration tests
	c.KubeletConfig.CAdvisorInterface = defaultCadvisorInterface
	// hook for overriding the container manager interface for integration tests
	c.KubeletConfig.ContainerManager = defaultContainerManagerInterface

	go func() {
		glog.Fatal(kubeletapp.Run(c.KubeletServer, c.KubeletConfig))
	}()
}

// defaultCadvisorInterface holds the overridden default interface
// exists only to allow stubbing integration tests, should always be nil in production
var defaultCadvisorInterface cadvisor.Interface = nil

// SetFakeCadvisorInterfaceForIntegrationTest sets a fake cadvisor implementation to allow the node to run in integration tests
func SetFakeCadvisorInterfaceForIntegrationTest() {
	defaultCadvisorInterface = &cadvisor.Fake{}
}

// defaultContainerManagerInterface holds the overridden default interface
// exists only to allow stubbing integration tests, should always be nil in production
var defaultContainerManagerInterface cm.ContainerManager = nil

// SetFakeContainerManagerInterfaceForIntegrationTest sets a fake container manager implementation to allow the node to run in integration tests
func SetFakeContainerManagerInterfaceForIntegrationTest() {
	defaultContainerManagerInterface = cm.NewStubContainerManager()
}

func (c *NodeConfig) RunSDN() {
	if c.SDNPlugin != nil {
		if err := c.SDNPlugin.StartNode(c.MTU); err != nil {
			glog.Fatalf("SDN Node failed: %v", err)
		}
	}
}

// RunProxy starts the proxy
func (c *NodeConfig) RunProxy() {
	// initialize kube proxy
	serviceConfig := pconfig.NewServiceConfig()
	endpointsConfig := pconfig.NewEndpointsConfig()

	host, _, err := net.SplitHostPort(c.BindAddress)
	if err != nil {
		glog.Fatalf("The provided value to bind to must be an ip:port %q", c.BindAddress)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		glog.Fatalf("The provided value to bind to must be an ip:port: %q", c.BindAddress)
	}

	protocol := iptables.ProtocolIpv4
	if ip.To4() == nil {
		protocol = iptables.ProtocolIpv6
	}

	syncPeriod, err := time.ParseDuration(c.IPTablesSyncPeriod)
	if err != nil {
		glog.Fatalf("Cannot parse the provided ip-tables sync period (%s) : %v", c.IPTablesSyncPeriod, err)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(c.Client.Events(""))
	recorder := eventBroadcaster.NewRecorder(kapi.EventSource{Component: "kube-proxy", Host: c.KubeletConfig.NodeName})
	nodeRef := &kapi.ObjectReference{
		Kind: "Node",
		Name: c.KubeletConfig.NodeName,
	}

	exec := kexec.New()
	dbus := utildbus.New()
	iptables := iptables.New(exec, dbus, protocol)
	proxier, err := proxy.NewProxier(iptables, exec, syncPeriod, false)
	if err != nil {
		// This should be fatal, but that would break the integration tests
		glog.Warningf("WARNING: Could not initialize Kubernetes Proxy. You must run this process as root to use the service proxy: %v", err)
		return
	}
	iptables.AddReloadFunc(proxier.Sync)

	pconfig.NewSourceAPI(
		c.Client,
		10*time.Minute,
		serviceConfig.Channel("api"),
		endpointsConfig.Channel("api"))

	serviceConfig.RegisterHandler(proxier)
	if c.FilteringEndpointsHandler == nil {
		endpointsConfig.RegisterHandler(proxier)
	} else {
		c.FilteringEndpointsHandler.SetBaseEndpointsHandler(proxier)
		endpointsConfig.RegisterHandler(c.FilteringEndpointsHandler)
	}
	recorder.Eventf(nodeRef, kapi.EventTypeNormal, "Starting", "Starting kube-proxy.")
	glog.Infof("Started Kubernetes Proxy on %s", host)
}

// TODO: more generic location
func includesServicePort(ports []kapi.ServicePort, port int, portName string) bool {
	for _, p := range ports {
		if p.Port == port && p.Name == portName {
			return true
		}
	}
	return false
}

// TODO: more generic location
func includesEndpointPort(ports []kapi.EndpointPort, port int) bool {
	for _, p := range ports {
		if p.Port == port {
			return true
		}
	}
	return false
}

// TODO: more generic location
func firstEndpointIP(endpoints *kapi.Endpoints, port int) (string, bool) {
	for _, s := range endpoints.Subsets {
		if !includesEndpointPort(s.Ports, port) {
			continue
		}
		for _, a := range s.Addresses {
			return a.IP, true
		}
	}
	return "", false
}

// TODO: more generic location
func firstEndpointIPWithNamedPort(endpoints *kapi.Endpoints, port int, portName string) (string, bool) {
	for _, s := range endpoints.Subsets {
		if !includesNamedEndpointPort(s.Ports, port, portName) {
			continue
		}
		for _, a := range s.Addresses {
			return a.IP, true
		}
	}
	return "", false
}

// TODO: more generic location
func includesNamedEndpointPort(ports []kapi.EndpointPort, port int, portName string) bool {
	for _, p := range ports {
		if p.Port == port && p.Name == portName {
			return true
		}
	}
	return false
}
