package docker

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types/versions"
	cliconfig "github.com/docker/docker/cli/config"
	dockerclient "github.com/docker/docker/client"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	aggregatorinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	userv1client "github.com/openshift/client-go/user/clientset/versioned"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	oauthclientinternal "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/components/registry"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/components/service-catalog"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/host"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/openshift"
	"github.com/openshift/origin/pkg/version"
)

const (
	// CmdUpRecommendedName is the recommended command name
	CmdUpRecommendedName = "up"

	initialUser     = "developer"
	initialPassword = "developer"

	initialProjectName    = "myproject"
	initialProjectDisplay = "My Project"
	initialProjectDesc    = "Initial developer project"

	defaultRedirectClient  = "openshift-web-console"
	developmentRedirectURI = "https://localhost:9000"

	dockerAPIVersion122 = "1.22"
)

var (
	cmdUpLong = templates.LongDesc(`
		Starts an OpenShift cluster using Docker containers, provisioning a registry, router,
		initial templates, and a default project.

		This command will attempt to use an existing connection to a Docker daemon. Before running
		the command, ensure that you can execute docker commands successfully (i.e. 'docker ps').

		By default, the OpenShift cluster will be setup to use a routing suffix that ends in nip.io.
		This is to allow dynamic host names to be created for routes. An alternate routing suffix
		can be specified using the --routing-suffix flag.

		A public hostname can also be specified for the server with the --public-hostname flag.`)

	cmdUpExample = templates.Examples(`
	  # Start OpenShift using a specific public host name
	  %[1]s --public-hostname=my.address.example.com`)

	// defaultImageStreams is the default key for the above imageStreams mapping.
	// It should be set during build via -ldflags.
	defaultImageStreams string
)

// NewCmdUp creates a command that starts OpenShift on Docker with reasonable defaults
func NewCmdUp(name, fullName string, out, errout io.Writer, clusterAdd *cobra.Command) *cobra.Command {
	config := &ClusterUpConfig{
		UserEnabledComponents: []string{"*"},

		Out:            out,
		UsePorts:       openshift.BasePorts,
		PortForwarding: defaultPortForwarding(),
		DNSPort:        openshift.DefaultDNSPort,

		ImageTemplate: variable.NewDefaultImageTemplate(),

		// We pass cluster add as a command to prevent anyone from ever cheating with their wiring. You either work from flags or
		// or you don't work.  You cannot add glue of any sort.
		ClusterAdd: clusterAdd,
	}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Start OpenShift on Docker with reasonable defaults",
		Long:    cmdUpLong,
		Example: fmt.Sprintf(cmdUpExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(config.Complete(c))
			kcmdutil.CheckErr(config.Validate())
			kcmdutil.CheckErr(config.Check())
			if err := config.Start(out); err != nil {
				PrintError(err, errout)
				os.Exit(1)
			}
		},
	}
	config.Bind(cmd.Flags())
	return cmd
}

type ClusterUpConfig struct {
	ImageTemplate variable.ImageTemplate
	ImageTag      string

	DockerMachine         string
	SkipRegistryCheck     bool
	PortForwarding        bool
	ClusterAdd            *cobra.Command
	UserEnabledComponents []string

	Out io.Writer

	// BaseTempDir is the directory to use as the root for temp directories
	// This allows us to bundle all of the cluster-up directories in one spot for easier cleanup and ensures we aren't
	// doing crazy thing like dirtying /var on the host (that does weird stuff)
	BaseDir           string
	SpecifiedBaseDir  bool
	HostName          string
	UseExistingConfig bool
	ServerLogLevel    int

	ComponentsToEnable       []string
	HostVolumesDir           string
	HostConfigDir            string
	WriteConfig              bool
	HostDataDir              string
	UsePorts                 []int
	DNSPort                  int
	ServerIP                 string
	AdditionalIPs            []string
	UseNsenterMount          bool
	PublicHostname           string
	RoutingSuffix            string
	HostPersistentVolumesDir string
	HTTPProxy                string
	HTTPSProxy               string
	NoProxy                  []string

	dockerClient        dockerhelper.Interface
	dockerHelper        *dockerhelper.Helper
	hostHelper          *host.HostHelper
	openshiftHelper     *openshift.Helper
	command             *cobra.Command
	defaultClientConfig clientcmdapi.Config
	isRemoteDocker      bool

	usingDefaultImages         bool
	usingDefaultOpenShiftImage bool

	pullPolicy string

	createdUser bool
}

func (c *ClusterUpConfig) Bind(flags *pflag.FlagSet) {
	flags.StringVar(&c.ImageTag, "tag", "", "Specify an explicit version for OpenShift images")
	flags.MarkHidden("tag")
	flags.StringVar(&c.ImageTemplate.Format, "image", c.ImageTemplate.Format, "Specify the images to use for OpenShift")
	flags.BoolVar(&c.SkipRegistryCheck, "skip-registry-check", false, "Skip Docker daemon registry check")
	flags.StringVar(&c.PublicHostname, "public-hostname", "", "Public hostname for OpenShift cluster")
	flags.StringVar(&c.RoutingSuffix, "routing-suffix", "", "Default suffix for server routes")
	flags.StringVar(&c.BaseDir, "base-dir", c.BaseDir, "Directory on Docker host for cluster up configuration")
	flags.BoolVar(&c.WriteConfig, "write-config", false, "Write the configuration files into host config dir")
	flags.BoolVar(&c.PortForwarding, "forward-ports", c.PortForwarding, "Use Docker port-forwarding to communicate with origin container. Requires 'socat' locally.")
	flags.IntVar(&c.ServerLogLevel, "server-loglevel", 0, "Log level for OpenShift server")
	flags.StringSliceVar(&c.UserEnabledComponents, "enable", c.UserEnabledComponents, fmt.Sprintf(""+
		"A list of components to enable.  '*' enables all on-by-default components, 'foo' enables the component "+
		"named 'foo', '-foo' disables the component named 'foo'.\nAll components: %s\nDisabled-by-default components: %s",
		strings.Join(knownComponents.List(), ", "), strings.Join(componentsDisabledByDefault.List(), ", ")))
	flags.StringVar(&c.HTTPProxy, "http-proxy", "", "HTTP proxy to use for master and builds")
	flags.StringVar(&c.HTTPSProxy, "https-proxy", "", "HTTPS proxy to use for master and builds")
	flags.StringArrayVar(&c.NoProxy, "no-proxy", c.NoProxy, "List of hosts or subnets for which a proxy should not be used")
}

var (
	knownComponents = sets.NewString(
		"centos-imagestreams",
		"registry",
		"rhel-imagestreams",
		"router",
		"sample-templates",
		"persistent-volumes",
		"service-catalog",
		"template-service-broker",
		"web-console",
	)

	componentsDisabledByDefault = sets.NewString(
		"service-catalog",
		"template-service-broker")
)

func init() {
	switch defaultImageStreams {
	case "centos7":
		componentsDisabledByDefault.Insert("rhel-imagestreams")
	case "rhel7":
		componentsDisabledByDefault.Insert("centos-imagestreams")
	}
}

func (c *ClusterUpConfig) Complete(cmd *cobra.Command) error {
	// TODO: remove this when we move to container/apply based component installation
	aggregatorinstall.Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)

	// Set the ImagePullPolicy field in static pods and components based in whether users specified
	// the --tag flag or not.
	c.pullPolicy = "Always"
	if len(c.ImageTag) > 0 {
		c.pullPolicy = "IfNotPresent"
	}
	glog.V(5).Infof("Using %q as default image pull policy", c.pullPolicy)

	// Get the default client config for login
	var err error
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	c.defaultClientConfig, err = kcmdutil.DefaultClientConfig(flags).RawConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		c.defaultClientConfig = *clientcmdapi.NewConfig()
	}

	c.command = cmd

	c.isRemoteDocker = len(os.Getenv("DOCKER_HOST")) > 0

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

	// Check if users are not trying to re-run cluster up on existing configuration
	// but with different components enabled.
	if !sets.NewString(c.UserEnabledComponents...).Equal(sets.NewString("*")) {
		if _, err := os.Stat(filepath.Join(c.BaseDir, "etcd")); err == nil {
			return fmt.Errorf("cannot use --enable when the cluster is already initialized, use cluster add instead")
		}
	}

	for _, currComponent := range knownComponents.UnsortedList() {
		if isComponentEnabled(currComponent, componentsDisabledByDefault, c.UserEnabledComponents...) {
			c.ComponentsToEnable = append(c.ComponentsToEnable, currComponent)
		}
	}

	// Get a Docker client.
	// If a Docker machine was specified, make sure that the machine is running.
	// Otherwise, use environment variables.
	c.printProgress("Getting a Docker client")
	client, err := GetDockerClient()
	if err != nil {
		return err
	}
	c.dockerClient = client

	// Ensure that the OpenShift Docker image is available.
	// If not present, pull it.
	// We do this here because the image is used in the next step if running Red Hat docker.
	c.printProgress(fmt.Sprintf("Checking if image %s is available", c.openshiftImage()))
	if err := c.checkOpenShiftImage(); err != nil {
		return err
	}

	// Check whether the Docker host has the right binaries to use Kubernetes' nsenter mounter
	// If not, use a shared volume to mount volumes on OpenShift
	if isRedHatDocker, err := c.DockerHelper().IsRedHat(); err == nil && isRedHatDocker {
		c.printProgress("Checking type of volume mount")
		c.UseNsenterMount, err = c.HostHelper().CanUseNsenterMounter()
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(c.BaseDir, 0755); err != nil {
		return err
	}

	if c.UseNsenterMount {
		// This is default path when you run cluster up locally, with local docker daemon
		c.HostVolumesDir = path.Join(c.BaseDir, "openshift.local.volumes")
		// This is a snowflake when Docker runs on remote host
		if c.isRemoteDocker {
			c.HostVolumesDir = c.RemoteDirFor("openshift.local.volumes")
		} else {
			if err := os.MkdirAll(c.HostVolumesDir, 0755); err != nil {
				return err
			}
		}
	} else {
		// Snowflake for OSX Docker for Mac
		c.HostVolumesDir = c.RemoteDirFor("openshift.local.volumes")
	}

	c.HostPersistentVolumesDir = path.Join(c.BaseDir, "openshift.local.pv")
	if c.isRemoteDocker {
		c.HostPersistentVolumesDir = c.RemoteDirFor("openshift.local.pv")
	} else {
		if err := os.MkdirAll(c.HostPersistentVolumesDir, 0755); err != nil {
			return err
		}
	}

	c.HostDataDir = path.Join(c.BaseDir, "etcd")
	if c.isRemoteDocker {
		c.HostDataDir = c.RemoteDirFor("etcd")
	} else {
		if err := os.MkdirAll(c.HostDataDir, 0755); err != nil {
			return err
		}
	}

	// Ensure that host directories exist.
	// If not using the nsenter mounter, create a volume share on the host machine to
	// mount OpenShift volumes.
	if !c.UseNsenterMount {
		c.printProgress("Creating shared mount directory on the remote host")
		if err := c.HostHelper().EnsureVolumeUseShareMount(c.HostVolumesDir); err != nil {
			return err
		}
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

	if len(c.RoutingSuffix) == 0 {
		c.RoutingSuffix = c.ServerIP + ".nip.io"
	}
	// this used to be done in the openshift start method, but its mutating state.
	if len(c.HTTPProxy) > 0 || len(c.HTTPSProxy) > 0 {
		c.updateNoProxy()
	}

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
	// Check for an OpenShift container. If one exists and is running, exit.
	// If one exists but not running, delete it.
	c.printProgress("Checking if OpenShift is already running")
	if err := checkExistingOpenShiftContainer(c.DockerHelper()); err != nil {
		return err
	}

	// Docker checks
	c.printProgress(fmt.Sprintf("Checking for supported Docker version (=>%s)", dockerAPIVersion122))
	ver, err := c.DockerHelper().APIVersion()
	if err != nil {
		return err
	}
	if versions.LessThan(ver.APIVersion, dockerAPIVersion122) {
		return fmt.Errorf("unsupported Docker version %s, need at least %s", ver.APIVersion, dockerAPIVersion122)
	}

	if !c.SkipRegistryCheck {
		c.printProgress("Checking if insecured registry is configured properly in Docker")
		if err := c.checkDockerInsecureRegistry(); err != nil {
			return err
		}
	}

	// Networking checks
	if c.PortForwarding {
		c.printProgress("Checking prerequisites for port forwarding")
		if err := checkPortForwardingPrerequisites(); err != nil {
			return err
		}
		if err := openshift.CheckSocat(); err != nil {
			return err
		}
	}

	c.printProgress("Checking if required ports are available")
	if err := c.checkAvailablePorts(); err != nil {
		return err
	}

	// OpenShift checks
	c.printProgress("Checking if OpenShift client is configured properly")
	if err := c.checkOpenShiftClient(); err != nil {
		return err
	}

	// Ensure that the OpenShift Docker image is available.
	// If not present, pull it.
	c.printProgress(fmt.Sprintf("Checking if image %s is available", c.openshiftImage()))
	if err := c.checkOpenShiftImage(); err != nil {
		return err
	}

	return nil
}

// Start runs the start tasks ensuring that they are executed in sequence
func (c *ClusterUpConfig) Start(out io.Writer) error {
	fmt.Fprintf(out, "Starting OpenShift using %s ...\n", c.openshiftImage())

	if c.PortForwarding {
		if err := c.OpenShiftHelper().StartSocatTunnel(c.ServerIP); err != nil {
			return err
		}
	}

	if err := c.StartSelfHosted(out); err != nil {
		return err
	}
	if c.WriteConfig {
		return nil
	}
	if err := c.PostClusterStartupMutations(out); err != nil {
		return err
	}

	// Add default redirect URIs to an OAuthClient to enable local web-console development.
	c.printProgress("Adding default OAuthClient redirect URIs")
	if err := c.ensureDefaultRedirectURIs(out); err != nil {
		return err
	}

	if len(c.ComponentsToEnable) > 0 {
		args := append([]string{}, "--image="+c.ImageTemplate.Format)
		args = append(args, "--base-dir="+c.BaseDir)
		if len(c.ImageTag) > 0 {
			args = append(args, "--tag="+c.ImageTag)
		}
		args = append(args, c.ComponentsToEnable...)

		if err := c.ClusterAdd.ParseFlags(args); err != nil {
			return err
		}
		glog.V(2).Infof("oc cluster add %v", args)
		if err := c.ClusterAdd.RunE(c.ClusterAdd, args); err != nil {
			return err
		}
	}

	if c.ShouldCreateUser() {
		// Login with an initial default user
		c.printProgress("Login to server")
		if err := c.login(out); err != nil {
			return err
		}
		c.createdUser = true

		// Create an initial project
		c.printProgress(fmt.Sprintf("Creating initial project %q", initialProjectName))
		if err := c.createProject(out); err != nil {
			return err
		}
	}

	c.printProgress("Server Information")
	c.serverInfo(out)

	return nil
}

func defaultPortForwarding() bool {
	// Defaults to true if running on Mac, with no DOCKER_HOST defined
	return runtime.GOOS == "darwin" && len(os.Getenv("DOCKER_HOST")) == 0
}

// checkOpenShiftClient ensures that the client can be configured
// for the new server
func (c *ClusterUpConfig) checkOpenShiftClient() error {
	kubeConfig := os.Getenv("KUBECONFIG")
	if len(kubeConfig) == 0 {
		return nil
	}

	// if you're trying to use the kubeconfig into a subdirectory of the basedir, you're probably using a KUBECONFIG
	// location that is going to overwrite a "real" kubeconfig, usually admin.kubeconfig which will break every other component
	// relying on it being a full power kubeconfig
	kubeConfigDir := filepath.Dir(kubeConfig)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	absKubeConfigDir, err := cmdutil.MakeAbs(kubeConfigDir, cwd)
	if err != nil {
		return err
	}
	if strings.HasPrefix(absKubeConfigDir, c.BaseDir+"/") {
		return fmt.Errorf("cannot choose kubeconfig in subdirectory of the --base-dir: %q", kubeConfig)
	}

	var (
		kubeConfigError error
		f               *os.File
	)
	_, err = os.Stat(kubeConfig)
	switch {
	case os.IsNotExist(err):
		err = os.MkdirAll(filepath.Dir(kubeConfig), 0755)
		if err != nil {
			kubeConfigError = fmt.Errorf("cannot make directory: %v", err)
			break
		}
		f, err = os.Create(kubeConfig)
		if err != nil {
			kubeConfigError = fmt.Errorf("cannot create file: %v", err)
			break
		}
		f.Close()
	case err == nil:
		f, err = os.OpenFile(kubeConfig, os.O_RDWR, 0644)
		if err != nil {
			kubeConfigError = fmt.Errorf("cannot open %s for write: %v", kubeConfig, err)
			break
		}
		f.Close()
	default:
		kubeConfigError = fmt.Errorf("cannot access %s: %v", kubeConfig, err)
	}
	if kubeConfigError != nil {
		return errors.ErrKubeConfigNotWriteable(kubeConfig, kubeConfigError)
	}
	return nil
}

// GetDockerClient obtains a new Docker client from the environment or
// from a Docker machine, starting it if necessary
func (c *ClusterUpConfig) GetDockerClient() dockerhelper.Interface {
	return c.dockerClient
}

// GetDockerClient obtains a new Docker client from the environment or
// from a Docker machine, starting it if necessary and permitted
func GetDockerClient() (dockerhelper.Interface, error) {
	dockerTLSVerify := os.Getenv("DOCKER_TLS_VERIFY")
	dockerCertPath := os.Getenv("DOCKER_CERT_PATH")
	if len(dockerTLSVerify) > 0 && len(dockerCertPath) == 0 {
		dockerCertPath = cliconfig.Dir()
		os.Setenv("DOCKER_CERT_PATH", dockerCertPath)
	}

	if glog.V(4) {
		dockerHost := os.Getenv("DOCKER_HOST")
		if len(dockerHost) == 0 && len(dockerTLSVerify) == 0 && len(dockerCertPath) == 0 {
			glog.Infof("No Docker environment variables found. Will attempt default socket.")
		}
		if len(dockerHost) > 0 {
			glog.Infof("Will try Docker connection with host (DOCKER_HOST) %q", dockerHost)
		} else {
			glog.Infof("No Docker host (DOCKER_HOST) configured. Will attempt default socket.")
		}
		if len(dockerTLSVerify) > 0 {
			glog.Infof("DOCKER_TLS_VERIFY=%s", dockerTLSVerify)
		}
		if len(dockerCertPath) > 0 {
			glog.Infof("DOCKER_CERT_PATH=%s", dockerCertPath)
		}
	}
	dockerHost := os.Getenv("DOCKER_HOST")
	if len(dockerHost) == 0 {
		dockerHost = dockerclient.DefaultDockerHost
	}
	engineAPIClient, err := dockerclient.NewEnvClient()
	if err != nil {
		return nil, errors.ErrNoDockerClient(err)
	}
	// negotiate the correct API version with the server
	ctx, fn := context.WithTimeout(context.Background(), 10*time.Second)
	defer fn()
	engineAPIClient.NegotiateAPIVersion(ctx)
	return dockerhelper.NewClient(dockerHost, engineAPIClient), nil
}

// checkExistingOpenShiftContainer checks the state of an OpenShift container.
// If one is already running, it throws an error.
// If one exists, it removes it so a new one can be created.
func checkExistingOpenShiftContainer(dockerHelper *dockerhelper.Helper) error {
	container, running, err := dockerHelper.GetContainerState(openshift.ContainerName)
	if err != nil {
		return errors.NewError("unexpected error while checking OpenShift container state").WithCause(err)
	}
	if running {
		return errors.NewError("OpenShift is already running").WithSolution("To start OpenShift again, stop the current cluster:\n$ %s\n", "oc cluster down")
	}
	if container != nil {
		err = dockerHelper.RemoveContainer(openshift.ContainerName)
		if err != nil {
			return errors.NewError("cannot delete existing OpenShift container").WithCause(err)
		}
		glog.V(2).Info("Deleted existing OpenShift container")
	}
	return nil
}

// checkOpenShiftImage checks whether the OpenShift image exists.
// If not it tells the Docker daemon to pull it.
func (c *ClusterUpConfig) checkOpenShiftImage() error {
	if err := c.DockerHelper().CheckAndPull(c.openshiftImage(), c.Out); err != nil {
		return err
	}
	if err := c.DockerHelper().CheckAndPull(c.cliImage(), c.Out); err != nil {
		return err
	}
	if err := c.DockerHelper().CheckAndPull(c.nodeImage(), c.Out); err != nil {
		return err
	}
	return nil
}

// checkDockerInsecureRegistry checks to see if the Docker daemon has an appropriate insecure registry argument set so that our services can access the registry
func (c *ClusterUpConfig) checkDockerInsecureRegistry() error {
	configured, hasEntries, err := c.DockerHelper().InsecureRegistryIsConfigured(openshift.DefaultSvcCIDR)
	if err != nil {
		return err
	}
	if !configured {
		if hasEntries {
			return errors.ErrInvalidInsecureRegistryArgument()
		}
		return errors.ErrNoInsecureRegistryArgument()
	}
	return nil
}

// checkPortForwardingPrerequisites checks that socat is installed when port forwarding is enabled
// Socat needs to be installed manually on MacOS
func checkPortForwardingPrerequisites() error {
	commandOut, err := exec.Command("socat", "-V").CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Error from socat command execution: %v\n%s", err, string(commandOut))
		glog.Warning("Port forwarding requires socat command line utility." +
			"Cluster public ip may not be reachable. Please make sure socat installed in your operating system.")
	}
	return nil
}

// ensureDefaultRedirectURIs merges a default URL to an auth client's RedirectURIs array
func (c *ClusterUpConfig) ensureDefaultRedirectURIs(out io.Writer) error {
	restConfig, err := c.RESTConfig()
	if err != nil {
		return err
	}
	oauthClient, err := oauthclientinternal.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	webConsoleOAuth, err := oauthClient.Oauth().OAuthClients().Get(defaultRedirectClient, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			fmt.Fprintf(out, "Unable to find OAuthClient %q\n", defaultRedirectClient)
			return nil
		}

		// announce fetch error without interrupting remaining tasks
		suggestedCmd := fmt.Sprintf("oc patch %s/%s -p '{%q:[%q]}'", "oauthclient", defaultRedirectClient, "redirectURIs", developmentRedirectURI)
		errMsg := fmt.Sprintf("Unable to fetch OAuthClient %q.\nTo manually add a development redirect URI, run %q\n", defaultRedirectClient, suggestedCmd)
		fmt.Fprintf(out, "%s\n", errMsg)
		return nil
	}

	// ensure the default redirect URI is not already present
	redirects := sets.NewString(webConsoleOAuth.RedirectURIs...)
	if redirects.Has(developmentRedirectURI) {
		return nil
	}

	webConsoleOAuth.RedirectURIs = append(webConsoleOAuth.RedirectURIs, developmentRedirectURI)

	_, err = oauthClient.Oauth().OAuthClients().Update(webConsoleOAuth)
	if err != nil {
		// announce error without interrupting remaining tasks
		suggestedCmd := fmt.Sprintf("oc patch %s/%s -p '{%q:[%q]}'", "oauthclient", defaultRedirectClient, "redirectURIs", developmentRedirectURI)
		fmt.Fprintf(out, fmt.Sprintf("Unable to add development redirect URI to the %q OAuthClient.\nTo manually add it, run %q\n", defaultRedirectClient, suggestedCmd))
		return nil
	}

	return nil
}

// checkAvailablePorts ensures that ports used by OpenShift are available on the Docker host
func (c *ClusterUpConfig) checkAvailablePorts() error {
	err := c.OpenShiftHelper().TestPorts(openshift.AllPorts)
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

// updateNoProxy will add some default values to the NO_PROXY setting if they are not present
func (c *ClusterUpConfig) updateNoProxy() {
	values := []string{"127.0.0.1", c.ServerIP, "localhost", service_catalog.ServiceCatalogServiceIP, registry.RegistryServiceClusterIP}
	ipFromServer, err := c.OpenShiftHelper().ServerIP()
	if err == nil {
		values = append(values, ipFromServer)
	}
	noProxySet := sets.NewString(c.NoProxy...)
	for _, v := range values {
		if !noProxySet.Has(v) {
			noProxySet.Insert(v)
			c.NoProxy = append(c.NoProxy, v)
		}
	}
}

func (c *ClusterUpConfig) PostClusterStartupMutations(out io.Writer) error {
	restConfig, err := c.RESTConfig()
	if err != nil {
		return err
	}
	kClient, err := kclientset.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// Remove any duplicate nodes
	if err := c.OpenShiftHelper().CheckNodes(kClient); err != nil {
		return err
	}

	return nil
}

func (c *ClusterUpConfig) imageFormat() string {
	return c.ImageTemplate.Format
}

// Login logs into the new server and sets up a default user and project
func (c *ClusterUpConfig) login(out io.Writer) error {
	server := c.OpenShiftHelper().Master(c.ServerIP)
	return openshift.Login(initialUser, initialPassword, server, c.GetKubeAPIServerConfigDir(), c.defaultClientConfig, c.command, out, out)
}

// createProject creates a new project for the current user
func (c *ClusterUpConfig) createProject(out io.Writer) error {
	f, err := openshift.LoggedInUserFactory()
	if err != nil {
		return errors.NewError("cannot get logged in user client").WithCause(err)
	}
	return openshift.CreateProject(f, initialProjectName, initialProjectDisplay, initialProjectDesc, "oc", out)
}

// serverInfo displays server information after a successful start
func (c *ClusterUpConfig) serverInfo(out io.Writer) {
	masterURL := fmt.Sprintf("https://%s:8443", c.GetPublicHostName())

	msg := fmt.Sprintf("OpenShift server started.\n\n"+
		"The server is accessible via web console at:\n"+
		"    %s\n\n", masterURL)

	if c.createdUser {
		msg += fmt.Sprintf("You are logged in as:\n"+
			"    User:     %s\n"+
			"    Password: <any value>\n\n", initialUser)
		msg += "To login as administrator:\n" +
			"    oc login -u system:admin\n\n"
	}

	msg += c.checkProxySettings()

	fmt.Fprintf(out, msg)
}

// checkProxySettings compares proxy settings specified for cluster up
// and those on the Docker daemon and generates appropriate warnings.
func (c *ClusterUpConfig) checkProxySettings() string {
	warnings := []string{}
	dockerHTTPProxy, dockerHTTPSProxy, dockerNoProxy, err := c.DockerHelper().GetDockerProxySettings()
	if err != nil {
		return "Unexpected error: " + err.Error()
	}
	// Check HTTP proxy
	if len(c.HTTPProxy) > 0 && len(dockerHTTPProxy) == 0 {
		warnings = append(warnings, "You specified an HTTP proxy for cluster up, but one is not configured for the Docker daemon")
	} else if len(c.HTTPProxy) == 0 && len(dockerHTTPProxy) > 0 {
		warnings = append(warnings, fmt.Sprintf("An HTTP proxy (%s) is configured for the Docker daemon, but you did not specify one for cluster up", dockerHTTPProxy))
	} else if c.HTTPProxy != dockerHTTPProxy {
		warnings = append(warnings, fmt.Sprintf("The HTTP proxy configured for the Docker daemon (%s) is not the same one you specified for cluster up", dockerHTTPProxy))
	}

	// Check HTTPS proxy
	if len(c.HTTPSProxy) > 0 && len(dockerHTTPSProxy) == 0 {
		warnings = append(warnings, "You specified an HTTPS proxy for cluster up, but one is not configured for the Docker daemon")
	} else if len(c.HTTPSProxy) == 0 && len(dockerHTTPSProxy) > 0 {
		warnings = append(warnings, fmt.Sprintf("An HTTPS proxy (%s) is configured for the Docker daemon, but you did not specify one for cluster up", dockerHTTPSProxy))
	} else if c.HTTPSProxy != dockerHTTPSProxy {
		warnings = append(warnings, fmt.Sprintf("The HTTPS proxy configured for the Docker daemon (%s) is not the same one you specified for cluster up", dockerHTTPSProxy))
	}

	if len(dockerHTTPProxy) > 0 || len(dockerHTTPSProxy) > 0 {
		dockerNoProxyList := strings.Split(dockerNoProxy, ",")
		dockerNoProxySet := sets.NewString(dockerNoProxyList...)
		if !dockerNoProxySet.Has(registry.RegistryServiceClusterIP) {
			warnings = append(warnings, fmt.Sprintf("A proxy is configured for Docker, however %[1]s is not included in its NO_PROXY list.\n"+
				"   %[1]s needs to be included in the Docker daemon's NO_PROXY environment variable so pushes to the local OpenShift registry can succeed.", registry.RegistryServiceClusterIP))
		}
	}

	if len(warnings) > 0 {
		buf := &bytes.Buffer{}
		for _, w := range warnings {
			fmt.Fprintf(buf, "WARNING: %s\n", w)
		}
		return buf.String()
	}
	return ""
}

// OpenShiftHelper returns a helper object to work with OpenShift on the server
func (c *ClusterUpConfig) OpenShiftHelper() *openshift.Helper {
	if c.openshiftHelper == nil {
		c.openshiftHelper = openshift.NewHelper(c.DockerHelper(), c.openshiftImage(), openshift.ContainerName)
	}
	return c.openshiftHelper
}

// HostHelper returns a helper object to check Host configuration
func (c *ClusterUpConfig) HostHelper() *host.HostHelper {
	if c.hostHelper == nil {
		c.hostHelper = host.NewHostHelper(c.DockerHelper(), c.openshiftImage())
	}
	return c.hostHelper
}

// DockerHelper returns a helper object to work with the Docker client
func (c *ClusterUpConfig) DockerHelper() *dockerhelper.Helper {
	if c.dockerHelper == nil {
		c.dockerHelper = dockerhelper.NewHelper(c.dockerClient)
	}
	return c.dockerHelper
}

func (c *ClusterUpConfig) openshiftImage() string {
	return c.ImageTemplate.ExpandOrDie("control-plane")
}

func (c *ClusterUpConfig) hypershiftImage() string {
	return c.ImageTemplate.ExpandOrDie("hypershift")
}

func (c *ClusterUpConfig) hyperkubeImage() string {
	return c.ImageTemplate.ExpandOrDie("hyperkube")
}

func (c *ClusterUpConfig) cliImage() string {
	return c.ImageTemplate.ExpandOrDie("cli")
}

func (c *ClusterUpConfig) nodeImage() string {
	return c.ImageTemplate.ExpandOrDie("node")
}

func (c *ClusterUpConfig) determineAdditionalIPs(ip string) ([]string, error) {
	additionalIPs := sets.NewString()
	serverIPs, err := c.OpenShiftHelper().OtherIPs(ip)
	if err != nil {
		return nil, errors.NewError("could not determine additional IPs").WithCause(err)
	}
	additionalIPs.Insert(serverIPs...)
	if c.PortForwarding {
		localIPs, err := c.localIPs()
		if err != nil {
			return nil, errors.NewError("could not determine additional local IPs").WithCause(err)
		}
		additionalIPs.Insert(localIPs...)
	}
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

	// If using port-forwarding, use the default loopback address
	if c.PortForwarding {
		return "127.0.0.1", nil
	}

	// Try to get the host from the DOCKER_HOST if communicating via tcp
	var err error
	ip := c.DockerHelper().HostIP()
	if ip != "" {
		glog.V(2).Infof("Testing Docker host IP (%s)", ip)
		if err = c.OpenShiftHelper().TestIP(ip); err == nil {
			return ip, nil
		}
	}
	glog.V(2).Infof("Cannot use the Docker host IP(%s): %v", ip, err)

	// If IP is not specified, try to use the loopback IP
	// This is to default to an ip-agnostic client setup
	// where the real IP of the host will not affect client operations
	if err = c.OpenShiftHelper().TestIP("127.0.0.1"); err == nil {
		return "127.0.0.1", nil
	}

	// Next, use the the --print-ip output from openshift
	ip, err = c.OpenShiftHelper().ServerIP()
	if err == nil {
		glog.V(2).Infof("Testing openshift --print-ip (%s)", ip)
		if err = c.OpenShiftHelper().TestIP(ip); err == nil {
			return ip, nil
		}
		glog.V(2).Infof("OpenShift server ip test failed: %v", err)
	}
	glog.V(2).Infof("Cannot use OpenShift IP: %v", err)

	// Next, try other IPs on Docker host
	ips, err := c.OpenShiftHelper().OtherIPs(ip)
	if err != nil {
		return "", err
	}
	for i := range ips {
		glog.V(2).Infof("Testing additional IP (%s)", ip)
		if err = c.OpenShiftHelper().TestIP(ips[i]); err == nil {
			return ip, nil
		}
		glog.V(2).Infof("OpenShift additional ip test failed: %v", err)
	}
	return "", errors.NewError("cannot determine an IP to use for your server.")
}

// ShouldCreateUser determines whether a user and project should
// be created. If the user provider has been modified in the config, then it should
// not attempt to create a user. Also, even if the user provider has not been
// modified, but data has been initialized, then we should also not create user.
func (c *ClusterUpConfig) ShouldCreateUser() bool {
	restClientConfig, err := c.RESTConfig()
	if err != nil {
		glog.Warningf("error checking user: %v", err)
		return true
	}
	userClient, err := userv1client.NewForConfig(restClientConfig)
	if err != nil {
		glog.Warningf("error checking user: %v", err)
		return true
	}
	_, err = userClient.UserV1().Users().Get(initialUser, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		return true
	}
	if err != nil {
		glog.Warningf("error checking user: %v", err)
		return true
	}

	return false
}

func (c *ClusterUpConfig) GetKubeAPIServerConfigDir() string {
	return path.Join(c.BaseDir, kubeapiserver.KubeAPIServerDirName)
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

func (c *ClusterUpConfig) ClusterAdminKubeConfigBytes() ([]byte, error) {
	return ioutil.ReadFile(path.Join(c.GetKubeAPIServerConfigDir(), "admin.kubeconfig"))
}

func (c *ClusterUpConfig) GetPublicHostName() string {
	if len(c.PublicHostname) > 0 {
		return c.PublicHostname
	}
	return c.ServerIP
}

func isComponentEnabled(name string, disabledByDefaultComponents sets.String, components ...string) bool {
	hasStar := false
	for _, ctrl := range components {
		if ctrl == name {
			return true
		}
		if ctrl == "-"+name {
			return false
		}
		if ctrl == "*" {
			hasStar = true
		}
	}
	// if we get here, there was no explicit choice
	if !hasStar {
		// nothing on by default
		return false
	}
	if disabledByDefaultComponents.Has(name) {
		return false
	}

	return true
}
