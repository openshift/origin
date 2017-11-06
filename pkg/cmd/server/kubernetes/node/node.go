package node

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"

	dockertypes "github.com/docker/engine-api/types"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/kubelet/cadvisor"
	cadvisortesting "k8s.io/kubernetes/pkg/kubelet/cadvisor/testing"
	"k8s.io/kubernetes/pkg/kubelet/cm"
	dockertools "k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	"k8s.io/kubernetes/pkg/volume"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/volume/emptydir"
)

const minimumDockerAPIVersionWithPullByID = "1.22"

// EnsureKubeletAccess performs a number of test operations that the Kubelet requires to properly function.
// All errors here are fatal.
func (c *NodeConfig) EnsureKubeletAccess() {
	if _, err := os.Stat("/var/lib/docker"); os.IsPermission(err) {
		c.HandleDockerError("Unable to view the /var/lib/docker directory - are you running as root?")
	}
	if c.Containerized {
		if _, err := os.Stat("/rootfs"); os.IsPermission(err) || os.IsNotExist(err) {
			glog.Fatal("error: Running in containerized mode, but cannot find the /rootfs directory - be sure to mount the host filesystem at /rootfs (read-only) in the container.")
		}
		if !sameFileStat(true, "/rootfs/sys", "/sys") {
			glog.Fatal("error: Running in containerized mode, but the /sys directory in the container does not appear to match the host /sys directory - be sure to mount /sys into the container.")
		}
		if !sameFileStat(true, "/rootfs/var/run", "/var/run") {
			glog.Fatal("error: Running in containerized mode, but the /var/run directory in the container does not appear to match the host /var/run directory - be sure to mount /var/run (read-write) into the container.")
		}
	}
	// TODO: check whether we can mount disks (for volumes)
	// TODO: check things cAdvisor needs to properly function
	// TODO: test a cGroup move?
}

// sameFileStat checks whether the provided paths are the same file, to verify that a user has correctly
// mounted those binaries
func sameFileStat(requireMode bool, src, dst string) bool {
	srcStat, err := os.Stat(src)
	if err != nil {
		glog.V(4).Infof("Unable to stat %q: %v", src, err)
		return false
	}
	dstStat, err := os.Stat(dst)
	if err != nil {
		glog.V(4).Infof("Unable to stat %q: %v", dst, err)
		return false
	}
	if requireMode && srcStat.Mode() != dstStat.Mode() {
		glog.V(4).Infof("Mode mismatch between %q (%s) and %q (%s)", src, srcStat.Mode(), dst, dstStat.Mode())
		return false
	}
	if !os.SameFile(srcStat, dstStat) {
		glog.V(4).Infof("inode and device mismatch between %q (%s) and %q (%s)", src, srcStat, dst, dstStat)
		return false
	}
	return true
}

// EnsureDocker attempts to connect to the Docker daemon defined by the helper,
// and if it is unable to it will print a warning.
func (c *NodeConfig) EnsureDocker(docker *dockerutil.Helper) {
	if c.KubeletServer.ContainerRuntime != "docker" {
		return
	}
	dockerClient, dockerAddr, err := docker.GetKubeClient(c.KubeletServer.RuntimeRequestTimeout.Duration, c.KubeletServer.ImagePullProgressDeadline.Duration)
	if err != nil {
		c.HandleDockerError(fmt.Sprintf("Unable to create a Docker client for %s - Docker must be installed and running to start containers.\n%v", dockerAddr, err))
		return
	}
	if url, err := url.Parse(dockerAddr); err == nil && url.Scheme == "unix" && len(url.Path) > 0 {
		s, err := os.Stat(url.Path)
		switch {
		case os.IsNotExist(err):
			c.HandleDockerError(fmt.Sprintf("No Docker socket found at %s. Have you started the Docker daemon?", url.Path))
			return
		case os.IsPermission(err):
			c.HandleDockerError(fmt.Sprintf("You do not have permission to connect to the Docker daemon (via %s). This process requires running as the root user.", url.Path))
			return
		case err == nil && s.IsDir():
			c.HandleDockerError(fmt.Sprintf("The Docker socket at %s is a directory instead of a unix socket - check that you have configured your connection to the Docker daemon properly.", url.Path))
			return
		}
	}
	if err := dockerClient.Ping(); err != nil {
		c.HandleDockerError(fmt.Sprintf("Docker could not be reached at %s.  Docker must be installed and running to start containers.\n%v", dockerAddr, err))
		return
	}

	glog.Infof("Connecting to Docker at %s", dockerAddr)

	version, err := dockerClient.Version()
	if err != nil {
		c.HandleDockerError(fmt.Sprintf("Unable to check for Docker server version.\n%v", err))
		return
	}

	serverVersion, err := dockerclient.NewAPIVersion(version.APIVersion)
	if err != nil {
		c.HandleDockerError(fmt.Sprintf("Unable to determine Docker server version from %q.\n%v", version.APIVersion, err))
		return
	}

	minimumPullByIDVersion, err := dockerclient.NewAPIVersion(minimumDockerAPIVersionWithPullByID)
	if err != nil {
		c.HandleDockerError(fmt.Sprintf("Unable to check for Docker server version.\n%v", err))
		return
	}

	if serverVersion.LessThan(minimumPullByIDVersion) {
		c.HandleDockerError(fmt.Sprintf("Docker 1.6 or later (server API version %s or later) required.", minimumDockerAPIVersionWithPullByID))
		return
	}

	c.DockerClient = dockerClient
}

// HandleDockerError handles an an error from the docker daemon
func (c *NodeConfig) HandleDockerError(message string) {
	if !c.AllowDisabledDocker {
		glog.Fatalf("error: %s", message)
	}
	glog.Errorf("WARNING: %s", message)
	c.DockerClient = &dockertools.FakeDockerClient{
		VersionInfo: dockertypes.Version{
			APIVersion: minimumDockerAPIVersionWithPullByID,
			Version:    "1.13",
		},
		Information: dockertypes.Info{
			CgroupDriver: "systemd",
		},
	}
}

// EnsureVolumeDir attempts to convert the provided volume directory argument to
// an absolute path and create the directory if it does not exist. Will exit if
// an error is encountered.
func (c *NodeConfig) EnsureVolumeDir() {
	if volumeDir, err := c.initializeVolumeDir(c.VolumeDir); err != nil {
		glog.Fatal(err)
	} else {
		c.VolumeDir = volumeDir
	}
}

func (c *NodeConfig) initializeVolumeDir(path string) (string, error) {
	rootDirectory, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("Error converting volume directory to an absolute path: %v", err)
	}

	if _, err := os.Stat(rootDirectory); os.IsNotExist(err) {
		if err := os.MkdirAll(rootDirectory, 0750); err != nil {
			return "", fmt.Errorf("Couldn't create kubelet volume root directory '%s': %s", rootDirectory, err)
		}
	}
	return rootDirectory, nil
}

// EnsureLocalQuota checks if the node config specifies a local storage
// perFSGroup quota, and if so will test that the volumeDirectory is on a
// filesystem suitable for quota enforcement. If checks pass the k8s emptyDir
// volume plugin will be replaced with a wrapper version which adds quota
// functionality.
func (c *NodeConfig) EnsureLocalQuota(nodeConfig configapi.NodeConfig) {
	if nodeConfig.VolumeConfig.LocalQuota.PerFSGroup == nil {
		return
	}
	glog.V(4).Info("Replacing empty-dir volume plugin with quota wrapper")
	wrappedEmptyDirPlugin := false

	quotaApplicator, err := emptydir.NewQuotaApplicator(nodeConfig.VolumeDirectory)
	if err != nil {
		glog.Fatalf("Could not set up local quota, %s", err)
	}

	// Create a volume spec with emptyDir we can use to search for the
	// emptyDir plugin with CanSupport:
	emptyDirSpec := &volume.Spec{
		Volume: &kapiv1.Volume{
			VolumeSource: kapiv1.VolumeSource{
				EmptyDir: &kapiv1.EmptyDirVolumeSource{},
			},
		},
	}

	for idx, plugin := range c.KubeletDeps.VolumePlugins {
		// Can't really do type checking or use a constant here as they are not exported:
		if plugin.CanSupport(emptyDirSpec) {
			wrapper := emptydir.EmptyDirQuotaPlugin{
				VolumePlugin:    plugin,
				Quota:           *nodeConfig.VolumeConfig.LocalQuota.PerFSGroup,
				QuotaApplicator: quotaApplicator,
			}
			c.KubeletDeps.VolumePlugins[idx] = &wrapper
			wrappedEmptyDirPlugin = true
		}
	}
	// Because we can't look for the k8s emptyDir plugin by any means that would
	// survive a refactor, error out if we couldn't find it:
	if !wrappedEmptyDirPlugin {
		glog.Fatal(errors.New("No plugin handling EmptyDir was found, unable to apply local quotas"))
	}
}

// RunKubelet starts the Kubelet.
func (c *NodeConfig) RunKubelet() {
	var clusterDNS net.IP
	if len(c.KubeletServer.ClusterDNS) == 0 {
		if service, err := c.DNSClient.Core().Services(metav1.NamespaceDefault).Get("kubernetes", metav1.GetOptions{}); err == nil {
			if includesServicePort(service.Spec.Ports, 53, "dns") {
				// Use master service if service includes "dns" port 53.
				clusterDNS = net.ParseIP(service.Spec.ClusterIP)
			}
		}
	}
	if clusterDNS == nil {
		if endpoint, err := c.DNSClient.Core().Endpoints(metav1.NamespaceDefault).Get("kubernetes", metav1.GetOptions{}); err == nil {
			if endpointIP, ok := firstEndpointIPWithNamedPort(endpoint, 53, "dns"); ok {
				// Use first endpoint if endpoint includes "dns" port 53.
				clusterDNS = net.ParseIP(endpointIP)
			} else if endpointIP, ok := firstEndpointIP(endpoint, 53); ok {
				// Test and use first endpoint if endpoint includes any port 53.
				if err := cmdutil.WaitForSuccessfulDial(false, "tcp", fmt.Sprintf("%s:%d", endpointIP, 53), 50*time.Millisecond, 0, 2); err == nil {
					clusterDNS = net.ParseIP(endpointIP)
				}
			}
		}
	}
	if clusterDNS != nil && !clusterDNS.IsUnspecified() {
		c.KubeletServer.ClusterDNS = []string{clusterDNS.String()}
	}

	// only set when ContainerRuntime == "docker"
	c.KubeletDeps.DockerClient = c.DockerClient
	// updated by NodeConfig.EnsureVolumeDir
	c.KubeletServer.RootDirectory = c.VolumeDir

	// hook for overriding the cadvisor interface for integration tests
	c.KubeletDeps.CAdvisorInterface = defaultCadvisorInterface
	// hook for overriding the container manager interface for integration tests
	c.KubeletDeps.ContainerManager = defaultContainerManagerInterface

	go func() {
		glog.Fatal(kubeletapp.Run(c.KubeletServer, c.KubeletDeps))
	}()
}

// defaultCadvisorInterface holds the overridden default interface
// exists only to allow stubbing integration tests, should always be nil in production
var defaultCadvisorInterface cadvisor.Interface = nil

// SetFakeCadvisorInterfaceForIntegrationTest sets a fake cadvisor implementation to allow the node to run in integration tests
func SetFakeCadvisorInterfaceForIntegrationTest() {
	defaultCadvisorInterface = &cadvisortesting.Fake{}
}

// defaultContainerManagerInterface holds the overridden default interface
// exists only to allow stubbing integration tests, should always be nil in production
var defaultContainerManagerInterface cm.ContainerManager = nil

// SetFakeContainerManagerInterfaceForIntegrationTest sets a fake container manager implementation to allow the node to run in integration tests
func SetFakeContainerManagerInterfaceForIntegrationTest() {
	defaultContainerManagerInterface = cm.NewStubContainerManager()
}

// TODO: more generic location
func includesServicePort(ports []kapiv1.ServicePort, port int, portName string) bool {
	for _, p := range ports {
		if p.Port == int32(port) && p.Name == portName {
			return true
		}
	}
	return false
}

// TODO: more generic location
func includesEndpointPort(ports []kapiv1.EndpointPort, port int) bool {
	for _, p := range ports {
		if p.Port == int32(port) {
			return true
		}
	}
	return false
}

// TODO: more generic location
func firstEndpointIP(endpoints *kapiv1.Endpoints, port int) (string, bool) {
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
func firstEndpointIPWithNamedPort(endpoints *kapiv1.Endpoints, port int, portName string) (string, bool) {
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
func includesNamedEndpointPort(ports []kapiv1.EndpointPort, port int, portName string) bool {
	for _, p := range ports {
		if p.Port == int32(port) && p.Name == portName {
			return true
		}
	}
	return false
}
