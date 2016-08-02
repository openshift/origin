package docker

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/blang/semver"
	dockerclient "github.com/docker/engine-api/client"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/bootstrap/docker/dockermachine"
	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/bootstrap/docker/host"
	"github.com/openshift/origin/pkg/bootstrap/docker/openshift"
	"github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

const (
	// CmdUpRecommendedName is the recommended command name
	CmdUpRecommendedName = "up"

	openShiftContainer = "origin"
	openShiftNamespace = "openshift"

	initialUser     = "developer"
	initialPassword = "developer"

	initialProjectName    = "myproject"
	initialProjectDisplay = "My Project"
	initialProjectDesc    = "Initial developer project"

	defaultImages         = "openshift/origin-${component}:${version}"
	defaultOpenShiftImage = "openshift/origin:${version}"

	cmdUpLong = `
Starts an OpenShift cluster using Docker containers, provisioning a registry, router,
initial templates, and a default project.

This command will attempt to use an existing connection to a Docker daemon. Before running
the command, ensure that you can execure docker commands successfully (ie. 'docker ps').

Optionally, the command can create a new Docker machine for OpenShift using the VirtualBox
driver when the --create-machine argument is specified. The machine will be named 'openshift'
by default. To name the machine differently, use the --docker-machine=NAME argument. If the
--docker-machine=NAME argument is specified, but --create-machine is not, the command will attempt
to find an existing docker machine with that name and start it if it's not running.

By default, the OpenShift cluster will be setup to use a routing suffix that ends in xip.io.
This is to allow dynamic host names to be created for routes. An alternate routing suffix
can be specified using the --routing-suffix flag.

A public hostname can also be specified for the server with the --public-hostname flag.
`
	cmdUpExample = `
  # Start OpenShift on a new docker machine named 'openshift'
  %[1]s --create-machine

  # Start OpenShift using a specific public host name
  %[1]s --public-hostname=my.address.example.com

  # Start OpenShift and preserve data and config between restarts
  %[1]s --host-data-dir=/mydata --use-existing-config

  # Use a different set of images
  %[1]s --image="registry.example.com/origin" --version="v1.1"
`
)

var (
	imageStreamLocations = map[string]string{
		"origin centos7 image streams": "examples/image-streams/image-streams-centos7.json",
	}
	templateLocations = map[string]string{
		"mongodb":            "examples/db-templates/mongodb-ephemeral-template.json",
		"mariadb":            "examples/db-templates/mariadb-ephemeral-template.json",
		"mysql":              "examples/db-templates/mysql-ephemeral-template.json",
		"postgresql":         "examples/db-templates/postgresql-ephemeral-template.json",
		"cakephp quickstart": "examples/quickstarts/cakephp-mysql.json",
		"dancer quickstart":  "examples/quickstarts/dancer-mysql.json",
		"django quickstart":  "examples/quickstarts/django-postgresql.json",
		"nodejs quickstart":  "examples/quickstarts/nodejs-mongodb.json",
		"rails quickstart":   "examples/quickstarts/rails-postgresql.json",
		"jenkins pipeline":   "examples/jenkins/pipeline/jenkinstemplate.json",
		"sample pipeline":    "examples/jenkins/pipeline/samplepipeline.json",
	}
)

// NewCmdUp creates a command that starts openshift on Docker with reasonable defaults
func NewCmdUp(name, fullName string, f *osclientcmd.Factory, out io.Writer) *cobra.Command {
	config := &ClientStartConfig{
		Out:            out,
		PortForwarding: defaultPortForwarding(),
	}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Start OpenShift on Docker with reasonable defaults",
		Long:    cmdUpLong,
		Example: fmt.Sprintf(cmdUpExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(config.Complete(f, c))
			kcmdutil.CheckErr(config.Validate(out))
			if err := config.Start(out); err != nil {
				os.Exit(1)
			}
		},
	}
	cmd.Flags().BoolVar(&config.ShouldCreateDockerMachine, "create-machine", false, "Create a Docker machine if one doesn't exist")
	cmd.Flags().StringVar(&config.DockerMachine, "docker-machine", "", "Specify the Docker machine to use")
	cmd.Flags().StringVar(&config.ImageVersion, "version", "", "Specify the tag for OpenShift images")
	cmd.Flags().StringVar(&config.Image, "image", "openshift/origin", "Specify the images to use for OpenShift")
	cmd.Flags().BoolVar(&config.SkipRegistryCheck, "skip-registry-check", false, "Skip Docker daemon registry check")
	cmd.Flags().StringVar(&config.PublicHostname, "public-hostname", "", "Public hostname for OpenShift cluster")
	cmd.Flags().StringVar(&config.RoutingSuffix, "routing-suffix", "", "Default suffix for server routes")
	cmd.Flags().BoolVar(&config.UseExistingConfig, "use-existing-config", false, "Use existing configuration if present")
	cmd.Flags().StringVar(&config.HostConfigDir, "host-config-dir", host.DefaultConfigDir, "Directory on Docker host for OpenShift configuration")
	cmd.Flags().StringVar(&config.HostVolumesDir, "host-volumes-dir", host.DefaultVolumesDir, "Directory on Docker host for OpenShift volumes")
	cmd.Flags().StringVar(&config.HostDataDir, "host-data-dir", "", "Directory on Docker host for OpenShift data. If not specified, etcd data will not be persisted on the host.")
	cmd.Flags().BoolVar(&config.PortForwarding, "forward-ports", config.PortForwarding, "Use Docker port-forwarding to communicate with origin container. Requires 'socat' locally.")
	cmd.Flags().IntVar(&config.ServerLogLevel, "server-loglevel", 0, "Log level for OpenShift server")
	cmd.Flags().StringSliceVarP(&config.Environment, "env", "e", config.Environment, "Specify key value pairs of environment variables to set on OpenShift container")
	cmd.Flags().BoolVar(&config.ShouldInstallMetrics, "metrics", false, "Install metrics (experimental)")
	return cmd
}

// taskFunc is a function that executes a start task
type taskFunc func(io.Writer) error

// task is a named task for the start process
type task struct {
	name string
	fn   taskFunc
}

// ClientStartConfig is the configuration for the client start command
type ClientStartConfig struct {
	ImageVersion              string
	Image                     string
	DockerMachine             string
	ShouldCreateDockerMachine bool
	SkipRegistryCheck         bool
	ShouldInstallMetrics      bool
	PortForwarding            bool

	UseNsenterMount bool
	Out             io.Writer
	TaskPrinter     *TaskPrinter
	Tasks           []task
	HostName        string
	ServerIP        string
	CACert          string
	PublicHostname  string
	RoutingSuffix   string
	DNSPort         int

	LocalConfigDir    string
	HostVolumesDir    string
	HostConfigDir     string
	HostDataDir       string
	UseExistingConfig bool
	Environment       []string
	ServerLogLevel    int

	dockerClient    *docker.Client
	engineAPIClient *dockerclient.Client
	dockerHelper    *dockerhelper.Helper
	hostHelper      *host.HostHelper
	openShiftHelper *openshift.Helper
	factory         *clientcmd.Factory
	originalFactory *clientcmd.Factory
	command         *cobra.Command

	usingDefaultImages         bool
	usingDefaultOpenShiftImage bool
}

func (c *ClientStartConfig) addTask(name string, fn taskFunc) {
	c.Tasks = append(c.Tasks, task{name: name, fn: fn})
}

// Complete initializes fields in StartConfig based on command parameters
// and execution environment
func (c *ClientStartConfig) Complete(f *osclientcmd.Factory, cmd *cobra.Command) error {
	c.TaskPrinter = NewTaskPrinter(c.Out)
	c.originalFactory = f
	c.command = cmd

	if len(c.ImageVersion) == 0 {
		c.ImageVersion = defaultImageVersion()
	}

	c.addTask("Checking OpenShift client", c.CheckOpenShiftClient)

	if c.ShouldCreateDockerMachine {
		// Create a Docker machine first if flag specified
		c.addTask("Create Docker machine", c.CreateDockerMachine)
	}
	// Get a Docker client.
	// If a Docker machine was specified, make sure that the machine is
	// running. Otherwise, use environment variables.
	c.addTask("Checking Docker client", c.GetDockerClient)

	// Check for an OpenShift container. If one exists and is running, exit.
	// If one exists but not running, delete it.
	c.addTask("Checking for existing OpenShift container", c.CheckExistingOpenShiftContainer)

	// Ensure that the OpenShift Docker image is available. If not present,
	// pull it.
	c.addTask(fmt.Sprintf("Checking for %s image", c.openShiftImage()), c.CheckOpenShiftImage)

	// Ensure that the Docker daemon has the right --insecure-registry argument. If
	// not, then exit.
	if !c.SkipRegistryCheck {
		c.addTask("Checking Docker daemon configuration", c.CheckDockerInsecureRegistry)
	}

	// Ensure that ports used by OpenShift are available on the host machine
	c.addTask("Checking for available ports", c.CheckAvailablePorts)

	// Check whether the Docker host has the right binaries to use Kubernetes' nsenter mounter
	// If not, use a shared volume to mount volumes on OpenShift
	c.addTask("Checking type of volume mount", c.CheckNsenterMounter)

	// Check that we have the minimum Docker version available to run OpenShift
	c.addTask("Checking Docker version", c.CheckDockerVersion)

	// Ensure that host directories exist.
	// If not using the nsenter mounter, create a volume share on the host machine to
	// mount OpenShift volumes.
	c.addTask("Creating host directories", c.EnsureHostDirectories)

	// Determine an IP to use for OpenShift. Uses the following sources:
	// - Docker host
	// - openshift start --print-ip
	// - hostname -I
	// Each IP is tested to ensure that it can be accessed from the current client
	c.addTask("Finding server IP", c.DetermineServerIP)

	// Create an OpenShift configuration and start a container that uses it.
	c.addTask("Starting OpenShift container", c.StartOpenShift)

	// Install a registry
	c.addTask("Installing registry", c.InstallRegistry)

	// Install a router
	c.addTask("Installing router", c.InstallRouter)

	// Install metrics
	if c.ShouldInstallMetrics {
		c.addTask("Install Metrics", c.InstallMetrics)
	}

	// Import default image streams
	c.addTask("Importing image streams", c.ImportImageStreams)

	// Import templates
	c.addTask("Importing templates", c.ImportTemplates)

	// Login with an initial default user
	c.addTask("Login to server", c.Login)

	// Create an initial project
	c.addTask(fmt.Sprintf("Creating initial project %q", initialProjectName), c.CreateProject)

	// Display server information
	c.addTask("Server Information", c.ServerInfo)

	return nil
}

// Validate validates that required fields in StartConfig have been populated
func (c *ClientStartConfig) Validate(out io.Writer) error {
	if len(c.Tasks) == 0 {
		return fmt.Errorf("no startup tasks to execute")
	}
	return nil
}

// Start runs the start tasks ensuring that they are executed in sequence
func (c *ClientStartConfig) Start(out io.Writer) error {
	for _, task := range c.Tasks {
		c.TaskPrinter.StartTask(task.name)
		w := c.TaskPrinter.TaskWriter()
		err := task.fn(w)
		if err != nil {
			c.TaskPrinter.Failure(err)
			return err
		}
		c.TaskPrinter.Success()
	}
	return nil
}

func defaultPortForwarding() bool {
	// Defaults to true if running on Mac, with no DOCKER_HOST defined
	return runtime.GOOS == "darwin" && len(os.Getenv("DOCKER_HOST")) == 0
}

const defaultDockerMachineName = "openshift"

func defaultImageVersion() string {
	return variable.OverrideVersion.LastSemanticVersion()
}

// CreateDockerMachine will create a new Docker machine to run OpenShift
func (c *ClientStartConfig) CreateDockerMachine(out io.Writer) error {
	if len(c.DockerMachine) == 0 {
		c.DockerMachine = defaultDockerMachineName
	}
	fmt.Fprintf(out, "Creating docker-machine %s\n", c.DockerMachine)
	return dockermachine.NewBuilder().Name(c.DockerMachine).Create()
}

// CheckOpenShiftClient ensures that the client can be configured
// for the new server
func (c *ClientStartConfig) CheckOpenShiftClient(out io.Writer) error {
	kubeConfig := os.Getenv("KUBECONFIG")
	if len(kubeConfig) == 0 {
		return nil
	}
	var (
		kubeConfigError error
		f               *os.File
	)
	_, err := os.Stat(kubeConfig)
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

// GetDockerClient will obtain a new Docker client from the environment or
// from a Docker machine, starting it if necessary
func (c *ClientStartConfig) GetDockerClient(out io.Writer) error {
	var err error

	if len(c.DockerMachine) > 0 {
		glog.V(2).Infof("Getting client for Docker machine %q", c.DockerMachine)
		c.dockerClient, c.engineAPIClient, err = getDockerMachineClient(c.DockerMachine, out)
		if err != nil {
			return errors.ErrNoDockerMachineClient(c.DockerMachine, err)
		}
		return nil
	}

	if glog.V(4) {
		dockerHost := os.Getenv("DOCKER_HOST")
		dockerTLSVerify := os.Getenv("DOCKER_TLS_VERIFY")
		dockerCertPath := os.Getenv("DOCKER_CERT_PATH")
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
	c.dockerClient, _, err = dockerutil.NewHelper().GetClient()
	if err != nil {
		return errors.ErrNoDockerClient(err)
	}
	// FIXME: Workaround for docker engine API client on OS X - sets the default to
	// the wrong DOCKER_HOST string
	if runtime.GOOS == "darwin" {
		dockerHost := os.Getenv("DOCKER_HOST")
		if len(dockerHost) == 0 {
			os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
		}
	}
	c.engineAPIClient, err = dockerclient.NewEnvClient()
	if err != nil {
		return errors.ErrNoDockerClient(err)
	}
	if err = c.dockerClient.Ping(); err != nil {
		return errors.ErrCannotPingDocker(err)
	}
	glog.V(4).Infof("Docker ping succeeded")
	return nil
}

// CheckExistingOpenShiftContainer checks the state of an OpenShift container. If one
// is already running, it throws an error. If one exists, it removes it so a new one
// can be created.
func (c *ClientStartConfig) CheckExistingOpenShiftContainer(out io.Writer) error {
	exists, running, err := c.DockerHelper().GetContainerState(openShiftContainer)
	if err != nil {
		return errors.NewError("unexpected error while checking OpenShift container state").WithCause(err)
	}
	if running {
		return errors.NewError("OpenShift is already running").WithSolution("To start OpenShift again, stop the current cluster:\n$ %s\n", cmdutil.SiblingCommand(c.command, "down"))
	}
	if exists {
		err = c.DockerHelper().RemoveContainer(openShiftContainer)
		if err != nil {
			return errors.NewError("cannot delete existing OpenShift container").WithCause(err)
		}
		fmt.Fprintf(out, "Deleted existing OpenShift container\n")
	}
	return nil
}

// CheckOpenShiftImage checks whether the OpenShift image exists. If not it tells the
// Docker daemon to pull it.
func (c *ClientStartConfig) CheckOpenShiftImage(out io.Writer) error {
	return c.DockerHelper().CheckAndPull(c.openShiftImage(), out)
}

// CheckDockerInsecureRegistry checks whether the Docker daemon is using the right --insecure-registry argument
func (c *ClientStartConfig) CheckDockerInsecureRegistry(out io.Writer) error {
	hasArg, err := c.DockerHelper().HasInsecureRegistryArg()
	if err != nil {
		return err
	}
	if !hasArg {
		return errors.ErrNoInsecureRegistryArgument()
	}
	return nil
}

// CheckNsenterMounter checks whether the Docker host can use the nsenter mounter from Kubernetes. Otherwise,
// a shared volume is needed in Docker
func (c *ClientStartConfig) CheckNsenterMounter(out io.Writer) error {
	var err error
	c.UseNsenterMount, err = c.HostHelper().CanUseNsenterMounter()
	if c.UseNsenterMount {
		fmt.Fprintf(out, "Using nsenter mounter for OpenShift volumes\n")
	} else {
		fmt.Fprintf(out, "Using Docker shared volumes for OpenShift volumes\n")
	}
	return err
}

// CheckDockerVersion checks that the appropriate Docker version is installed based on whether we are using the nsenter mounter
// or shared volumes for OpenShift
func (c *ClientStartConfig) CheckDockerVersion(io.Writer) error {
	var minDockerVersion semver.Version
	if c.UseNsenterMount {
		minDockerVersion = semver.MustParse("1.8.1")
	} else {
		minDockerVersion = semver.MustParse("1.10.0")
	}
	ver, err := c.DockerHelper().Version()
	if err != nil {
		return err
	}
	glog.V(5).Infof("Checking that docker version is at least %v", minDockerVersion)
	if ver.LT(minDockerVersion) {
		return fmt.Errorf("Docker version is %v, it needs to be %v", ver, minDockerVersion)
	}
	return nil
}

func (c *ClientStartConfig) EnsureHostDirectories(io.Writer) error {
	err := c.HostHelper().EnsureHostDirectories()
	if err != nil {
		return err
	}
	// A host volume share is not needed if using the nsenter mounter
	if c.UseNsenterMount {
		glog.V(5).Infof("Volume share is not needed when using nsenter mounter.")
		return nil
	}
	return c.HostHelper().EnsureVolumeShare()
}

// CheckAvailablePorts ensures that ports used by OpenShift are available on the Docker host
func (c *ClientStartConfig) CheckAvailablePorts(out io.Writer) error {
	err := c.OpenShiftHelper().TestPorts(openshift.DefaultPorts)
	if err == nil {
		c.DNSPort = openshift.DefaultDNSPort
		return nil
	}
	if !openshift.IsPortsNotAvailableErr(err) {
		return err
	}
	conflicts := openshift.UnavailablePorts(err)
	if len(conflicts) == 1 && conflicts[0] == openshift.DefaultDNSPort {
		err = c.OpenShiftHelper().TestPorts(openshift.PortsWithAlternateDNS)
		if err == nil {
			c.DNSPort = openshift.AlternateDNSPort
			fmt.Fprintf(out, "WARNING: Binding DNS on port %d instead of 53, which may be not be resolvable from all clients.\n", openshift.AlternateDNSPort)
			return nil
		}
	}
	return errors.NewError("a port needed by OpenShift is not available").WithCause(err)
}

// DetermineServerIP gets an appropriate IP address to communicate with the OpenShift server
func (c *ClientStartConfig) DetermineServerIP(out io.Writer) error {
	ip, err := c.determineIP(out)
	if err != nil {
		return errors.NewError("cannot determine a server IP to use").WithCause(err)
	}
	c.ServerIP = ip
	fmt.Fprintf(out, "Using %s as the server IP\n", ip)
	return nil
}

// StartOpenShift starts the OpenShift container
func (c *ClientStartConfig) StartOpenShift(out io.Writer) error {
	var err error
	opt := &openshift.StartOptions{
		ServerIP:          c.ServerIP,
		UseSharedVolume:   !c.UseNsenterMount,
		Images:            c.imageFormat(),
		HostVolumesDir:    c.HostVolumesDir,
		HostConfigDir:     c.HostConfigDir,
		HostDataDir:       c.HostDataDir,
		UseExistingConfig: c.UseExistingConfig,
		Environment:       c.Environment,
		LogLevel:          c.ServerLogLevel,
		DNSPort:           c.DNSPort,
		PortForwarding:    c.PortForwarding,
	}
	if c.ShouldInstallMetrics {
		opt.MetricsHost = openshift.MetricsHost(c.RoutingSuffix, c.ServerIP)
	}
	c.LocalConfigDir, err = c.OpenShiftHelper().Start(opt, out)
	return err
}

func (c *ClientStartConfig) imageFormat() string {
	return fmt.Sprintf("%s-${component}:%s", c.Image, c.ImageVersion)
}

// InstallRegistry installs the OpenShift registry on the server
func (c *ClientStartConfig) InstallRegistry(out io.Writer) error {
	_, kubeClient, err := c.Clients()
	if err != nil {
		return err
	}
	f, err := c.Factory()
	if err != nil {
		return err
	}
	return c.OpenShiftHelper().InstallRegistry(kubeClient, f, c.LocalConfigDir, c.imageFormat(), out)
}

// InstallRouter installs a default router on the server
func (c *ClientStartConfig) InstallRouter(out io.Writer) error {
	_, kubeClient, err := c.Clients()
	if err != nil {
		return err
	}
	f, err := c.Factory()
	if err != nil {
		return err
	}
	return c.OpenShiftHelper().InstallRouter(kubeClient, f, c.LocalConfigDir, c.imageFormat(), c.ServerIP, c.PortForwarding, out)
}

// ImportImageStreams imports default image streams into the server
// TODO: Use streams compiled into oc
func (c *ClientStartConfig) ImportImageStreams(out io.Writer) error {
	return c.importObjects(out, imageStreamLocations)
}

// ImportTemplates imports default templates into the server
// TODO: Use templates compiled into oc
func (c *ClientStartConfig) ImportTemplates(out io.Writer) error {
	return c.importObjects(out, templateLocations)
}

/*
// TODO: implement this
func (c *ClientStartConfig) InstallLogging() error {
	return nil
}
*/

func (c *ClientStartConfig) InstallMetrics(out io.Writer) error {
	f, err := c.Factory()
	if err != nil {
		return err
	}
	return c.OpenShiftHelper().InstallMetrics(f, openshift.MetricsHost(c.RoutingSuffix, c.ServerIP), c.Image, c.ImageVersion)
}

// Login logs into the new server and sets up a default user and project
func (c *ClientStartConfig) Login(out io.Writer) error {
	server := c.OpenShiftHelper().Master(c.ServerIP)
	return openshift.Login(initialUser, initialPassword, server, c.LocalConfigDir, c.originalFactory, c.command, out)
}

// CreateProject creates a new project for the current user
func (c *ClientStartConfig) CreateProject(out io.Writer) error {
	return openshift.CreateProject(initialProjectName, initialProjectDisplay, initialProjectDesc, "oc", out)

}

// ServerInfo displays server information after a successful start
func (c *ClientStartConfig) ServerInfo(out io.Writer) error {
	metricsInfo := ""
	if c.ShouldInstallMetrics {
		metricsInfo = fmt.Sprintf("The metrics service is available at:\n"+
			"    https://%s\n\n", openshift.MetricsHost(c.RoutingSuffix, c.ServerIP))
	}
	fmt.Fprintf(out, "OpenShift server started.\n"+
		"The server is accessible via web console at:\n"+
		"    %s\n\n%s"+
		"You are logged in as:\n"+
		"    User:     %s\n"+
		"    Password: %s\n\n"+
		"To login as administrator:\n"+
		"    oc login -u system:admin\n\n",
		c.OpenShiftHelper().Master(c.ServerIP),
		metricsInfo,
		initialUser,
		initialPassword)
	return nil
}

// Factory returns a command factory that works with OpenShift server's admin credentials
func (c *ClientStartConfig) Factory() (*clientcmd.Factory, error) {
	if c.factory == nil {
		cfg, err := kclientcmd.LoadFromFile(filepath.Join(c.LocalConfigDir, "master", "admin.kubeconfig"))
		if err != nil {
			return nil, err
		}
		overrides := &kclientcmd.ConfigOverrides{}
		if c.PortForwarding {
			overrides.ClusterInfo.Server = fmt.Sprintf("https://%s:8443", c.ServerIP)
		}
		defaultCfg := kclientcmd.NewDefaultClientConfig(*cfg, overrides)
		c.factory = clientcmd.NewFactory(defaultCfg)
	}
	return c.factory, nil
}

// Clients returns clients for OpenShift and Kube
func (c *ClientStartConfig) Clients() (*client.Client, *kclient.Client, error) {
	f, err := c.Factory()
	if err != nil {
		return nil, nil, err
	}
	return f.Clients()
}

// OpenShiftHelper returns a helper object to work with OpenShift on the server
func (c *ClientStartConfig) OpenShiftHelper() *openshift.Helper {
	if c.openShiftHelper == nil {
		c.openShiftHelper = openshift.NewHelper(c.dockerClient, c.HostHelper(), c.openShiftImage(), openShiftContainer, c.PublicHostname, c.RoutingSuffix)
	}
	return c.openShiftHelper
}

// HostHelper returns a helper object to check Host configuration
func (c *ClientStartConfig) HostHelper() *host.HostHelper {
	if c.hostHelper == nil {
		c.hostHelper = host.NewHostHelper(c.dockerClient, c.openShiftImage(), c.HostVolumesDir, c.HostConfigDir, c.HostDataDir)
	}
	return c.hostHelper
}

// DockerHelper returns a helper object to work with the Docker client
func (c *ClientStartConfig) DockerHelper() *dockerhelper.Helper {
	if c.dockerHelper == nil {
		c.dockerHelper = dockerhelper.NewHelper(c.dockerClient, c.engineAPIClient)
	}
	return c.dockerHelper
}

func (c *ClientStartConfig) importObjects(out io.Writer, locations map[string]string) error {
	f, err := c.Factory()
	if err != nil {
		return err
	}
	for name, location := range locations {
		glog.V(2).Infof("Importing %s from %s", name, location)
		err = openshift.ImportObjects(f, openShiftNamespace, location)
		if err != nil {
			return errors.NewError("cannot import %s", name).WithCause(err).WithDetails(c.OpenShiftHelper().OriginLog())
		}
	}
	return nil
}

func (c *ClientStartConfig) openShiftImage() string {
	return fmt.Sprintf("%s:%s", c.Image, c.ImageVersion)
}

func getDockerMachineClient(machine string, out io.Writer) (*docker.Client, *dockerclient.Client, error) {
	if !dockermachine.IsRunning(machine) {
		fmt.Fprintf(out, "Starting Docker machine '%s'\n", machine)
		err := dockermachine.Start(machine)
		if err != nil {
			return nil, nil, errors.NewError("cannot start Docker machine %q", machine).WithCause(err)
		}
		fmt.Fprintf(out, "Started Docker machine '%s'\n", machine)
	}
	return dockermachine.Client(machine)
}

func (c *ClientStartConfig) determineIP(out io.Writer) (string, error) {
	if ip := net.ParseIP(c.PublicHostname); ip != nil && !ip.IsUnspecified() {
		fmt.Fprintf(out, "Using public hostname IP %s as the host IP\n", ip)
		return ip.String(), nil
	}

	if len(c.DockerMachine) > 0 {
		// If a docker machine is specified, port forwarding will not be used
		c.PortForwarding = false
		glog.V(2).Infof("Using docker machine %q to determine server IP", c.DockerMachine)
		ip, err := dockermachine.IP(c.DockerMachine)
		if err != nil {
			return "", errors.NewError("Could not determine IP address").WithCause(err).WithSolution("Ensure that docker-machine is functional.")
		}
		fmt.Fprintf(out, "Using docker-machine IP %s as the host IP\n", ip)
		return ip, nil
	}

	// If using port-forwarding, find a local IP that can be used to communicate with the
	// Origin container
	if c.PortForwarding {
		ip4, err := cmdutil.DefaultLocalIP4()
		if err != nil {
			return "", errors.NewError("cannot determine local IP address").WithCause(err)
		}
		glog.V(2).Infof("Testing local IP %s", ip4.String())
		err = c.OpenShiftHelper().TestForwardedIP(ip4.String())
		if err == nil {
			return ip4.String(), nil
		}
		glog.V(2).Infof("Failed to use %s: %v", ip4.String(), err)
		otherIPs, err := cmdutil.AllLocalIP4()
		if err != nil {
			return "", errors.NewError("cannot find local IP addresses to test").WithCause(err)
		}
		for _, ip := range otherIPs {
			if ip.String() == ip4.String() {
				continue
			}
			err = c.OpenShiftHelper().TestForwardedIP(ip.String())
			if err == nil {
				return ip.String(), nil
			}
			glog.V(2).Infof("Failed to use %s: %v", ip.String(), err)
		}
		return "", errors.NewError("could not determine local IP address to use").WithCause(err)
	}

	// First, try to get the host from the DOCKER_HOST if communicating via tcp
	var err error
	ip := c.DockerHelper().HostIP()
	if ip != "" {
		glog.V(2).Infof("Testing Docker host IP (%s)", ip)
		if err = c.OpenShiftHelper().TestIP(ip); err == nil {
			return ip, nil
		}
	}
	glog.V(2).Infof("Cannot use the Docker host IP(%s): %v", ip, err)

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
