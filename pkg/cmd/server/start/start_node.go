package start

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/docker"
	utilflags "github.com/openshift/origin/pkg/cmd/util/flags"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/version"
)

type NodeOptions struct {
	NodeArgs *NodeArgs

	ConfigFile string
	Output     io.Writer
}

var nodeLong = templates.LongDesc(`
	Start a node

	This command helps you launch a node.  Running

	    %[1]s start node --config=<node-config>

	will start a node with given configuration file. The node will run in the
	foreground until you terminate the process.`)

// NewCommandStartNode provides a CLI handler for 'start node' command
func NewCommandStartNode(basename string, out, errout io.Writer) (*cobra.Command, *NodeOptions) {
	options := &NodeOptions{Output: out}

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

	options.NodeArgs = NewDefaultNodeArgs()

	BindNodeArgs(options.NodeArgs, flags, "", true)
	BindListenArg(options.NodeArgs.ListenArg, flags, "")
	BindImageFormatArgs(options.NodeArgs.ImageFormatArgs, flags, "")
	BindKubeConnectionArgs(options.NodeArgs.KubeConnectionArgs, flags, "")

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
	options.NodeArgs.Components = NewNetworkComponentFlag()
	BindNodeNetworkArgs(options.NodeArgs, flags, "")
	BindImageFormatArgs(options.NodeArgs.ImageFormatArgs, flags, "")
	BindKubeConnectionArgs(options.NodeArgs.KubeConnectionArgs, flags, "")

	// autocompletion hints
	cmd.MarkFlagFilename("config", "yaml", "yml")

	return cmd, options
}

func (options *NodeOptions) Run(c *cobra.Command, errout io.Writer, args []string) {
	kcmdutil.CheckErr(options.Complete())
	kcmdutil.CheckErr(options.Validate(args))

	startProfiler()

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

	if o.IsWriteConfigOnly() {
		if o.IsRunFromConfig() {
			return errors.New("--config may not be set if you're only writing the config")
		}
	}

	// if we are starting up using a config file, run no validations here
	if !o.IsRunFromConfig() {
		if err := o.NodeArgs.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (o NodeOptions) Complete() error {
	o.NodeArgs.NodeName = strings.ToLower(o.NodeArgs.NodeName)

	return nil
}

// StartNode calls RunNode and then waits forever
func (o NodeOptions) StartNode() error {
	if err := o.RunNode(); err != nil {
		return err
	}

	if o.IsWriteConfigOnly() {
		return nil
	}

	go daemon.SdNotify("READY=1")
	select {}
}

// RunNode takes the options and:
// 1.  Creates certs if needed
// 2.  Reads fully specified node config OR builds a fully specified node config from the args
// 3.  Writes the fully specified node config and exits if needed
// 4.  Starts the node based on the fully specified config
func (o NodeOptions) RunNode() error {
	if !o.IsRunFromConfig() || o.IsWriteConfigOnly() {
		glog.V(2).Infof("Generating node configuration")
		if err := o.CreateNodeConfig(); err != nil {
			return err
		}
	}

	if o.IsWriteConfigOnly() {
		return nil
	}

	var nodeConfig *configapi.NodeConfig
	var err error
	if o.IsRunFromConfig() {
		nodeConfig, err = configapilatest.ReadAndResolveNodeConfig(o.ConfigFile)
	} else {
		nodeConfig, err = o.NodeArgs.BuildSerializeableNodeConfig()
	}
	if err != nil {
		return err
	}

	validationResults := validation.ValidateNodeConfig(nodeConfig, nil)
	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("Warning: %v, node start will continue.", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return kerrors.NewInvalid(configapi.Kind("NodeConfig"), o.ConfigFile, validationResults.Errors)
	}

	if err := ValidateRuntime(nodeConfig, o.NodeArgs.Components); err != nil {
		return err
	}

	if err := StartNode(*nodeConfig, o.NodeArgs.Components); err != nil {
		return err
	}

	return nil
}

func (o NodeOptions) CreateNodeConfig() error {
	getSignerOptions := &admin.SignerCertOptions{
		CertFile:   admin.DefaultCertFilename(o.NodeArgs.MasterCertDir, admin.CAFilePrefix),
		KeyFile:    admin.DefaultKeyFilename(o.NodeArgs.MasterCertDir, admin.CAFilePrefix),
		SerialFile: admin.DefaultSerialFilename(o.NodeArgs.MasterCertDir, admin.CAFilePrefix),
	}

	var dnsIP string
	if len(o.NodeArgs.ClusterDNS) > 0 {
		dnsIP = o.NodeArgs.ClusterDNS.String()
	}

	masterAddr, err := o.NodeArgs.KubeConnectionArgs.GetKubernetesAddress(o.NodeArgs.DefaultKubernetesURL)
	if err != nil {
		return err
	}

	hostnames, err := o.NodeArgs.GetServerCertHostnames()
	if err != nil {
		return err
	}

	nodeConfigDir := o.NodeArgs.ConfigDir.Value()
	createNodeConfigOptions := admin.CreateNodeConfigOptions{
		SignerCertOptions: getSignerOptions,

		NodeConfigDir: nodeConfigDir,

		NodeName:            o.NodeArgs.NodeName,
		Hostnames:           hostnames.List(),
		VolumeDir:           o.NodeArgs.VolumeDir,
		ImageTemplate:       o.NodeArgs.ImageFormatArgs.ImageTemplate,
		AllowDisabledDocker: o.NodeArgs.AllowDisabledDocker,
		DNSDomain:           o.NodeArgs.ClusterDomain,
		DNSIP:               dnsIP,
		ListenAddr:          o.NodeArgs.ListenArg.ListenAddr,
		NetworkPluginName:   o.NodeArgs.NetworkPluginName,

		APIServerURL:     masterAddr.String(),
		APIServerCAFiles: []string{admin.DefaultCABundleFile(o.NodeArgs.MasterCertDir)},

		NodeClientCAFile: getSignerOptions.CertFile,
		Output:           cmdutil.NewGLogWriterV(3),
	}

	if err := createNodeConfigOptions.Validate(nil); err != nil {
		return err
	}
	if err := createNodeConfigOptions.CreateNodeFolder(); err != nil {
		return err
	}

	return nil
}

func (o NodeOptions) IsWriteConfigOnly() bool {
	return o.NodeArgs.ConfigDir.Provided()
}

func (o NodeOptions) IsRunFromConfig() bool {
	return (len(o.ConfigFile) > 0)
}

func StartNode(nodeConfig configapi.NodeConfig, components *utilflags.ComponentFlag) error {
	config, err := kubernetes.BuildKubernetesNodeConfig(nodeConfig, components.Enabled(ComponentProxy), components.Enabled(ComponentDNS))
	if err != nil {
		return err
	}

	if sdnapi.IsOpenShiftNetworkPlugin(config.KubeletServer.NetworkPluginName) {
		// TODO: SDN plugin depends on the Kubelet registering as a Node and doesn't retry cleanly,
		// and Kubelet also can't start the PodSync loop until the SDN plugin has loaded.
		if components.Enabled(ComponentKubelet) != components.Enabled(ComponentPlugins) {
			return fmt.Errorf("the SDN plugin must be run in the same process as the kubelet")
		}
	}

	if components.Enabled(ComponentKubelet) {
		glog.Infof("Starting node %s (%s)", config.KubeletServer.HostnameOverride, version.Get().String())
	} else {
		glog.Infof("Starting node networking %s (%s)", config.KubeletServer.HostnameOverride, version.Get().String())
	}

	_, kubeClientConfig, err := configapi.GetKubeClient(nodeConfig.MasterKubeConfig, nodeConfig.MasterClientConnectionOverrides)
	if err != nil {
		return err
	}
	glog.Infof("Connecting to API server %s", kubeClientConfig.Host)

	// preconditions
	if components.Enabled(ComponentKubelet) {
		config.EnsureKubeletAccess()
		config.EnsureVolumeDir()
		config.EnsureDocker(docker.NewHelper())
		config.EnsureLocalQuota(nodeConfig) // must be performed after EnsureVolumeDir
	}

	if components.Enabled(ComponentKubelet) {
		config.RunKubelet()
	}
	if components.Enabled(ComponentPlugins) {
		config.RunPlugin()
	}
	if components.Enabled(ComponentProxy) {
		config.RunProxy()
	}
	if components.Enabled(ComponentDNS) {
		config.RunDNS()
	}

	config.RunServiceStores(components.Enabled(ComponentProxy), components.Enabled(ComponentDNS))

	return nil
}
