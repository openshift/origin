package start

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/server/origin/node"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/master/ports"
	"k8s.io/kubernetes/pkg/util/interrupt"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/common"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes/network"
	networkoptions "github.com/openshift/origin/pkg/cmd/server/kubernetes/network/options"
	utilflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/version"
)

type NetworkOptions struct {
	NodeArgs   *NodeArgs
	ExpireDays int

	ConfigFile string
	Output     io.Writer
}

var networkLong = templates.LongDesc(`
	Start node network components

	This command helps you launch node networking.  Running

	    %[1]s start network --config=<node-config>

	will start the network proxy and SDN plugins with given configuration file. The proxy will
	run in the foreground until you terminate the process.`)

// NewCommandStartNetwork provides a CLI handler for 'start network' command
func NewCommandStartNetwork(basename string, out, errout io.Writer) (*cobra.Command, *NetworkOptions) {
	options := &NetworkOptions{Output: out}

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Launch node network",
		Long:  fmt.Sprintf(networkLong, basename),
		Run: func(c *cobra.Command, args []string) {
			ch := make(chan struct{})
			interrupt.New(func(s os.Signal) {
				close(ch)
				fmt.Fprintf(errout, "interrupt: Gracefully shutting down ...\n")
				time.Sleep(200 * time.Millisecond)
				os.Exit(1)
			}).Run(func() error {
				options.Run(c, errout, args, ch)
				return nil
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the node configuration file to run from. When running from a configuration file, all other command-line arguments are ignored.")

	options.NodeArgs = NewDefaultNetworkArgs()
	options.NodeArgs.ListenArg.ListenAddr.DefaultPort = ports.ProxyHealthzPort
	BindNodeNetworkArgs(options.NodeArgs, flags, "")
	BindListenArg(options.NodeArgs.ListenArg, flags, "")
	BindImageFormatArgs(options.NodeArgs.ImageFormatArgs, flags, "")
	BindKubeConnectionArgs(options.NodeArgs.KubeConnectionArgs, flags, "")

	// autocompletion hints
	cmd.MarkFlagFilename("config", "yaml", "yml")

	return cmd, options
}

func (options *NetworkOptions) Run(c *cobra.Command, errout io.Writer, args []string, stopCh <-chan struct{}) {
	kcmdutil.CheckErr(options.Complete(c))
	kcmdutil.CheckErr(options.Validate(args))

	origin.StartProfiler()

	if err := options.StartNetwork(stopCh); err != nil {
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

func (o NetworkOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported for start network")
	}
	return nil
}

func (o NetworkOptions) Complete(cmd *cobra.Command) error {
	o.NodeArgs.NodeName = strings.ToLower(o.NodeArgs.NodeName)
	if len(o.ConfigFile) > 0 {
		o.NodeArgs.ConfigDir.Default(filepath.Dir(o.ConfigFile))
	}
	return nil
}

// StartNetwork starts the networking processes and then waits until the stop
// channel receives a message or is closed.
func (o NetworkOptions) StartNetwork(stopCh <-chan struct{}) error {
	if err := o.RunNetwork(stopCh); err != nil {
		return err
	}

	go daemon.SdNotify(false, "READY=1")
	<-stopCh
	return nil
}

// RunNetwork takes the network options and does the following:
// 1. Reads the fully specified node config.
// 2. Starts the node networking based on the fully specified config.
func (o NetworkOptions) RunNetwork(stopCh <-chan struct{}) error {
	nodeConfig, configFile, err := o.resolveNodeConfig()
	if err != nil {
		return err
	}

	// allow listen address to be overriden
	if addr := o.NodeArgs.ListenArg.ListenAddr; addr.Provided {
		nodeConfig.ServingInfo.BindAddress = addr.HostPort(o.NodeArgs.ListenArg.ListenAddr.DefaultPort)
	}
	// do a local resolution of node config DNS IP, supports bootstrapping cases
	if err := node.SetDNSIP(nodeConfig); err != nil {
		return err
	}

	var validationResults common.ValidationResults
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

	return StartNetwork(*nodeConfig, o.NodeArgs.Components, stopCh)
}

// resolveNodeConfig creates a new configuration on disk by reading from the master, reads
// the config file from disk if specified, or generates a new config from the incoming arguments.
// After this call returns without an error, config files will exist on disk. It also returns
// a string for messages indicating which config file contains the config.
func (o NetworkOptions) resolveNodeConfig() (*configapi.NodeConfig, string, error) {
	if len(o.ConfigFile) == 0 {
		return nil, "", fmt.Errorf("you must specify a configuration file with --config")
	}
	glog.V(2).Infof("Reading node configuration from %s", o.ConfigFile)
	cfg, err := configapilatest.ReadAndResolveNodeConfig(o.ConfigFile)
	return cfg, o.ConfigFile, err
}

// StartNetwork launches the node networking processes.
func StartNetwork(nodeConfig configapi.NodeConfig, components *utilflags.ComponentFlag, stopCh <-chan struct{}) error {
	glog.Infof("Starting node networking %s (%s)", nodeConfig.NodeName, version.Get().String())

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
		networkConfig.RunDNS(stopCh)
	}

	networkConfig.InternalKubeInformers.Start(stopCh)
	if networkConfig.NetworkInformers != nil {
		networkConfig.NetworkInformers.Start(stopCh)
	}

	return nil
}
