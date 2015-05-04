package kubernetes

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/capabilities"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/cadvisor"
	kconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/config"
	kubecontainer "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/container"
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
	"github.com/openshift/origin/pkg/kubelet/app"
	"github.com/openshift/origin/pkg/service"
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
	if err == nil {
		// TODO: use VersionInfo after the next rebase
		_, err = cadvisorInterface.MachineInfo()
	}
	if err != nil {
		glog.Errorf("WARNING: cAdvisor cannot be started: %v", err)
		cadvisorInterface = &cadvisor.Fake{}
	}

	hostNetworkCapabilities := []string{kubelet.ApiserverSource, kubelet.FileSource}
	// initialize Kubelet
	// Allow privileged containers
	// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
	capabilities.Setup(true, hostNetworkCapabilities)
	recorder := record.NewBroadcaster().NewRecorder(kapi.EventSource{Component: "kubelet", Host: c.NodeHost})

	cfg := kconfig.NewPodConfig(kconfig.PodConfigNotificationSnapshotAndUpdates, recorder)
	kconfig.NewSourceApiserver(c.Client, c.NodeHost, cfg.Channel(kubelet.ApiserverSource))
	// define manifest file source for pods, if specified
	if len(c.PodManifestPath) > 0 {
		_, err = os.Stat(c.PodManifestPath)
		if err == nil {
			glog.Infof("Adding pod manifest file/dir: %v", c.PodManifestPath)
			kconfig.NewSourceFile(c.PodManifestPath, c.NodeHost,
				time.Duration(c.PodManifestCheckIntervalSeconds)*time.Second,
				cfg.Channel(kubelet.FileSource))
		} else {
			glog.Errorf("WARNING: PodManifestPath specified is not a valid file/directory: %v", err)
		}
	}

	gcPolicy := kubelet.ContainerGCPolicy{
		MinAge:             10 * time.Second,
		MaxPerPodContainer: 5,
		MaxContainers:      100,
	}
	imageGCPolicy := kubelet.ImageGCPolicy{
		HighThresholdPercent: 90,
		LowThresholdPercent:  80,
	}

	k, err := kubelet.NewMainKubelet(
		c.NodeHost,
		c.DockerClient,
		c.Client,
		c.VolumeDir,
		c.ImageFor("pod"),
		3*time.Second,
		0.0,
		10,
		gcPolicy,
		cfg.SeenAllSources,
		c.ClusterDomain,
		clusterDNS,
		kapi.NamespaceDefault,
		app.ProbeVolumePlugins(),
		app.ProbeNetworkPlugins(),
		c.NetworkPluginName,
		5*time.Minute,
		recorder,
		cadvisorInterface,
		imageGCPolicy,
		nil,
		15*time.Second,
		"/kubelet",
		kubecontainer.RealOS{},
		"",
		"docker",
		mount.New(),
	)
	if err != nil {
		glog.Fatalf("Couldn't run kubelet: %s", err)
	}
	go util.Forever(func() { k.Run(cfg.Updates()) }, 0)

	handler := kubelet.NewServer(k, true)

	server := &http.Server{
		Addr:           c.BindAddress,
		Handler:        &handler,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		glog.Infof("Started Kubelet for node %s, server at %s, tls=%v", c.NodeHost, c.BindAddress, c.TLS)
		if clusterDNS != nil {
			glog.Infof("  Kubelet is setting %s as a DNS nameserver for domain %q", clusterDNS, c.ClusterDomain)
		}
		k.BirthCry()

		if c.TLS {
			server.TLSConfig = &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
				// RequireAndVerifyClientCert lets us limit requests to ones with a valid client certificate
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  c.ClientCAs,
			}
			glog.Fatal(server.ListenAndServeTLS(c.KubeletCertFile, c.KubeletKeyFile))
		} else {
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)
}

// RunProxy starts the proxy
func (c *NodeConfig) RunProxy() {
	// initialize kube proxy
	serviceConfig := pconfig.NewServiceConfig()
	endpointsConfig := pconfig.NewEndpointsConfig()
	pconfig.NewSourceAPI(
		c.Client.Services(kapi.NamespaceAll),
		c.Client.Endpoints(kapi.NamespaceAll),
		30*time.Second,
		serviceConfig.Channel("api"),
		endpointsConfig.Channel("api"))
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

	var proxier pconfig.ServiceConfigHandler
	proxier = proxy.NewProxier(loadBalancer, ip, iptables.New(kexec.New(), protocol))
	if proxier == nil || reflect.ValueOf(proxier).IsNil() { // explicitly declared interfaces aren't plain nil, you must reflect inside to see if it's really nil or not
		glog.Errorf("WARNING: Could not modify iptables.  iptables must be mutable by this process to use services.  Do you have root permissions?")
		proxier = &service.FailingServiceConfigProxy{}
	}
	serviceConfig.RegisterHandler(proxier)

	glog.Infof("Started Kubernetes Proxy on %s", host)
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
