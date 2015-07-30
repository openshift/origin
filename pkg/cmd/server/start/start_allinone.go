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
	"strings"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/start/kubernetes"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

type AllInOneOptions struct {
	MasterArgs *MasterArgs
	NodeArgs   *NodeArgs

	CreateCerts      bool
	ConfigDir        util.StringFlag
	MasterConfigFile string
	NodeConfigFile   string
	PrintIP          bool
	Output           io.Writer
	DisabledFeatures []string
}

const allInOneLong = `
Start an all-in-one server

This command helps you launch an all-in-one server, which allows you to run all of the
components of an enterprise Kubernetes system on a server with Docker. Running:

  $ %[1]s start

will start listening on all interfaces, launch an etcd server to store persistent
data, and launch the Kubernetes system components. The server will run in the foreground until
you terminate the process.  This command delegates to "%[1]s start master" and
"%[1]s start node".

Note: starting OpenShift without passing the --master address will attempt to find the IP
address that will be visible inside running Docker containers. This is not always successful,
so if you have problems tell OpenShift what public address it will be via --master=<ip>.

You may also pass --etcd=<address> to connect to an external etcd server.

You may also pass --kubeconfig=<path> to connect to an external Kubernetes cluster.`

// NewCommandStartAllInOne provides a CLI handler for 'start' command
func NewCommandStartAllInOne(fullName string, out io.Writer) (*cobra.Command, *AllInOneOptions) {
	options := &AllInOneOptions{Output: out}

	switch fullName {
	case "atomic-enterprise":
		options.DisabledFeatures = configapi.AtomicDisabledFeatures
	}

	cmds := &cobra.Command{
		Use:   "start",
		Short: "Launch all-in-one server",
		Long:  fmt.Sprintf(allInOneLong, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(); err != nil {
				fmt.Println(kcmdutil.UsageError(c, err.Error()))
				return
			}
			if err := options.Validate(args); err != nil {
				fmt.Println(kcmdutil.UsageError(c, err.Error()))
				return
			}

			startProfiler()

			if err := options.StartAllInOne(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(c.Out(), "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(c.Out(), "  %s: %s\n", cause.Field, cause.Message)
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
	flags.StringVar(&options.MasterConfigFile, "master-config", "", "Location of the master configuration file to run from. When running from configuration files, all other command-line arguments are ignored.")
	flags.StringVar(&options.NodeConfigFile, "node-config", "", "Location of the node configuration file to run from. When running from configuration files, all other command-line arguments are ignored.")
	flags.BoolVar(&options.CreateCerts, "create-certs", true, "Indicates whether missing certs should be created.")
	flags.BoolVar(&options.PrintIP, "print-ip", false, "Print the IP that would be used if no master IP is specified and exit.")

	masterArgs, nodeArgs, listenArg, imageFormatArgs, _ := GetAllInOneArgs()
	options.MasterArgs, options.NodeArgs = masterArgs, nodeArgs
	// by default, all-in-ones all disabled docker.  Set it here so that if we allow it to be bound later, bindings take precedence
	options.NodeArgs.AllowDisabledDocker = true

	BindMasterArgs(masterArgs, flags, "")
	BindNodeArgs(nodeArgs, flags, "")
	BindListenArg(listenArg, flags, "")
	BindImageFormatArgs(imageFormatArgs, flags, "")

	startMaster, _ := NewCommandStartMaster(fullName, out)
	startNode, _ := NewCommandStartNode(fullName, out)
	cmds.AddCommand(startMaster)
	cmds.AddCommand(startNode)

	startKube := kubernetes.NewCommand("kubernetes", fullName, out)
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
	nodeArgs := NewDefaultNodeArgs()

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

	if (len(o.MasterConfigFile) == 0) != (len(o.NodeConfigFile) == 0) {
		return errors.New("--master-config and --node-config must both be specified or both be unspecified")
	}

	if o.IsRunFromConfig() && o.IsWriteConfigOnly() {
		return errors.New("--master-config and --node-config cannot be specified when --write-config is specified")
	}

	if len(o.ConfigDir.Value()) == 0 {
		return errors.New("config directory must have a value")
	}

	// if we are not starting up using a config file, run the argument validation
	if !o.IsRunFromConfig() {
		if err := o.MasterArgs.Validate(); err != nil {
			return err
		}

		if err := o.NodeArgs.Validate(); err != nil {
			return err
		}

	}

	if len(o.MasterArgs.KubeConnectionArgs.ClientConfigLoadingRules.ExplicitPath) != 0 {
		return errors.New("all-in-one cannot start with a remote Kubernetes server, start the master instead")
	}

	return nil
}

func (o *AllInOneOptions) Complete() error {
	if o.ConfigDir.Provided() {
		o.MasterArgs.ConfigDir.Set(path.Join(o.ConfigDir.Value(), "master"))
		o.NodeArgs.ConfigDir.Set(path.Join(o.ConfigDir.Value(), admin.DefaultNodeDir(o.NodeArgs.NodeName)))
	} else {
		o.ConfigDir.Default("openshift.local.config")
		o.MasterArgs.ConfigDir.Default(path.Join(o.ConfigDir.Value(), "master"))
		o.NodeArgs.ConfigDir.Default(path.Join(o.ConfigDir.Value(), admin.DefaultNodeDir(o.NodeArgs.NodeName)))
	}

	nodeList := util.NewStringSet(strings.ToLower(o.NodeArgs.NodeName))
	// take everything toLower
	for _, s := range o.MasterArgs.NodeList {
		nodeList.Insert(strings.ToLower(s))
	}
	o.MasterArgs.NodeList = nodeList.List()

	o.MasterArgs.NetworkArgs.NetworkPluginName = o.NodeArgs.NetworkPluginName

	masterAddr, err := o.MasterArgs.GetMasterAddress()
	if err != nil {
		return err
	}
	// in the all-in-one, default kubernetes URL to the master's address
	o.NodeArgs.DefaultKubernetesURL = masterAddr
	o.NodeArgs.NodeName = strings.ToLower(o.NodeArgs.NodeName)
	o.NodeArgs.MasterCertDir = o.MasterArgs.ConfigDir.Value()

	// in the all-in-one, default ClusterDNS to the master's address
	if host, _, err := net.SplitHostPort(masterAddr.Host); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			o.NodeArgs.ClusterDNS = ip
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
	masterOptions := MasterOptions{o.MasterArgs, o.CreateCerts, o.MasterConfigFile, o.Output, o.DisabledFeatures}
	if err := masterOptions.RunMaster(); err != nil {
		return err
	}

	nodeOptions := NodeOptions{o.NodeArgs, o.NodeConfigFile, o.Output}
	if err := nodeOptions.RunNode(); err != nil {
		return err
	}

	if o.IsWriteConfigOnly() {
		return nil
	}

	daemon.SdNotify("READY=1")
	select {}

	return nil
}

func startProfiler() {
	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			glog.Infof("Starting profiling endpoint at http://127.0.0.1:6060/debug/pprof/")
			glog.Fatal(http.ListenAndServe("127.0.0.1:6060", nil))
		}()
	}
}

func (o AllInOneOptions) IsWriteConfigOnly() bool {
	return o.ConfigDir.Provided()
}

func (o AllInOneOptions) IsRunFromConfig() bool {
	return (len(o.MasterConfigFile) > 0) && (len(o.NodeConfigFile) > 0)
}
