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
	"k8s.io/kubernetes/pkg/util/sysctl"

	osdn "github.com/openshift/openshift-sdn/plugins/osdn/ovs"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/version"
)

type NodeOptions struct {
	NodeArgs *NodeArgs

	ConfigFile string
	Output     io.Writer
}

const nodeLong = `
Start a node

This command helps you launch a node.  Running

  $ %[1]s start node --config=<node-config>

will start a node with given configuration file. The node will run in the
foreground until you terminate the process.`

// NewCommandStartNode provides a CLI handler for 'start node' command
func NewCommandStartNode(basename string, out io.Writer) (*cobra.Command, *NodeOptions) {
	options := &NodeOptions{Output: out}

	cmd := &cobra.Command{
		Use:   "node",
		Short: "Launch a node",
		Long:  fmt.Sprintf(nodeLong, basename),
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(); err != nil {
				fmt.Fprintln(c.Out(), kcmdutil.UsageError(c, err.Error()))
				return
			}
			if err := options.Validate(args); err != nil {
				fmt.Fprintln(c.Out(), kcmdutil.UsageError(c, err.Error()))
				return
			}

			startProfiler()

			if err := options.StartNode(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(c.Out(), "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(c.Out(), "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.ConfigFile, "config", "", "Location of the node configuration file to run from. When running from a configuration file, all other command-line arguments are ignored.")

	options.NodeArgs = NewDefaultNodeArgs()

	BindNodeArgs(options.NodeArgs, flags, "")
	BindListenArg(options.NodeArgs.ListenArg, flags, "")
	BindImageFormatArgs(options.NodeArgs.ImageFormatArgs, flags, "")
	BindKubeConnectionArgs(options.NodeArgs.KubeConnectionArgs, flags, "")

	// autocompletion hints
	cmd.MarkFlagFilename("config", "yaml", "yml")

	return cmd, options
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

	// if we are not starting up using a config file, run the argument validation
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
			glog.Warningf("%v", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return kerrors.NewInvalid(configapi.Kind("NodeConfig"), o.ConfigFile, validationResults.Errors)
	}

	_, kubeClientConfig, err := configapi.GetKubeClient(nodeConfig.MasterKubeConfig)
	if err != nil {
		return err
	}
	glog.Infof("Starting a node connected to %s", kubeClientConfig.Host)

	if err := StartNode(*nodeConfig); err != nil {
		return err
	}

	return nil
}

func (o NodeOptions) CreateNodeConfig() error {
	getSignerOptions := &admin.SignerCertOptions{
		CertFile:   admin.DefaultCertFilename(o.NodeArgs.MasterCertDir, "ca"),
		KeyFile:    admin.DefaultKeyFilename(o.NodeArgs.MasterCertDir, "ca"),
		SerialFile: admin.DefaultSerialFilename(o.NodeArgs.MasterCertDir, "ca"),
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

		APIServerURL:    masterAddr.String(),
		APIServerCAFile: getSignerOptions.CertFile,

		NodeClientCAFile: getSignerOptions.CertFile,
		Output:           o.Output,
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

func StartNode(nodeConfig configapi.NodeConfig) error {
	config, err := kubernetes.BuildKubernetesNodeConfig(nodeConfig)
	if err != nil {
		return err
	}
	glog.Infof("Starting node %s (%s)", config.KubeletServer.HostnameOverride, version.Get().String())

	// preconditions
	config.EnsureVolumeDir()
	config.EnsureDocker(docker.NewHelper())

	// async starts
	config.RunKubelet()
	config.RunSDN()
	config.RunProxy()

	// HACK: RunProxy resets bridge-nf-call-iptables from what openshift-sdn requires
	if config.SDNPlugin != nil {
		sdnPluginName := nodeConfig.NetworkConfig.NetworkPluginName
		if sdnPluginName == osdn.SingleTenantPluginName() || sdnPluginName == osdn.MultiTenantPluginName() {
			if err := sysctl.SetSysctl("net/bridge/bridge-nf-call-iptables", 0); err != nil {
				glog.Warningf("Could not set net.bridge.bridge-nf-call-iptables sysctl: %s", err)
			}
		}
	}

	return nil
}
