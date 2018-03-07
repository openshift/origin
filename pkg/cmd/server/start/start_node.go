package start

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/master/ports"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes/network"
	networkoptions "github.com/openshift/origin/pkg/cmd/server/kubernetes/network/options"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes/node"
	nodeoptions "github.com/openshift/origin/pkg/cmd/server/kubernetes/node/options"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	utilflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/version"
)

type NodeOptions struct {
	NodeArgs   *NodeArgs
	ExpireDays int

	ConfigFile string
	Output     io.Writer
}

var nodeLong = templates.LongDesc(`
	Start a node

	This command helps you launch a node.  Running

	    %[1]s start node --config=<node-config>

	will start a node with given configuration file. The node will run in the
	foreground until you terminate the process.
	
	The --bootstrap-config-name flag instructs the node to use the provided 
	kubeconfig file to contact the master and request a client cert (its identity) and
	a serving cert, and then downloads node-config.yaml from the named config map. 
	If no config map exists in the openshift-node namespace the node will exit with
	an error. In this mode --config will be location of the downloaded config. 
	Turning	on bootstrapping will always use certificate rotation by default at the
	master's preferred rotation interval.
	`)

// NewCommandStartNode provides a CLI handler for 'start node' command
func NewCommandStartNode(basename string, out, errout io.Writer) (*cobra.Command, *NodeOptions) {
	options := &NodeOptions{
		ExpireDays: crypto.DefaultCertificateLifetimeInDays,
		Output:     out,
	}

	cmd := &cobra.Command{
		Use:   "node",
		Short: "Launch a node",
		Long:  fmt.Sprintf(nodeLong, basename),
		Run: func(c *cobra.Command, args []string) {
			options.Run(c, errout, args)
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.ConfigFile, "config", "", "Location of the node configuration file to run from. When running from a configuration file, all other command-line arguments are ignored.")
	flags.IntVar(&options.ExpireDays, "expire-days", options.ExpireDays, "Validity of the certificates in days (defaults to 2 years). WARNING: extending this above default value is highly discouraged.")

	options.NodeArgs = NewDefaultNodeArgs()
	BindNodeArgs(options.NodeArgs, flags, "", true)
	BindListenArg(options.NodeArgs.ListenArg, flags, "")
	BindImageFormatArgs(options.NodeArgs.ImageFormatArgs, flags, "")
	BindKubeConnectionArgs(options.NodeArgs.KubeConnectionArgs, flags, "")

	flags.BoolVar(&options.NodeArgs.WriteFlagsOnly, "write-flags", false, "When this is specified only the arguments necessary to start the Kubelet will be output.")
	flags.StringVar(&options.NodeArgs.BootstrapConfigName, "bootstrap-config-name", options.NodeArgs.BootstrapConfigName, "On startup, the node will request a client cert from the master and get its config from this config map in the openshift-node namespace (experimental).")

	// autocompletion hints
	cmd.MarkFlagFilename("config", "yaml", "yml")

	return cmd, options
}

var networkLong = templates.LongDesc(`
	Start node network components

	This command helps you launch node networking.  Running

	    %[1]s start network --config=<node-config>

	will start the network proxy and SDN plugins with given configuration file. The proxy will
	run in the foreground until you terminate the process.`)

// NewCommandStartNetwork provides a CLI handler for 'start network' command
func NewCommandStartNetwork(basename string, out, errout io.Writer) (*cobra.Command, *NodeOptions) {
	options := &NodeOptions{Output: out}

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Launch node network",
		Long:  fmt.Sprintf(networkLong, basename),
		Run: func(c *cobra.Command, args []string) {
			options.Run(c, errout, args)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the node configuration file to run from. When running from a configuration file, all other command-line arguments are ignored.")

	options.NodeArgs = NewDefaultNodeArgs()
	options.NodeArgs.ListenArg.ListenAddr.DefaultPort = ports.ProxyHealthzPort
	options.NodeArgs.Components = NewNetworkComponentFlag()
	BindNodeNetworkArgs(options.NodeArgs, flags, "")
	BindListenArg(options.NodeArgs.ListenArg, flags, "")
	BindImageFormatArgs(options.NodeArgs.ImageFormatArgs, flags, "")
	BindKubeConnectionArgs(options.NodeArgs.KubeConnectionArgs, flags, "")

	// autocompletion hints
	cmd.MarkFlagFilename("config", "yaml", "yml")

	return cmd, options
}

func (options *NodeOptions) Run(c *cobra.Command, errout io.Writer, args []string) {
	kcmdutil.CheckErr(options.Complete(c))
	kcmdutil.CheckErr(options.Validate(args))

	origin.StartProfiler()

	if err := options.StartNode(); err != nil {
		if kerrors.IsInvalid(err) {
			if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
				fmt.Fprintf(errout, "Invalid %s %s\n", details.Kind, details.Name)
				for _, cause := range details.Causes {
					fmt.Fprintf(errout, "  %s: %s\n", cause.Field, cause.Message)
				}
				os.Exit(255)
			}
		}
		glog.Fatal(err)
	}
}

func (o NodeOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported for start node")
	}

	if o.ExpireDays < 0 {
		return errors.New("expire-days must be valid number of days")
	}

	if o.IsWriteConfigOnly() {
		if o.IsRunFromConfig() {
			return errors.New("--config may not be set if you're only writing the config")
		}
		if o.NodeArgs.WriteFlagsOnly {
			return errors.New("--write-config and --write-flags are mutually exclusive")
		}
	}

	// if we are starting up using a config file, run no validations here
	if len(o.NodeArgs.BootstrapConfigName) > 0 {
		if o.NodeArgs.WriteFlagsOnly {
			return errors.New("--write-flags is mutually exclusive with --bootstrap-config-name")
		}
		if !o.IsRunFromConfig() {
			if err := o.NodeArgs.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (o NodeOptions) Complete(cmd *cobra.Command) error {
	o.NodeArgs.NodeName = strings.ToLower(o.NodeArgs.NodeName)
	if len(o.ConfigFile) > 0 {
		o.NodeArgs.ConfigDir.Default(filepath.Dir(o.ConfigFile))
	}
	if flag := cmd.Flags().Lookup("volume-dir"); flag != nil {
		o.NodeArgs.VolumeDirProvided = flag.Changed
	}
	return nil
}

// StartNode calls RunNode and then waits forever
func (o NodeOptions) StartNode() error {
	if err := o.RunNode(); err != nil {
		return err
	}

	if o.NodeArgs.WriteFlagsOnly || o.IsWriteConfigOnly() {
		return nil
	}

	go daemon.SdNotify(false, "READY=1")
	select {}
}

// RunNode takes the options and:
// 1.  Creates certs if needed
// 2.  Reads fully specified node config OR builds a fully specified node config from the args
// 3.  Writes the fully specified node config and exits if needed
// 4.  Starts the node based on the fully specified config
func (o NodeOptions) RunNode() error {
	nodeConfig, configFile, err := o.resolveNodeConfig()
	if err != nil {
		return err
	}

	// allow listen address to be overriden
	if addr := o.NodeArgs.ListenArg.ListenAddr; addr.Provided {
		nodeConfig.ServingInfo.BindAddress = addr.HostPort(o.NodeArgs.ListenArg.ListenAddr.DefaultPort)
	}
	// do a local resolution of node config DNS IP, supports bootstrapping cases
	if nodeConfig.DNSIP == "0.0.0.0" {
		glog.V(4).Infof("Defaulting to the DNSIP config to the node's IP")
		nodeConfig.DNSIP = nodeConfig.NodeIP
		// TODO: the Kubelet should do this defaulting (to the IP it recognizes)
		if len(nodeConfig.DNSIP) == 0 {
			if ip, err := cmdutil.DefaultLocalIP4(); err == nil {
				nodeConfig.DNSIP = ip.String()
			}
		}
	}

	var validationResults validation.ValidationResults
	switch {
	case o.NodeArgs.Components.Calculated().Equal(NewNetworkComponentFlag().Calculated()):
		if len(nodeConfig.NodeName) == 0 {
			nodeConfig.NodeName = o.NodeArgs.NodeName
		}
		nodeConfig.MasterKubeConfig = o.NodeArgs.KubeConnectionArgs.ClientConfigLoadingRules.ExplicitPath
		validationResults = validation.ValidateInClusterNodeConfig(nodeConfig, nil)
	default:
		validationResults = validation.ValidateNodeConfig(nodeConfig, nil)
	}

	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("Warning: %v, node start will continue.", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		glog.V(4).Infof("Configuration is invalid: %#v", nodeConfig)
		return kerrors.NewInvalid(configapi.Kind("NodeConfig"), configFile, validationResults.Errors)
	}

	if err := ValidateRuntime(nodeConfig, o.NodeArgs.Components); err != nil {
		glog.V(4).Infof("Unable to validate runtime configuration: %v", err)
		return err
	}

	if o.IsWriteConfigOnly() {
		return nil
	}

	if o.NodeArgs.WriteFlagsOnly {
		return WriteKubeletFlags(*nodeConfig)
	}

	return StartNode(*nodeConfig, o.NodeArgs.Components)
}

// resolveNodeConfig creates a new configuration on disk by reading from the master, reads
// the config file from disk if specified, or generates a new config from the incoming arguments.
// After this call returns without an error, config files will exist on disk. It also returns
// a string for messages indicating which config file contains the config.
func (o NodeOptions) resolveNodeConfig() (*configapi.NodeConfig, string, error) {
	switch {
	case len(o.NodeArgs.BootstrapConfigName) > 0:
		glog.V(2).Infof("Bootstrapping from master configuration")

		nodeConfigDir := o.NodeArgs.ConfigDir.Value()
		if err := o.loadBootstrap(nodeConfigDir); err != nil {
			return nil, "", err
		}
		configFile := o.ConfigFile
		if len(configFile) == 0 {
			configFile = filepath.Join(o.NodeArgs.ConfigDir.Value(), "node-config.yaml")
		}
		cfg, err := configapilatest.ReadAndResolveNodeConfig(configFile)
		return cfg, configFile, err

	case o.IsRunFromConfig():
		glog.V(2).Infof("Reading node configuration from %s", o.ConfigFile)
		cfg, err := configapilatest.ReadAndResolveNodeConfig(o.ConfigFile)
		return cfg, o.ConfigFile, err

	default:
		glog.V(2).Infof("Generating new node configuration")
		configFile, err := o.createNodeConfig()
		if err != nil {
			return nil, "", err
		}
		cfg, err := configapilatest.ReadAndResolveNodeConfig(configFile)
		if err != nil {
			return nil, "", err
		}
		return cfg, configFile, err
	}
}

// createNodeConfig writes the appropriate config file to the ConfigDir location and then
// returns the path to that config file or an error.
func (o NodeOptions) createNodeConfig() (string, error) {
	hostnames, err := o.NodeArgs.GetServerCertHostnames()
	if err != nil {
		return "", err
	}
	nodeConfigDir := o.NodeArgs.ConfigDir.Value()
	var dnsIP string
	if len(o.NodeArgs.ClusterDNS) > 0 {
		dnsIP = o.NodeArgs.ClusterDNS.String()
	}
	masterAddr, err := o.NodeArgs.KubeConnectionArgs.GetKubernetesAddress(o.NodeArgs.DefaultKubernetesURL)
	if err != nil {
		return "", err
	}
	if masterAddr == nil {
		return "", errors.New("--kubeconfig must be set to provide API server connection information")
	}

	getSignerOptions := &admin.SignerCertOptions{
		CertFile:   admin.DefaultCertFilename(o.NodeArgs.MasterCertDir, admin.CAFilePrefix),
		KeyFile:    admin.DefaultKeyFilename(o.NodeArgs.MasterCertDir, admin.CAFilePrefix),
		SerialFile: admin.DefaultSerialFilename(o.NodeArgs.MasterCertDir, admin.CAFilePrefix),
	}
	createNodeConfigOptions := admin.CreateNodeConfigOptions{
		SignerCertOptions: getSignerOptions,

		NodeConfigDir: nodeConfigDir,

		NodeName:            o.NodeArgs.NodeName,
		Hostnames:           hostnames.List(),
		VolumeDir:           o.NodeArgs.VolumeDir,
		ImageTemplate:       o.NodeArgs.ImageFormatArgs.ImageTemplate,
		AllowDisabledDocker: o.NodeArgs.AllowDisabledDocker,
		DNSBindAddress:      o.NodeArgs.DNSBindAddr,
		DNSDomain:           o.NodeArgs.ClusterDomain,
		DNSIP:               dnsIP,
		DNSRecursiveResolvConf: o.NodeArgs.RecursiveResolvConf,
		ListenAddr:             o.NodeArgs.ListenArg.ListenAddr,
		NetworkPluginName:      o.NodeArgs.NetworkPluginName,

		APIServerURL:     masterAddr.String(),
		APIServerCAFiles: []string{admin.DefaultCABundleFile(o.NodeArgs.MasterCertDir)},

		KubeletArguments: map[string][]string{
			"fail-swap-on": {"false"},
		},

		NodeClientCAFile: getSignerOptions.CertFile,
		ExpireDays:       o.ExpireDays,
		Output:           cmdutil.NewGLogWriterV(3),
	}

	if err := createNodeConfigOptions.Validate(nil); err != nil {
		return "", err
	}
	return createNodeConfigOptions.CreateNodeFolder()
}

func (o NodeOptions) IsWriteConfigOnly() bool {
	return o.NodeArgs.ConfigDir.Provided()
}

func (o NodeOptions) IsRunFromConfig() bool {
	return (len(o.ConfigFile) > 0)
}

// execKubelet attempts to call execve() for the kubelet with the configuration defined
// in server passed as flags.
func execKubelet(kubeletArgs []string) error {
	path := "kubelet"
	kubeletPath, err := exec.LookPath(path)
	if err != nil {
		return err
	}
	// convert current settings to flags
	args := append([]string{kubeletPath}, kubeletArgs...)
	for i := glog.Level(10); i > 0; i-- {
		if glog.V(i) {
			args = append(args, fmt.Sprintf("--v=%d", i))
			break
		}
	}
	for i, s := range os.Args {
		if s == "--vmodule" {
			if i+1 < len(os.Args) {
				args = append(args, fmt.Sprintf("--vmodule=", os.Args[i+1]))
				break
			}
		}
		if strings.HasPrefix(s, "--vmodule=") {
			args = append(args, s)
			break
		}
	}
	// execve the child process, replacing this process
	glog.V(3).Infof("Exec %s %s", kubeletPath, strings.Join(args, " "))
	return syscall.Exec(kubeletPath, args, os.Environ())
}

// safeArgRegexp matches only characters that are known safe. DO NOT add to this list
// without fully considering whether that new character can be used to break shell escaping
// rules.
var safeArgRegexp = regexp.MustCompile(`^[\da-zA-Z\-=_\.,/\:]+$`)

// shellEscapeArg quotes an argument if it contains characters that my cause a shell
// interpreter to split the single argument into multiple.
func shellEscapeArg(s string) string {
	if safeArgRegexp.MatchString(s) {
		return s
	}
	return strconv.Quote(s)
}

// WriteKubeletFlags writes the correct set of flags to start a Kubelet from the provided node config to
// stdout, instead of launching anything.
func WriteKubeletFlags(nodeConfig configapi.NodeConfig) error {
	kubeletArgs, err := nodeoptions.ComputeKubeletFlags(nodeConfig.KubeletArguments, nodeConfig)
	if err != nil {
		return fmt.Errorf("cannot create kubelet args: %v", err)
	}
	if err := nodeoptions.CheckFlags(kubeletArgs); err != nil {
		return err
	}
	var outputArgs []string
	for _, s := range kubeletArgs {
		outputArgs = append(outputArgs, shellEscapeArg(s))
	}
	fmt.Println(strings.Join(outputArgs, " "))
	return nil
}

// StartNode launches the node processes.
func StartNode(nodeConfig configapi.NodeConfig, components *utilflags.ComponentFlag) error {
	kubeletArgs, err := nodeoptions.ComputeKubeletFlags(nodeConfig.KubeletArguments, nodeConfig)
	if err != nil {
		return fmt.Errorf("cannot create kubelet args: %v", err)
	}
	if err := nodeoptions.CheckFlags(kubeletArgs); err != nil {
		return err
	}

	// as a step towards decomposing OpenShift into Kubernetes components, perform an execve
	// to launch the Kubelet instead of loading in-process
	if components.Calculated().Equal(sets.NewString(ComponentKubelet)) {
		if err := execKubelet(kubeletArgs); err != nil {
			utilruntime.HandleError(fmt.Errorf("Unable to call exec on kubelet, continuing with normal startup: %v", err))
		}
	}

	if components.Enabled(ComponentKubelet) {
		glog.Infof("Starting node %s (%s)", nodeConfig.NodeName, version.Get().String())

		// TODO this is a best effort check at the moment that should either move to kubelet or be removed entirely
		node.EnsureKubeletAccess()
		// TODO perform this "ensure" in ansible and skip it entirely.
		node.EnsureVolumeDir(nodeConfig.VolumeDirectory)
		// TODO accept an --openshift-config in our fork.  This overwrites the volume creation patch for the node.
		kubeletapp.ProbeVolumePlugins = node.PatchUpstreamVolumePluginsForLocalQuota(nodeConfig)

		go func() {
			glog.Fatal(runKubeletInProcess(kubeletArgs))
		}()

	} else {
		glog.Infof("Starting node networking %s (%s)", nodeConfig.NodeName, version.Get().String())
	}

	proxyConfig, err := networkoptions.Build(nodeConfig)
	if err != nil {
		glog.V(4).Infof("Unable to build network options: %v", err)
		return err
	}
	clusterDomain := nodeConfig.DNSDomain
	if len(nodeConfig.KubeletArguments["cluster-domain"]) > 0 {
		clusterDomain = nodeConfig.KubeletArguments["cluster-domain"][0]
	}
	networkConfig, err := network.New(nodeConfig, clusterDomain, proxyConfig, components.Enabled(ComponentProxy), components.Enabled(ComponentDNS) && len(nodeConfig.DNSBindAddress) > 0)
	if err != nil {
		glog.V(4).Infof("Unable to initialize network configuration: %v", err)
		return err
	}

	if components.Enabled(ComponentPlugins) {
		networkConfig.RunSDN()
	}
	if components.Enabled(ComponentProxy) {
		networkConfig.RunProxy()
	}
	if components.Enabled(ComponentDNS) && networkConfig.DNSServer != nil {
		networkConfig.RunDNS()
	}

	networkConfig.InternalKubeInformers.Start(wait.NeverStop)
	if networkConfig.InternalNetworkInformers != nil {
		networkConfig.InternalNetworkInformers.Start(wait.NeverStop)
	}

	return nil
}

// runKubeletInProcess runs the kubelet command using the provide args
func runKubeletInProcess(kubeletArgs []string) error {
	cmd := kubeletapp.NewKubeletCommand()
	if err := cmd.ParseFlags(kubeletArgs); err != nil {
		return err
	}
	glog.Infof("kubelet %v", kubeletArgs)
	cmd.Run(nil, nil)
	return nil
}
