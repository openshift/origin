package start

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
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

			if err := options.StartNode(); err != nil {
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
			return nil
		}
	}

	var nodeConfig *configapi.NodeConfig
	var err error
	if startUsingConfigFile {
		nodeConfig, err = ReadNodeConfig(o.ConfigFile)
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

		content, err := WriteNode(nodeConfig)
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
		return kerrors.NewInvalid("nodeConfig", "", errs)
	}

	_, kubeClientConfig, err := configapi.GetKubeClient(nodeConfig.MasterKubeConfig)
	if err != nil {
		return err
	}
	glog.Infof("Starting an OpenShift node, connecting to %s", kubeClientConfig.Host)

	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			glog.Infof("Starting profiling endpoint at http://127.0.0.1:6060/debug/pprof/")
			glog.Fatal(http.ListenAndServe("127.0.0.1:6060", nil))
		}()
	}

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
	if _, err := signerOptions.CreateSignerCert(); err != nil {
		return err
	}
	getSignerOptions := &admin.GetSignerCertOptions{
		CertFile:   admin.DefaultCertFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
		KeyFile:    admin.DefaultKeyFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
		SerialFile: admin.DefaultSerialFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
	}

	serverCertInfo := admin.DefaultNodeServingCertInfo(o.NodeArgs.CertArgs.CertDir, o.NodeArgs.NodeName)
	nodeServerCertOptions := admin.CreateServerCertOptions{
		GetSignerCertOptions: getSignerOptions,

		CertFile: serverCertInfo.CertFile,
		KeyFile:  serverCertInfo.KeyFile,

		Hostnames: []string{o.NodeArgs.NodeName},
	}

	if _, err := nodeServerCertOptions.CreateServerCert(); err != nil {
		return err
	}

	clientCertInfo := admin.DefaultNodeClientCertInfo(o.NodeArgs.CertArgs.CertDir, o.NodeArgs.NodeName)
	mintNodeClientCert := admin.CreateNodeClientCertOptions{
		GetSignerCertOptions: getSignerOptions,
		CertFile:             clientCertInfo.CertFile,
		KeyFile:              clientCertInfo.KeyFile,
		NodeName:             o.NodeArgs.NodeName,
	}
	if _, err := mintNodeClientCert.CreateNodeClientCert(); err != nil {
		return err
	}

	masterAddr, err := o.NodeArgs.KubeConnectionArgs.GetKubernetesAddress(&o.NodeArgs.DefaultKubernetesURL)
	if err != nil {
		return err
	}

	createKubeConfigOptions := admin.CreateKubeConfigOptions{
		APIServerURL:    masterAddr.String(),
		APIServerCAFile: getSignerOptions.CertFile,
		ServerNick:      "master",

		CertFile: mintNodeClientCert.CertFile,
		KeyFile:  mintNodeClientCert.KeyFile,
		UserNick: o.NodeArgs.NodeName,

		KubeConfigFile: admin.DefaultNodeKubeConfigFile(o.NodeArgs.CertArgs.CertDir, o.NodeArgs.NodeName),
	}
	if _, err := createKubeConfigOptions.CreateKubeConfig(); err != nil {
		return err
	}

	return nil
}

func ReadNodeConfig(filename string) (*configapi.NodeConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := &configapi.NodeConfig{}

	if err := configapilatest.Codec.DecodeInto(data, config); err != nil {
		return nil, err
	}

	base, err := cmdutil.MakeAbs(filepath.Dir(filename), "")
	if err != nil {
		return nil, err
	}
	if err := configapi.ResolveNodeConfigPaths(config, base); err != nil {
		return nil, err
	}

	return config, nil
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
