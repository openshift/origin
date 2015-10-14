package kubernetes

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubelet/cadvisor"
	"k8s.io/kubernetes/pkg/kubelet/dockertools"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
	proxy "k8s.io/kubernetes/pkg/proxy/userspace"
	"k8s.io/kubernetes/pkg/util"
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
		if chconPath, err := ce.LookPath("chcon"); err != nil {
			glog.V(2).Infof("Couldn't locate 'chcon' to set the kubelet volume root directory SELinux context: %s", err)
		} else {
			if err := ce.Run(chconPath, "-t", "svirt_sandbox_file_t", rootDirectory); err != nil {
				glog.Warningf("Error running 'chcon' to set the kubelet volume root directory SELinux context: %s", err)
			}
		}
	}
	return rootDirectory, nil
}

// RunKubelet starts the Kubelet.
func (c *NodeConfig) RunKubelet() {
	// TODO: clean this up and make it more formal (service named 'dns'?). Use multiple ports.
	clusterDNS := c.KubeletConfig.ClusterDNS
	if clusterDNS == nil {
		if service, err := c.Client.Endpoints(kapi.NamespaceDefault).Get("kubernetes"); err == nil {
			if ip, ok := firstIP(service, 53); ok {
				if err := cmdutil.WaitForSuccessfulDial(false, "tcp", fmt.Sprintf("%s:%d", ip, 53), 50*time.Millisecond, 0, 2); err == nil {
					c.KubeletConfig.ClusterDNS = net.ParseIP(ip)
				}
			}
		}
	}
	c.KubeletConfig.DockerClient = c.DockerClient
	// updated by NodeConfig.EnsureVolumeDir
	c.KubeletConfig.RootDirectory = c.VolumeDir

	// hook for overriding the cadvisor interface for integration tests
	c.KubeletConfig.CadvisorInterface = defaultCadvisorInterface

	go func() {
		glog.Fatal(c.KubeletServer.Run(c.KubeletConfig))
	}()
}

// defaultCadvisorInterface holds the overridden default interface
// exists only to allow stubbing integration tests, should always be nil in production
var defaultCadvisorInterface cadvisor.Interface = nil

// SetFakeCadvisorInterfaceForIntegrationTest sets a fake cadvisor implementation to allow the node to run in integration tests
func SetFakeCadvisorInterfaceForIntegrationTest() {
	defaultCadvisorInterface = &cadvisor.Fake{}
}

type FilteringEndpointsConfigHandler interface {
	pconfig.EndpointsConfigHandler
	SetBaseEndpointsHandler(base pconfig.EndpointsConfigHandler)
}

// RunProxy starts the proxy
func (c *NodeConfig) RunProxy(endpointsFilterer FilteringEndpointsConfigHandler) {
	// initialize kube proxy
	serviceConfig := pconfig.NewServiceConfig()
	endpointsConfig := pconfig.NewEndpointsConfig()
	loadBalancer := proxy.NewLoadBalancerRR()
	if endpointsFilterer == nil {
		endpointsConfig.RegisterHandler(loadBalancer)
	} else {
		endpointsFilterer.SetBaseEndpointsHandler(loadBalancer)
		endpointsConfig.RegisterHandler(endpointsFilterer)
	}

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

	go util.Forever(func() {
		proxier, err := proxy.NewProxier(loadBalancer, ip, iptables.New(kexec.New(), protocol), util.PortRange{}, syncPeriod)
		if err != nil {
			switch {
			// conflicting use of iptables, retry
			case proxy.IsProxyLocked(err):
				glog.Errorf("Unable to start proxy, will retry: %v", err)
				return
			// on a system without iptables
			case strings.Contains(err.Error(), "executable file not found in path"):
				glog.V(4).Infof("kube-proxy initialization error: %v", err)
				glog.Warningf("WARNING: Could not find the iptables command. The service proxy requires iptables and will be disabled.")
			case err == proxy.ErrProxyOnLocalhost:
				glog.Warningf("WARNING: The service proxy cannot bind to localhost and will be disabled.")
			case strings.Contains(err.Error(), "you must be root"):
				glog.Warningf("WARNING: Could not modify iptables. You must run this process as root to use the service proxy.")
			default:
				glog.Warningf("WARNING: Could not modify iptables. You must run this process as root to use the service proxy: %v", err)
			}
			select {}
		}

		pconfig.NewSourceAPI(
			c.Client,
			30*time.Second,
			serviceConfig.Channel("api"),
			endpointsConfig.Channel("api"))

		serviceConfig.RegisterHandler(proxier)
		glog.Infof("Started Kubernetes Proxy on %s", host)
		select {}
	}, 5*time.Second)
}

// TODO: more generic location
func includesPort(ports []kapi.EndpointPort, port int) bool {
	for _, p := range ports {
		if p.Port == port {
			return true
		}
	}
	return false
}

// TODO: more generic location
func firstIP(endpoints *kapi.Endpoints, port int) (string, bool) {
	for _, s := range endpoints.Subsets {
		if !includesPort(s.Ports, port) {
			continue
		}
		for _, a := range s.Addresses {
			return a.IP, true
		}
	}
	return "", false
}
