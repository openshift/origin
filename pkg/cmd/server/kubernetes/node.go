package kubernetes

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	kapp "github.com/GoogleCloudPlatform/kubernetes/cmd/kubelet/app"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/cadvisor"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/dockertools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/proxy"
	pconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/proxy/config"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kexec "github.com/GoogleCloudPlatform/kubernetes/pkg/util/exec"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/iptables"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/mount"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

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
	c.DockerClient = &dockertools.FakeDockerClient{VersionInfo: dockerclient.Env{"ApiVersion=1.18"}}
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
	clusterDNS := c.ClusterDNS
	if clusterDNS == nil {
		if service, err := c.Client.Endpoints(kapi.NamespaceDefault).Get("kubernetes"); err == nil {
			if ip, ok := firstIP(service, 53); ok {
				if err := cmdutil.WaitForSuccessfulDial(false, "tcp", fmt.Sprintf("%s:%d", ip, 53), 50*time.Millisecond, 0, 2); err == nil {
					clusterDNS = net.ParseIP(ip)
				}
			}
		}
	}

	cadvisorInterface, err := cadvisor.New(4194)
	if err != nil {
		glog.Fatalf("Error instantiating cadvisor: %v", err)
	}

	hostNetworkCapabilities := []string{kubelet.ApiserverSource, kubelet.FileSource}

	imageGCPolicy := kubelet.ImageGCPolicy{
		HighThresholdPercent: 90,
		LowThresholdPercent:  80,
	}
	diskSpacePolicy := kubelet.DiskSpacePolicy{
		DockerFreeDiskMB: 256,
		RootFreeDiskMB:   256,
	}
	kubeAddress, kubePortStr, err := net.SplitHostPort(c.BindAddress)
	if err != nil {
		glog.Fatalf("Cannot parse node address: %v", err)
	}
	kubePort, err := strconv.Atoi(kubePortStr)
	if err != nil {
		glog.Fatalf("Cannot parse node port: %v", err)
	}

	var tlsOptions *kubelet.TLSOptions
	if c.TLS {
		tlsOptions = &kubelet.TLSOptions{
			Config: &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
				// RequireAndVerifyClientCert lets us limit requests to ones with a valid client certificate
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  c.ClientCAs,
			},
			CertFile: c.KubeletCertFile,
			KeyFile:  c.KubeletKeyFile,
		}
	}

	kcfg := kapp.KubeletConfig{
		Address: util.IP(net.ParseIP(kubeAddress)),
		// Allow privileged containers
		// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
		AllowPrivileged:                true,
		HostNetworkSources:             hostNetworkCapabilities,
		HostnameOverride:               c.NodeHost,
		RegisterNode:                   true,
		RootDirectory:                  c.VolumeDir,
		ConfigFile:                     c.PodManifestPath,
		ManifestURL:                    "",
		FileCheckFrequency:             time.Duration(c.PodManifestCheckIntervalSeconds) * time.Second,
		HTTPCheckFrequency:             0,
		PodInfraContainerImage:         c.ImageFor("pod"),
		SyncFrequency:                  10 * time.Second,
		RegistryPullQPS:                0.0,
		RegistryBurst:                  10,
		MinimumGCAge:                   10 * time.Second,
		MaxPerPodContainerCount:        5,
		MaxContainerCount:              100,
		ClusterDomain:                  c.ClusterDomain,
		ClusterDNS:                     util.IP(clusterDNS),
		Runonce:                        false,
		Port:                           uint(kubePort),
		ReadOnlyPort:                   0,
		CadvisorInterface:              cadvisorInterface,
		EnableServer:                   true,
		EnableDebuggingHandlers:        true,
		DockerClient:                   c.DockerClient,
		KubeClient:                     c.Client,
		MasterServiceNamespace:         kapi.NamespaceDefault,
		VolumePlugins:                  kapp.ProbeVolumePlugins(),
		NetworkPlugins:                 kapp.ProbeNetworkPlugins(),
		NetworkPluginName:              c.NetworkPluginName,
		StreamingConnectionIdleTimeout: 5 * time.Minute,
		TLSOptions:                     tlsOptions,
		ImageGCPolicy:                  imageGCPolicy,
		DiskSpacePolicy:                diskSpacePolicy,
		Cloud:                          nil,
		NodeStatusUpdateFrequency: 15 * time.Second,
		ResourceContainer:         "/kubelet",
		CgroupRoot:                "",
		ContainerRuntime:          "docker",
		Mounter:                   mount.New(),
		DockerDaemonContainer:     "",
		ConfigureCBR0:             false,
		MaxPods:                   200,
	}
	kapp.RunKubelet(&kcfg, nil)
}

// RunProxy starts the proxy
func (c *NodeConfig) RunProxy() {
	// initialize kube proxy
	serviceConfig := pconfig.NewServiceConfig()
	endpointsConfig := pconfig.NewEndpointsConfig()
	loadBalancer := proxy.NewLoadBalancerRR()
	endpointsConfig.RegisterHandler(loadBalancer)

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

	go util.Forever(func() {
		proxier, err := proxy.NewProxier(loadBalancer, ip, iptables.New(kexec.New(), protocol))
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
			c.Client.Services(kapi.NamespaceAll),
			c.Client.Endpoints(kapi.NamespaceAll),
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
