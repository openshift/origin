package start

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/pflag"

	"k8s.io/apiserver/pkg/util/flag"
	"k8s.io/kubernetes/pkg/master/ports"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	utilflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

const (
	ComponentProxy   = "proxy"
	ComponentDNS     = "dns"
	ComponentPlugins = "plugins"
	ComponentKubelet = "kubelet"
)

// NewNodeComponentFlag returns a flag capable of handling enabled components for the network
func NewNetworkComponentFlag() *utilflags.ComponentFlag {
	return utilflags.NewComponentFlag(nil, ComponentProxy, ComponentPlugins, ComponentDNS)
}

// NodeArgs is a struct that the command stores flag values into.  It holds a partially complete set of parameters for starting a node.
// This object should hold the common set values, but not attempt to handle all cases.  The expected path is to use this object to create
// a fully specified config later on.  If you need something not set here, then create a fully specified config file and pass that as argument
// to starting the master.
type NodeArgs struct {
	// Components is the set of enabled components.
	Components *utilflags.ComponentFlag

	// NodeName is the hostname to identify this node with the master.
	NodeName string

	MasterCertDir string
	ConfigDir     flag.StringFlag

	DefaultKubernetesURL *url.URL
	ClusterDomain        string
	ClusterDNS           net.IP
	// DNSBindAddr is provided for the all-in-one start only and is not exposed via a flag
	DNSBindAddr string
	// RecursiveResolvConf
	RecursiveResolvConf string

	// NetworkPluginName is the network plugin to be called for configuring networking for pods.
	NetworkPluginName string

	ListenArg          *ListenArg
	ImageFormatArgs    *ImageFormatArgs
	KubeConnectionArgs *KubeConnectionArgs
}

// BindNodeNetworkArgs binds the options to the flags with prefix + default flag names
func BindNodeNetworkArgs(args *NodeArgs, flags *pflag.FlagSet, prefix string) {
	args.Components.Bind(flags, "%s", "The set of network components to")

	flags.StringVar(&args.RecursiveResolvConf, prefix+"recursive-resolv-conf", args.RecursiveResolvConf, "An optional upstream resolv.conf that will override the DNS config.")

	flags.StringVar(&args.NetworkPluginName, prefix+"network-plugin", args.NetworkPluginName, "The network plugin to be called for configuring networking for pods.")
}

// NewDefaultNetworkArgs creates NodeArgs with sub-objects created and default values set.
func NewDefaultNetworkArgs() *NodeArgs {
	hostname, err := defaultHostname()
	if err != nil {
		hostname = "localhost"
	}

	config := &NodeArgs{
		Components: NewNetworkComponentFlag(),

		NodeName: hostname,

		ClusterDomain: cmdutil.Env("OPENSHIFT_DNS_DOMAIN", "cluster.local"),

		MasterCertDir: "openshift.local.config/master",

		NetworkPluginName: "",

		ListenArg:          NewDefaultListenArg(),
		ImageFormatArgs:    NewDefaultImageFormatArgs(),
		KubeConnectionArgs: NewDefaultKubeConnectionArgs(),
	}
	config.ConfigDir.Default("openshift.local.config/node")

	return config
}

func (args NodeArgs) Validate() error {
	if err := args.KubeConnectionArgs.Validate(); err != nil {
		return err
	}
	if addr, _ := args.KubeConnectionArgs.GetKubernetesAddress(args.DefaultKubernetesURL); addr == nil {
		return errors.New("--kubeconfig must be set to provide API server connection information")
	}
	return nil
}

func ValidateRuntime(config *configapi.NodeConfig, components *utilflags.ComponentFlag) error {
	actual, err := components.Validate()
	if err != nil {
		return err
	}
	if actual.Len() == 0 {
		return fmt.Errorf("at least one node component must be enabled (%s)", strings.Join(components.Allowed().List(), ", "))
	}
	return nil
}

// MergeSerializeableNodeConfig takes the NodeArgs (partially complete config) and overlays them onto an existing
// config. Only a subset of node args are allowed to override this config - those that may reasonably be specified
// as local overrides.
func (args NodeArgs) MergeSerializeableNodeConfig(config *configapi.NodeConfig) error {
	if len(args.NodeName) > 0 {
		config.NodeName = args.NodeName
	}
	if args.ListenArg.ListenAddr.Provided {
		config.ServingInfo.BindAddress = net.JoinHostPort(args.ListenArg.ListenAddr.Host, strconv.Itoa(ports.KubeletPort))
	}
	return nil
}

// defaultHostname returns the default hostname for this system.
func defaultHostname() (string, error) {
	// Note: We use exec here instead of os.Hostname() because we
	// want the FQDN, and this is the easiest way to get it.
	fqdn, err := exec.Command("uname", "-n").Output()
	if err != nil {
		return "", fmt.Errorf("Couldn't determine hostname: %v", err)
	}
	return strings.ToLower(strings.TrimSpace(string(fqdn))), nil
}
