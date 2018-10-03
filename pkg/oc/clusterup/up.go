package clusterup

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/versions"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	aggregatorinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/errors"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/host"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/openshift"
	dockerutil "github.com/openshift/origin/pkg/oc/clusterup/docker/util"
	"github.com/openshift/origin/pkg/version"
)

const (
	// CmdUpRecommendedName is the recommended command name
	CmdUpRecommendedName = "up"
	dockerAPIVersion122  = "1.22"
)

var (
	cmdUpLong = templates.LongDesc(`
		Starts an OpenShift cluster using Docker containers, provisioning a registry, router,
		initial templates, and a default project.

		This command will attempt to use an existing connection to a Docker daemon. Before running
		the command, ensure that you can execute docker commands successfully (i.e. 'docker ps').

		By default, the OpenShift cluster will be setup to use a routing suffix that ends in nip.io.
		This is to allow dynamic host names to be created for routes.

		A public hostname can also be specified for the server with the --public-hostname flag.`)

	cmdUpExample = templates.Examples(`
	  # Start OpenShift using a specific public host name
	  %[1]s --public-hostname=my.address.example.com`)
)

type ClusterUpConfig struct {
	ImageTemplate variable.ImageTemplate
	ImageTag      string
	ForcePull     bool

	// BaseTempDir is the directory to use as the root for temp directories
	// This allows us to bundle all of the cluster-up directories in one spot for easier cleanup and ensures we aren't
	// doing crazy thing like dirtying /var on the host (that does weird stuff)
	BaseDir           string
	SpecifiedBaseDir  bool
	HostName          string
	UseExistingConfig bool
	ServerLogLevel    int

	HostVolumesDir           string
	HostConfigDir            string
	HostDataDir              string
	UsePorts                 []int
	DNSPort                  int
	ServerIP                 string
	AdditionalIPs            []string
	PublicHostname           string
	HostPersistentVolumesDir string

	dockerClient        dockerutil.Interface
	dockerHelper        *dockerutil.Helper
	hostHelper          *host.HostHelper
	openshiftHelper     *openshift.Helper
	command             *cobra.Command
	defaultClientConfig clientcmdapi.Config

	usingDefaultImages         bool
	usingDefaultOpenShiftImage bool

	pullPolicy string

	genericclioptions.IOStreams
}

func NewClusterUpConfig(streams genericclioptions.IOStreams) *ClusterUpConfig {
	return &ClusterUpConfig{
		UsePorts: openshift.BasePorts,
		DNSPort:  openshift.DefaultDNSPort,

		ImageTemplate: variable.NewDefaultImageTemplate(),

		IOStreams: streams,
	}

}

// NewCmdUp creates a command that starts OpenShift on Docker with reasonable defaults
func NewCmdUp(name, fullName string, f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	config := NewClusterUpConfig(streams)
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Start OpenShift on Docker with reasonable defaults",
		Long:    cmdUpLong,
		Example: fmt.Sprintf(cmdUpExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			var err error

			if err = config.Complete(f, c); err != nil {
				PrintError(err, streams.ErrOut)
				os.Exit(1)
			}
			if err = config.Validate(); err != nil {
				PrintError(err, streams.ErrOut)
				os.Exit(1)
			}
			if err = config.Check(); err != nil {
				PrintError(err, streams.ErrOut)
				os.Exit(1)
			}
			if err := config.Start(); err != nil {
				PrintError(err, streams.ErrOut)
				os.Exit(1)
			}
		},
	}
	config.Bind(cmd.Flags())
	return cmd
}

func (c *ClusterUpConfig) Bind(flags *pflag.FlagSet) {
	flags.StringVar(&c.ImageTag, "tag", "", "Specify an explicit version for OpenShift images")
	flags.BoolVar(&c.ForcePull, "force-pull", false, "Force to pull all required images")
	flags.MarkHidden("tag")
	flags.MarkHidden("force-pull")
	flags.StringVar(&c.ImageTemplate.Format, "image", c.ImageTemplate.Format, "Specify the images to use for OpenShift")
	flags.StringVar(&c.PublicHostname, "public-hostname", "", "Public hostname for OpenShift cluster")
	flags.StringVar(&c.BaseDir, "base-dir", c.BaseDir, "Directory on Docker host for cluster up configuration")
	flags.IntVar(&c.ServerLogLevel, "server-loglevel", 0, "Log level for OpenShift server")
	flags.MarkHidden("kube-only")
}

func (c *ClusterUpConfig) Complete(f genericclioptions.RESTClientGetter, cmd *cobra.Command) error {
	// TODO: remove this when we move to container/apply based component installation
	aggregatorinstall.Install(legacyscheme.Scheme)

	// Set the ImagePullPolicy field in static pods and components based in whether users specified
	// the --tag flag or not.
	c.pullPolicy = "Always"
	if len(c.ImageTag) > 0 {
		c.pullPolicy = "IfNotPresent"
	}
	glog.V(5).Infof("Using %q as default image pull policy", c.pullPolicy)

	// Get the default client config for login
	var err error
	c.defaultClientConfig, err = f.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		c.defaultClientConfig = *clientcmdapi.NewConfig()
	}

	c.command = cmd

	c.ImageTemplate.Format = variable.Expand(c.ImageTemplate.Format, func(s string) (string, bool) {
		if s == "version" {
			if len(c.ImageTag) == 0 {
				return strings.TrimRight("v"+version.Get().Major+"."+version.Get().Minor, "+"), true
			}
			return c.ImageTag, true
		}
		return "", false
	}, variable.Identity)

	if len(c.BaseDir) == 0 {
		c.SpecifiedBaseDir = false
		c.BaseDir = "openshift.local.clusterup"
	}
	if !path.IsAbs(c.BaseDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		absHostDir, err := cmdutil.MakeAbs(c.BaseDir, cwd)
		if err != nil {
			return err
		}
		c.BaseDir = absHostDir
	}

	if _, err := os.Stat(c.BaseDir); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(c.BaseDir, os.ModePerm); err != nil {
			return fmt.Errorf("unable to create base directory %q: %v", c.BaseDir, err)
		}
	}

	// Get a Docker client.
	// If a Docker machine was specified, make sure that the machine is running.
	// Otherwise, use environment variables.
	c.printProgress("Getting a Docker client")
	client, err := dockerutil.GetDockerClient()
	if err != nil {
		return err
	}
	c.dockerClient = client

	c.printProgress(fmt.Sprintf("Pulling all required images"))
	if err := OpenShiftImages.EnsurePulled(c.Docker(), c.ImageTemplate, c.ForcePull); err != nil {
		return err
	}

	if err := os.MkdirAll(c.BaseDir, 0755); err != nil {
		return err
	}

	c.HostVolumesDir = path.Join(c.BaseDir, "openshift.local.volumes")
	if err := os.MkdirAll(c.HostVolumesDir, 0755); err != nil {
		return err
	}

	c.HostPersistentVolumesDir = path.Join(c.BaseDir, "openshift.local.pv")
	if err := os.MkdirAll(c.HostPersistentVolumesDir, 0755); err != nil {
		return err
	}

	c.HostDataDir = path.Join(c.BaseDir, "etcd")
	if err := os.MkdirAll(filepath.Join(c.HostDataDir, "data"), 0755); err != nil {
		return err
	}

	// Determine an IP to use for OpenShift.
	// The result is that c.ServerIP will be populated with
	// the IP that will be used on the client configuration file.
	// The c.ServerIP will be set to a specific IP when:
	// 1 - DOCKER_HOST is populated with a particular tcp:// type of address
	// 2 - a docker-machine has been specified
	// 3 - 127.0.0.1 is not working and an alternate IP has been found
	// Otherwise, the default c.ServerIP will be 127.0.0.1 which is what
	// will get stored in the client's config file. The reason for this is that
	// the client config will not depend on the machine's current IP address which
	// could change over time.
	//
	// c.AdditionalIPs will be populated with additional IPs that should be
	// included in the server's certificate. These include any IPs that are currently
	// assigned to the Docker host (hostname -I)
	// Each IP is tested to ensure that it can be accessed from the current client
	c.printProgress("Determining server IP")
	c.ServerIP, c.AdditionalIPs, err = c.determineServerIP()
	if err != nil {
		return err
	}
	glog.V(3).Infof("Using %q as primary server IP and %q as additional IPs", c.ServerIP, strings.Join(c.AdditionalIPs, ","))
	return nil
}

// Validate validates that required fields in StartConfig have been populated
func (c *ClusterUpConfig) Validate() error {
	if c.dockerClient == nil {
		return fmt.Errorf("missing dockerClient")
	}
	return nil
}

func (c *ClusterUpConfig) printProgress(msg string) {
	fmt.Fprintf(c.Out, msg+" ...\n")
}

// Check is a spot to do NON-MUTATING, preflight checks. Over time, we should try to move our non-mutating checks out of
// Complete and into Check.
func (c *ClusterUpConfig) Check() error {
	c.printProgress(fmt.Sprintf("Checking for supported Docker version (=>%s)", dockerAPIVersion122))
	ver, err := c.Docker().APIVersion()
	if err != nil {
		return err
	}
	if versions.LessThan(ver.APIVersion, dockerAPIVersion122) {
		return fmt.Errorf("unsupported Docker version %s, need at least %s", ver.APIVersion, dockerAPIVersion122)
	}
	c.printProgress("Checking if required ports are available")
	if err := c.checkAvailablePorts(); err != nil {
		return err
	}
	return nil
}

// Start runs the start tasks ensuring that they are executed in sequence
func (c *ClusterUpConfig) Start() error {
	if err := c.StartSelfHosted(c.Out); err != nil {
		return err
	}

	c.printProgress("Server Information")
	c.serverInfo(c.Out)

	return nil
}

// DockerClient obtains a new Docker client from the environment or
// from a Docker machine, starting it if necessary
func (c *ClusterUpConfig) DockerClient() dockerutil.Interface {
	return c.dockerClient
}

// checkAvailablePorts ensures that ports used by OpenShift are available on the Docker host
func (c *ClusterUpConfig) checkAvailablePorts() error {
	err := c.OpenShift().TestPorts(openshift.AllPorts)
	if err == nil {
		return nil
	}
	if !openshift.IsPortsNotAvailableErr(err) {
		return err
	}
	unavailable := sets.NewInt(openshift.UnavailablePorts(err)...)
	if unavailable.HasAny(openshift.BasePorts...) {
		return errors.NewError("a port needed by OpenShift is not available").WithCause(err)
	}
	if unavailable.Has(openshift.DefaultDNSPort) {
		return errors.NewError(fmt.Sprintf("DNS port %d is not available", openshift.DefaultDNSPort))
	}

	for _, port := range openshift.RouterPorts {
		if unavailable.Has(port) {
			glog.Warningf("Port %d is already in use and may cause routing issues for applications.\n", port)
		}
	}
	return nil
}

// determineServerIP gets an appropriate IP address to communicate with the OpenShift server
func (c *ClusterUpConfig) determineServerIP() (string, []string, error) {
	ip, err := c.determineIP()
	if err != nil {
		return "", nil, errors.NewError("cannot determine a server IP to use").WithCause(err)
	}
	serverIP := ip
	additionalIPs, err := c.determineAdditionalIPs(c.ServerIP)
	if err != nil {
		return "", nil, errors.NewError("cannot determine additional IPs").WithCause(err)
	}
	return serverIP, additionalIPs, nil
}

func (c *ClusterUpConfig) imageFormat() string {
	return c.ImageTemplate.Format
}

// serverInfo displays server information after a successful start
func (c *ClusterUpConfig) serverInfo(out io.Writer) {
	masterURL := fmt.Sprintf("https://%s:6443", c.GetPublicHostName())

	msg := fmt.Sprintf("OpenShift server started.\n\n"+
		"The server is accessible via web console at:\n"+
		"    %s\n\n", masterURL)
	fmt.Fprintf(out, msg)
}

// OpenShift returns a helper object to work with OpenShift on the server
func (c *ClusterUpConfig) OpenShift() *openshift.Helper {
	if c.openshiftHelper == nil {
		c.openshiftHelper = openshift.NewHelper(c.Docker(), OpenShiftImages.Get("control-plane").ToPullSpec(c.ImageTemplate).String(), "origin")
	}
	return c.openshiftHelper
}

// Host returns a helper object to check Host configuration
func (c *ClusterUpConfig) Host() *host.HostHelper {
	if c.hostHelper == nil {
		c.hostHelper = host.NewHostHelper(c.Docker(), OpenShiftImages.Get("control-plane").ToPullSpec(c.ImageTemplate).String())
	}
	return c.hostHelper
}

// Docker returns a helper object to work with the Docker client
func (c *ClusterUpConfig) Docker() *dockerutil.Helper {
	if c.dockerHelper == nil {
		c.dockerHelper = dockerutil.NewHelper(c.dockerClient)
	}
	return c.dockerHelper
}

func (c *ClusterUpConfig) determineAdditionalIPs(ip string) ([]string, error) {
	additionalIPs := sets.NewString()
	serverIPs, err := c.OpenShift().OtherIPs(ip)
	if err != nil {
		return nil, errors.NewError("could not determine additional IPs").WithCause(err)
	}
	additionalIPs.Insert(serverIPs...)
	return additionalIPs.List(), nil
}

func (c *ClusterUpConfig) localIPs() ([]string, error) {
	ips := []string{}
	devices, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, dev := range devices {
		if (dev.Flags&net.FlagUp != 0) && (dev.Flags&net.FlagLoopback == 0) {
			addrs, err := dev.Addrs()
			if err != nil {
				continue
			}
			for i := range addrs {
				if ip, ok := addrs[i].(*net.IPNet); ok {
					if ip.IP.To4() != nil {
						ips = append(ips, ip.IP.String())
					}
				}
			}
		}
	}
	return ips, nil
}

func (c *ClusterUpConfig) determineIP() (string, error) {
	if ip := net.ParseIP(c.PublicHostname); ip != nil && !ip.IsUnspecified() {
		fmt.Fprintf(c.Out, "Using public hostname IP %s as the host IP\n", ip)
		return ip.String(), nil
	}

	// Try to get the host from the DOCKER_HOST if communicating via tcp
	var err error
	ip := c.Docker().HostIP()
	if ip != "" {
		glog.V(2).Infof("Testing Docker host IP (%s)", ip)
		if err = c.OpenShift().TestIP(ip); err == nil {
			return ip, nil
		}
	}
	glog.V(2).Infof("Cannot use the Docker host IP(%s): %v", ip, err)

	// If IP is not specified, try to use the loopback IP
	// This is to default to an ip-agnostic client setup
	// where the real IP of the host will not affect client operations
	if err = c.OpenShift().TestIP("127.0.0.1"); err == nil {
		return "127.0.0.1", nil
	}

	// Next, use the the --print-ip output from openshift
	ip, err = c.OpenShift().ServerIP()
	if err == nil {
		glog.V(2).Infof("Testing openshift --print-ip (%s)", ip)
		if err = c.OpenShift().TestIP(ip); err == nil {
			return ip, nil
		}
		glog.V(2).Infof("OpenShift server ip test failed: %v", err)
	}
	glog.V(2).Infof("Cannot use OpenShift IP: %v", err)

	// Next, try other IPs on Docker host
	ips, err := c.OpenShift().OtherIPs(ip)
	if err != nil {
		return "", err
	}
	for i := range ips {
		glog.V(2).Infof("Testing additional IP (%s)", ip)
		if err = c.OpenShift().TestIP(ips[i]); err == nil {
			return ip, nil
		}
		glog.V(2).Infof("OpenShift additional ip test failed: %v", err)
	}
	return "", errors.NewError("cannot determine an IP to use for your server.")
}

func (c *ClusterUpConfig) ClusterAdminKubeConfigBytes() ([]byte, error) {
	return []byte(`apiVersion: v1
kind: Config`), nil
}

func (c *ClusterUpConfig) RESTConfig() (*rest.Config, error) {
	clusterAdminKubeConfigBytes, err := c.ClusterAdminKubeConfigBytes()
	if err != nil {
		return nil, err
	}
	clusterAdminKubeConfig, err := kclientcmd.RESTConfigFromKubeConfig(clusterAdminKubeConfigBytes)
	if err != nil {
		return nil, err
	}

	return clusterAdminKubeConfig, nil
}

func (c *ClusterUpConfig) GetPublicHostName() string {
	if len(c.PublicHostname) > 0 {
		return c.PublicHostname
	}
	return c.ServerIP
}
