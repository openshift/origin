package start

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/docker"
)

type NodeOptions struct {
	NodeArgs *NodeArgs

	WriteConfigOnly bool
	ConfigFile      string
}

const longNodeCommandDesc = `
Start an OpenShift node
This command helps you launch an OpenShift node.  Running

    $ openshift start node --master=<masterIP>

will start an OpenShift node that attempts to connect to the master on the provided IP. The 
node will run in the foreground until you terminate the process.
`

// NewCommandStartMaster provides a CLI handler for 'start' command
func NewCommandStartNode() (*cobra.Command, *NodeOptions) {
	options := &NodeOptions{}

	cmd := &cobra.Command{
		Use:   "node",
		Short: "Launch OpenShift node",
		Long:  longNodeCommandDesc,
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}
			if err := options.Validate(args); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}

			startProfiler()

			if err := options.StartNode(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(c.Out(), "Invalid %s %s\n", details.Kind, details.ID)
						for _, cause := range details.Causes {
							fmt.Fprintln(c.Out(), cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&options.WriteConfigOnly, "write-config", false, "Indicates that the command should build the configuration from command-line arguments, write it to the location specified by --config, and exit.")
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the node configuration file to run from, or write to (when used with --write-config). When running from a configuration file, all other command-line arguments are ignored.")

	options.NodeArgs = NewDefaultNodeArgs()
	// make sure that KubeConnectionArgs and NodeArgs use the same CertArgs for this command
	options.NodeArgs.KubeConnectionArgs.CertArgs = options.NodeArgs.CertArgs

	BindNodeArgs(options.NodeArgs, flags, "")
	BindListenArg(options.NodeArgs.ListenArg, flags, "")
	BindImageFormatArgs(options.NodeArgs.ImageFormatArgs, flags, "")
	BindKubeConnectionArgs(options.NodeArgs.KubeConnectionArgs, flags, "")
	BindCertArgs(options.NodeArgs.CertArgs, flags, "")

	return cmd, options
}

func (o NodeOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported for start node")
	}
	if o.WriteConfigOnly {
		if len(o.ConfigFile) == 0 {
			return errors.New("--config is required if --write-config is true")
		}
	}

	// if we are not starting up using a config file, run the argument validation
	if o.WriteConfigOnly || len(o.ConfigFile) == 0 {
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

	if o.WriteConfigOnly {
		return nil
	}

	select {}

	return nil
}

// RunNode takes the options and:
// 1.  Creates certs if needed
// 2.  Reads fully specified node config OR builds a fully specified node config from the args
// 3.  Writes the fully specified node config and exits if needed
// 4.  Starts the node based on the fully specified config
func (o NodeOptions) RunNode() error {
	startUsingConfigFile := !o.WriteConfigOnly && (len(o.ConfigFile) > 0)
	mintCerts := o.NodeArgs.CertArgs.CreateCerts && !startUsingConfigFile

	if mintCerts {
		if err := o.CreateCerts(); err != nil {
			return err
		}
	}

	var nodeConfig *configapi.NodeConfig
	var err error
	if startUsingConfigFile {
		nodeConfig, err = configapilatest.ReadAndResolveNodeConfig(o.ConfigFile)
	} else {
		nodeConfig, err = o.NodeArgs.BuildSerializeableNodeConfig()
	}
	if err != nil {
		return err
	}

	if o.WriteConfigOnly {
		// Resolve relative to CWD
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := configapi.ResolveNodeConfigPaths(nodeConfig, cwd); err != nil {
			return err
		}

		// Relativize to config file dir
		base, err := cmdutil.MakeAbs(filepath.Dir(o.ConfigFile), cwd)
		if err != nil {
			return err
		}
		if err := configapi.RelativizeNodeConfigPaths(nodeConfig, base); err != nil {
			return err
		}

		content, err := configapilatest.WriteNode(nodeConfig)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(o.ConfigFile, content, 0644); err != nil {
			return err
		}
		return nil
	}

	errs := validation.ValidateNodeConfig(nodeConfig)
	if len(errs) != 0 {
		return kerrors.NewInvalid("NodeConfig", o.ConfigFile, errs)
	}

	_, kubeClientConfig, err := configapi.GetKubeClient(nodeConfig.MasterKubeConfig)
	if err != nil {
		return err
	}
	glog.Infof("Starting an OpenShift node, connecting to %s", kubeClientConfig.Host)

	if err := StartNode(*nodeConfig); err != nil {
		return err
	}

	return nil
}

func (o NodeOptions) CreateCerts() error {
	signerOptions := &admin.CreateSignerCertOptions{
		CertFile:   admin.DefaultCertFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
		KeyFile:    admin.DefaultKeyFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
		SerialFile: admin.DefaultSerialFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
		Name:       admin.DefaultSignerName(),
	}
	if err := signerOptions.Validate(nil); err != nil {
		return err
	}
	if _, err := signerOptions.CreateSignerCert(); err != nil {
		return err
	}
	getSignerOptions := &admin.GetSignerCertOptions{
		CertFile:   admin.DefaultCertFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
		KeyFile:    admin.DefaultKeyFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
		SerialFile: admin.DefaultSerialFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
	}

	var dnsIP string
	if len(o.NodeArgs.ClusterDNS) > 0 {
		dnsIP = o.NodeArgs.ClusterDNS.String()
	}

	masterAddr, err := o.NodeArgs.KubeConnectionArgs.GetKubernetesAddress(o.NodeArgs.DefaultKubernetesURL)
	if err != nil {
		return err
	}

	nodeConfigDir := path.Join(o.NodeArgs.CertArgs.CertDir, admin.DefaultNodeDir(o.NodeArgs.NodeName))
	createNodeConfigOptions := admin.CreateNodeConfigOptions{
		GetSignerCertOptions: getSignerOptions,

		NodeConfigDir: nodeConfigDir,

		NodeName:            o.NodeArgs.NodeName,
		Hostnames:           []string{o.NodeArgs.NodeName},
		VolumeDir:           o.NodeArgs.VolumeDir,
		ImageTemplate:       o.NodeArgs.ImageFormatArgs.ImageTemplate,
		AllowDisabledDocker: o.NodeArgs.AllowDisabledDocker,
		DNSDomain:           o.NodeArgs.ClusterDomain,
		DNSIP:               dnsIP,
		ListenAddr:          o.NodeArgs.ListenArg.ListenAddr,

		APIServerURL:    masterAddr.String(),
		APIServerCAFile: getSignerOptions.CertFile,

		NodeClientCAFile: getSignerOptions.CertFile,
	}

	if err := createNodeConfigOptions.Validate(nil); err != nil {
		return err
	}
	if err := createNodeConfigOptions.CreateNodeFolder(); err != nil {
		return err
	}

	return nil
}

func StartNode(config configapi.NodeConfig) error {
	nodeConfig, err := kubernetes.BuildKubernetesNodeConfig(config)
	if err != nil {
		return err
	}

	// TODO: recording should occur in individual components
	if config.RecordEvents {
		record.StartRecording(nodeConfig.Client.Events(""))
	}

	nodeConfig.EnsureVolumeDir()
	nodeConfig.EnsureDocker(docker.NewHelper())
	nodeConfig.RunProxy()
	nodeConfig.RunKubelet()
	go daemon.SdNotify("READY=1")

	return nil
}
