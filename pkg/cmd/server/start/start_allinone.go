package start

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/util/flag"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/server/start/kubernetes"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

type AllInOneOptions struct {
	MasterOptions *MasterOptions

	NodeArgs *NodeArgs

	ExpireDays         int
	SignerExpireDays   int
	ConfigDir          flag.StringFlag
	NodeConfigFile     string
	PrintIP            bool
	ServiceNetworkCIDR string
	Output             io.Writer
}

var allInOneLong = templates.LongDesc(`
	Start an all-in-one server

	This command helps you launch an all-in-one server, which allows you to run all of the
	components of an enterprise Kubernetes system on a server with Docker. Running:

	    %[1]s start

	will start listening on all interfaces, launch an etcd server to store persistent
	data, and launch the Kubernetes system components. The server will run in the foreground until
	you terminate the process.  This command delegates to "%[1]s start master" and
	"%[1]s start node".

	Note: starting OpenShift without passing the --master address will attempt to find the IP
	address that will be visible inside running Docker containers. This is not always successful,
	so if you have problems tell OpenShift what public address it will be via --master=<ip>.

	You may also pass --etcd=<address> to connect to an external etcd server.

	You may also pass --kubeconfig=<path> to connect to an external Kubernetes cluster.`)

// NewCommandStartAllInOne provides a CLI handler for 'start' command
func NewCommandStartAllInOne(basename string, out, errout io.Writer) (*cobra.Command, *AllInOneOptions) {
	options := &AllInOneOptions{
		MasterOptions: &MasterOptions{
			Output: out,
		},
		ExpireDays:       crypto.DefaultCertificateLifetimeInDays,
		SignerExpireDays: crypto.DefaultCACertificateLifetimeInDays,
		Output:           out,
	}
	options.MasterOptions.DefaultsFromName(basename)

	cmds := &cobra.Command{
		Use:   "start",
		Short: "Launch all-in-one server",
		Long:  fmt.Sprintf(allInOneLong, basename),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete())
			kcmdutil.CheckErr(options.Validate(args))

			startProfiler()

			if err := options.StartAllInOne(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(errout, "error: Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(errout, "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatalf("Server could not start: %v", err)
			}
		},
	}
	cmds.SetOutput(out)

	flags := cmds.Flags()

	flags.Var(&options.ConfigDir, "write-config", "Directory to write an initial config into.  After writing, exit without starting the server.")
	flags.StringVar(&options.MasterOptions.ConfigFile, "master-config", "", "Location of the master configuration file to run from. When running from configuration files, all other command-line arguments are ignored.")
	flags.StringVar(&options.NodeConfigFile, "node-config", "", "Location of the node configuration file to run from. When running from configuration files, all other command-line arguments are ignored.")
	flags.BoolVar(&options.MasterOptions.CreateCertificates, "create-certs", true, "Indicates whether missing certs should be created.")
	flags.BoolVar(&options.PrintIP, "print-ip", false, "Print the IP that would be used if no master IP is specified and exit.")
	flags.StringVar(&options.ServiceNetworkCIDR, "portal-net", NewDefaultNetworkArgs().ServiceNetworkCIDR, "The CIDR string representing the network that portal/service IPs will be assigned from. This must not overlap with any IP ranges assigned to nodes for pods.")
	flags.IntVar(&options.ExpireDays, "expire-days", options.ExpireDays, "Validity of the certificates in days (defaults to 2 years). WARNING: extending this above default value is highly discouraged.")
	flags.IntVar(&options.SignerExpireDays, "signer-expire-days", options.SignerExpireDays, "Validity of the CA certificate in days (defaults to 5 years). WARNING: extending this above default value is highly discouraged.")

	masterArgs, nodeArgs, listenArg, imageFormatArgs, _ := GetAllInOneArgs()
	options.MasterOptions.MasterArgs, options.NodeArgs = masterArgs, nodeArgs

	BindMasterArgs(masterArgs, flags, "")
	BindNodeArgs(nodeArgs, flags, "", false)
	BindListenArg(listenArg, flags, "")
	BindImageFormatArgs(imageFormatArgs, flags, "")

	startMaster, _ := NewCommandStartMaster(basename, out, errout)
	startNode, _ := NewCommandStartNode(basename, out, errout)
	startNodeNetwork, _ := NewCommandStartNetwork(basename, out, errout)
	startEtcdServer, _ := NewCommandStartEtcdServer(RecommendedStartEtcdServerName, basename, out, errout)
	cmds.AddCommand(startMaster)
	cmds.AddCommand(startNode)
	cmds.AddCommand(startNodeNetwork)
	cmds.AddCommand(startEtcdServer)

	startKube := kubernetes.NewCommand("kubernetes", basename, out, errout)
	cmds.AddCommand(startKube)

	// autocompletion hints
	cmds.MarkFlagFilename("write-config")
	cmds.MarkFlagFilename("master-config", "yaml", "yml")
	cmds.MarkFlagFilename("node-config", "yaml", "yml")

	return cmds, options
}

// GetAllInOneArgs makes sure that the node and master args that should be shared, are shared
func GetAllInOneArgs() (*MasterArgs, *NodeArgs, *ListenArg, *ImageFormatArgs, *KubeConnectionArgs) {
	masterArgs := NewDefaultMasterArgs()
	masterArgs.StartAPI = true
	masterArgs.StartControllers = true
	masterArgs.OverrideConfig = func(config *configapi.MasterConfig) error {
		// use node DNS
		// config.DNSConfig = nil
		return nil
	}

	nodeArgs := NewDefaultNodeArgs()
	nodeArgs.Components.DefaultEnable(ComponentDNS)

	listenArg := NewDefaultListenArg()
	masterArgs.ListenArg = listenArg
	nodeArgs.ListenArg = listenArg

	imageFormatArgs := NewDefaultImageFormatArgs()
	masterArgs.ImageFormatArgs = imageFormatArgs
	nodeArgs.ImageFormatArgs = imageFormatArgs

	kubeConnectionArgs := NewDefaultKubeConnectionArgs()
	masterArgs.KubeConnectionArgs = kubeConnectionArgs
	nodeArgs.KubeConnectionArgs = kubeConnectionArgs

	return masterArgs, nodeArgs, listenArg, imageFormatArgs, kubeConnectionArgs
}

func (o AllInOneOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported for start")
	}

	if (len(o.MasterOptions.ConfigFile) == 0) != (len(o.NodeConfigFile) == 0) {
		return errors.New("--master-config and --node-config must both be specified or both be unspecified")
	}

	if o.IsRunFromConfig() && o.IsWriteConfigOnly() {
		return errors.New("--master-config and --node-config cannot be specified when --write-config is specified")
	}

	if len(o.ConfigDir.Value()) == 0 {
		return errors.New("config directory must have a value")
	}

	if len(o.NodeArgs.NodeName) == 0 {
		return errors.New("--hostname must have a value")
	}
	// if we are not starting up using a config file, run the argument validation
	if !o.IsRunFromConfig() {
		if err := o.MasterOptions.MasterArgs.Validate(); err != nil {
			return err
		}

		if err := o.NodeArgs.Validate(); err != nil {
			return err
		}

	}

	if len(o.MasterOptions.MasterArgs.KubeConnectionArgs.ClientConfigLoadingRules.ExplicitPath) != 0 {
		return errors.New("all-in-one cannot start with a remote Kubernetes server, start the master instead")
	}

	if o.ExpireDays < 0 {
		return errors.New("expire-days must be valid number of days")
	}
	if o.SignerExpireDays < 0 {
		return errors.New("signer-expire-days must be valid number of days")
	}

	return nil
}

func (o *AllInOneOptions) Complete() error {
	if o.ConfigDir.Provided() {
		o.MasterOptions.MasterArgs.ConfigDir.Set(path.Join(o.ConfigDir.Value(), "master"))
		o.NodeArgs.ConfigDir.Set(path.Join(o.ConfigDir.Value(), admin.DefaultNodeDir(o.NodeArgs.NodeName)))
	} else {
		o.ConfigDir.Default("openshift.local.config")
		o.MasterOptions.MasterArgs.ConfigDir.Default(path.Join(o.ConfigDir.Value(), "master"))
		o.NodeArgs.ConfigDir.Default(path.Join(o.ConfigDir.Value(), admin.DefaultNodeDir(o.NodeArgs.NodeName)))
	}

	o.MasterOptions.MasterArgs.NetworkArgs.NetworkPluginName = o.NodeArgs.NetworkPluginName
	o.MasterOptions.MasterArgs.NetworkArgs.ServiceNetworkCIDR = o.ServiceNetworkCIDR

	masterAddr, err := o.MasterOptions.MasterArgs.GetMasterAddress()
	if err != nil {
		return err
	}
	// in the all-in-one, default kubernetes URL to the master's address
	o.NodeArgs.DefaultKubernetesURL = masterAddr
	o.NodeArgs.NodeName = strings.ToLower(o.NodeArgs.NodeName)
	o.NodeArgs.MasterCertDir = o.MasterOptions.MasterArgs.ConfigDir.Value()

	// For backward compatibility of DNS queries to the master service IP, enabling node DNS
	// continues to start the master DNS, but the container DNS server will be the node's.
	// However, if the user has provided an override DNSAddr, we need to honor the value if
	// the port is not 53 and we do that by disabling node DNS.
	if !o.IsRunFromConfig() && o.NodeArgs.Components.Enabled(ComponentDNS) {
		dnsAddr := &o.MasterOptions.MasterArgs.DNSBindAddr

		if dnsAddr.Provided {
			if dnsAddr.Port == 53 {
				// the user has set the DNS port to 53, which is the effective default (node on 53, master on 8053)
				o.NodeArgs.DNSBindAddr = dnsAddr.URL.Host
				dnsAddr.Port = 8053
				dnsAddr.URL.Host = net.JoinHostPort(dnsAddr.Host, strconv.Itoa(dnsAddr.Port))
			} else {
				// if the user set the DNS port to anything but 53, disable node DNS since ClusterDNS (and glibc)
				// can't look up DNS on anything other than 53, so we'll continue to use the proxy.
				o.NodeArgs.Components.Disable(ComponentDNS)
				glog.V(2).Infof("Node DNS may not be used with a non-standard DNS port %d - disabled node DNS", dnsAddr.Port)
			}
		}

		// if node DNS is still enabled, then default the node cluster DNS to a reachable master address
		if o.NodeArgs.Components.Enabled(ComponentDNS) {
			if o.NodeArgs.ClusterDNS == nil {
				if dnsIP, err := findLocalIPForDNS(o.MasterOptions.MasterArgs); err == nil {
					o.NodeArgs.ClusterDNS = dnsIP
					if len(o.NodeArgs.DNSBindAddr) == 0 {
						o.NodeArgs.DNSBindAddr = net.JoinHostPort(dnsIP.String(), "53")
					}
				} else {
					glog.V(2).Infof("Unable to find a local address to report as the node DNS - not using node DNS: %v", err)
				}
			}
		}
	}

	return nil
}

// StartAllInOne:
// 1.  Creates the signer certificate if needed
// 2.  Calls RunMaster
// 3.  Calls RunNode
// 4.  If only writing configs, it exits
// 5.  Waits forever
func (o AllInOneOptions) StartAllInOne() error {
	if o.PrintIP {
		host, _, err := net.SplitHostPort(o.NodeArgs.DefaultKubernetesURL.Host)
		if err != nil {
			return err
		}
		fmt.Fprintf(o.Output, "%s\n", host)
		return nil
	}
	masterOptions := *o.MasterOptions
	masterOptions.ExpireDays = o.ExpireDays
	masterOptions.SignerExpireDays = o.SignerExpireDays
	if err := masterOptions.RunMaster(); err != nil {
		return err
	}

	nodeOptions := NodeOptions{
		NodeArgs:   o.NodeArgs,
		ExpireDays: o.ExpireDays,
		ConfigFile: o.NodeConfigFile,
		Output:     o.MasterOptions.Output,
	}
	if err := nodeOptions.RunNode(); err != nil {
		return err
	}

	if o.IsWriteConfigOnly() {
		return nil
	}

	daemon.SdNotify(false, "READY=1")
	select {}
}

func startProfiler() {
	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			runtime.SetBlockProfileRate(1)
			profilePort := cmdutil.Env("OPENSHIFT_PROFILE_PORT", "6060")
			profileHost := cmdutil.Env("OPENSHIFT_PROFILE_HOST", "127.0.0.1")
			glog.Infof(fmt.Sprintf("Starting profiling endpoint at http://%s:%s/debug/pprof/", profileHost, profilePort))
			glog.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", profileHost, profilePort), nil))
		}()
	}
}

func (o AllInOneOptions) IsWriteConfigOnly() bool {
	return o.ConfigDir.Provided()
}

func (o AllInOneOptions) IsRunFromConfig() bool {
	return (len(o.MasterOptions.ConfigFile) > 0) && (len(o.NodeConfigFile) > 0)
}
