package start

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"strings"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/capabilities"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/certs"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

type MasterOptions struct {
	MasterArgs *MasterArgs

	WriteConfigOnly bool
	ConfigFile      string
}

const longMasterCommandDesc = `
Start an OpenShift master

This command helps you launch an OpenShift master.  Running

    $ openshift start master

will start an OpenShift master listening on all interfaces, launch an etcd server to store 
persistent data, and launch the Kubernetes system components. The server will run in the 
foreground until you terminate the process.

Note: starting OpenShift without passing the --master address will attempt to find the IP
address that will be visible inside running Docker containers. This is not always successful,
so if you have problems tell OpenShift what public address it will be via --master=<ip>.

You may also pass an optional argument to the start command to start OpenShift in one of the
following roles:

    $ openshift start master --nodes=<host1,host2,host3,...>

      Launches the server and control plane for OpenShift. You may pass a list of the node
      hostnames you want to use, or create nodes via the REST API or 'openshift kube'.

You may also pass --etcd=<address> to connect to an external etcd server.

You may also pass --kubeconfig=<path> to connect to an external Kubernetes cluster.
`

// NewCommandStartMaster provides a CLI handler for 'start' command
func NewCommandStartMaster() (*cobra.Command, *MasterOptions) {
	options := &MasterOptions{}

	cmd := &cobra.Command{
		Use:   "master",
		Short: "Launch OpenShift master",
		Long:  longMasterCommandDesc,
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

			if err := options.StartMaster(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&options.WriteConfigOnly, "write-config", false, "Indicates that the command should build the configuration from command-line arguments, write it to the location specified by --config, and exit.")
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the master configuration file to run from, or write to (when used with --write-config). When running from a configuration file, all other command-line arguments are ignored.")

	options.MasterArgs = NewDefaultMasterArgs()
	// make sure that KubeConnectionArgs and MasterArgs use the same CertArgs for this command
	options.MasterArgs.KubeConnectionArgs.CertArgs = options.MasterArgs.CertArgs

	BindMasterArgs(options.MasterArgs, flags, "")
	BindBindAddrArg(options.MasterArgs.BindAddrArg, flags, "")
	BindImageFormatArgs(options.MasterArgs.ImageFormatArgs, flags, "")
	BindKubeConnectionArgs(options.MasterArgs.KubeConnectionArgs, flags, "")
	BindCertArgs(options.MasterArgs.CertArgs, flags, "")

	return cmd, options
}

func (o MasterOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported for start master")
	}
	if o.WriteConfigOnly {
		if len(o.ConfigFile) == 0 {
			return errors.New("--config is required if --write-config is true")
		}
	}

	return nil
}

func (o MasterOptions) Complete() error {
	nodeList := util.NewStringSet()
	// take everything toLower
	for _, s := range o.MasterArgs.NodeList {
		nodeList.Insert(strings.ToLower(s))
	}

	o.MasterArgs.NodeList = nodeList.List()

	return nil
}

// StartMaster calls RunMaster and then waits forever
func (o MasterOptions) StartMaster() error {
	if err := o.RunMaster(); err != nil {
		return err
	}

	if o.WriteConfigOnly {
		return nil
	}

	daemon.SdNotify("READY=1")
	select {}

	return nil
}

// RunMaster takes the options and:
// 1.  Creates certs if needed
// 2.  Reads fully specified master config OR builds a fully specified master config from the args
// 3.  Writes the fully specified master config and exits if needed
// 4.  Starts the master based on the fully specified config
func (o MasterOptions) RunMaster() error {
	startUsingConfigFile := !o.WriteConfigOnly && (len(o.ConfigFile) > 0)
	mintCerts := o.MasterArgs.CertArgs.CreateCerts && !startUsingConfigFile

	if mintCerts {
		if err := o.CreateCerts(); err != nil {
			return nil
		}
	}

	var masterConfig *configapi.MasterConfig
	var err error
	if startUsingConfigFile {
		masterConfig, err = ReadMasterConfig(o.ConfigFile)
	} else {
		masterConfig, err = o.MasterArgs.BuildSerializeableMasterConfig()
	}
	if err != nil {
		return err
	}

	if o.WriteConfigOnly {
		content, err := WriteMaster(masterConfig)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(o.ConfigFile, content, 0644); err != nil {
			return err
		}
		return nil
	}

	if err := StartMaster(masterConfig); err != nil {
		return err
	}

	return nil
}

func (o MasterOptions) CreateCerts() error {
	signerName := certs.DefaultSignerName()
	hostnames, err := o.MasterArgs.GetServerCertHostnames()
	if err != nil {
		return err
	}
	mintAllCertsOptions := certs.CreateAllCertsOptions{
		CertDir:    o.MasterArgs.CertArgs.CertDir,
		SignerName: signerName,
		Hostnames:  hostnames.List(),
		NodeList:   o.MasterArgs.NodeList,
	}
	if err := mintAllCertsOptions.CreateAllCerts(); err != nil {
		return err
	}

	rootCAFile := certs.DefaultRootCAFile(o.MasterArgs.CertArgs.CertDir)
	masterAddr, err := o.MasterArgs.GetMasterAddress()
	if err != nil {
		return err
	}
	publicMasterAddr, err := o.MasterArgs.GetMasterPublicAddress()
	if err != nil {
		return err
	}
	for _, clientCertInfo := range certs.DefaultClientCerts(o.MasterArgs.CertArgs.CertDir) {
		createKubeConfigOptions := certs.CreateKubeConfigOptions{
			APIServerURL:       masterAddr.String(),
			PublicAPIServerURL: publicMasterAddr.String(),
			APIServerCAFile:    rootCAFile,
			ServerNick:         "master",

			CertFile: clientCertInfo.CertLocation.CertFile,
			KeyFile:  clientCertInfo.CertLocation.KeyFile,
			UserNick: clientCertInfo.SubDir,

			KubeConfigFile: certs.DefaultKubeConfigFilename(o.MasterArgs.CertArgs.CertDir, clientCertInfo.SubDir),
		}

		if _, err := createKubeConfigOptions.CreateKubeConfig(); err != nil {
			return err
		}
	}

	return nil
}

func ReadMasterConfig(filename string) (*configapi.MasterConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := &configapi.MasterConfig{}

	if err := configapilatest.Codec.DecodeInto(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

func StartMaster(openshiftMasterConfig *configapi.MasterConfig) error {
	glog.Infof("Starting an OpenShift master, reachable at %s (etcd: %s)", openshiftMasterConfig.ServingInfo.BindAddress, openshiftMasterConfig.EtcdClientInfo.URL)
	glog.Infof("OpenShift master public address is %s", openshiftMasterConfig.AssetConfig.MasterPublicURL)

	if openshiftMasterConfig.EtcdConfig != nil {
		etcdConfig := &etcd.Config{
			BindAddr:     openshiftMasterConfig.EtcdConfig.ServingInfo.BindAddress,
			PeerBindAddr: openshiftMasterConfig.EtcdConfig.PeerAddress,
			MasterAddr:   openshiftMasterConfig.EtcdConfig.MasterAddress,
			EtcdDir:      openshiftMasterConfig.EtcdConfig.StorageDir,
		}

		etcdConfig.Run()
	}

	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			glog.Infof("Starting profiling endpoint at http://127.0.0.1:6060/debug/pprof/")
			glog.Fatal(http.ListenAndServe("127.0.0.1:6060", nil))
		}()
	}

	// Allow privileged containers
	// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged: true,
	})

	openshiftConfig, err := origin.BuildMasterConfig(*openshiftMasterConfig)
	if err != nil {
		return err
	}
	//	 must start policy caching immediately
	openshiftConfig.RunPolicyCache()

	authConfig, err := origin.BuildAuthConfig(*openshiftMasterConfig)
	if err != nil {
		return err
	}

	if openshiftMasterConfig.KubernetesMasterConfig != nil {
		glog.Infof("Static Nodes: %v", openshiftMasterConfig.KubernetesMasterConfig.StaticNodeNames)

		kubeConfig, err := kubernetes.BuildKubernetesMasterConfig(*openshiftMasterConfig, openshiftConfig.RequestContextMapper, openshiftConfig.KubeClient())
		if err != nil {
			return err
		}
		kubeConfig.EnsurePortalFlags()

		openshiftConfig.Run([]origin.APIInstaller{kubeConfig}, []origin.APIInstaller{authConfig})

		kubeConfig.RunScheduler()
		kubeConfig.RunReplicationController()
		kubeConfig.RunEndpointController()
		kubeConfig.RunMinionController()
		kubeConfig.RunResourceQuotaManager()

	} else {
		_, kubeConfig, err := configapi.GetKubeClient(openshiftMasterConfig.MasterClients.KubernetesKubeConfig)
		if err != nil {
			return err
		}

		proxy := &kubernetes.ProxyConfig{
			ClientConfig: kubeConfig,
		}

		openshiftConfig.Run([]origin.APIInstaller{proxy}, []origin.APIInstaller{authConfig})
	}

	// TODO: recording should occur in individual components
	record.StartRecording(openshiftConfig.KubeClient().Events(""), kapi.EventSource{Component: "master"})

	glog.Infof("Using images from %q", openshiftConfig.ImageFor("<component>"))

	if openshiftMasterConfig.DNSConfig != nil {
		openshiftConfig.RunDNSServer()
	}
	if openshiftMasterConfig.AssetConfig != nil {
		openshiftConfig.RunAssetServer()
	}
	openshiftConfig.RunBuildController()
	openshiftConfig.RunBuildPodController()
	openshiftConfig.RunBuildImageChangeTriggerController()
	if err := openshiftConfig.RunDeploymentController(); err != nil {
		return err
	}
	openshiftConfig.RunDeployerPodController()
	openshiftConfig.RunDeploymentConfigController()
	openshiftConfig.RunDeploymentConfigChangeController()
	openshiftConfig.RunDeploymentImageChangeTriggerController()
	openshiftConfig.RunProjectAuthorizationCache()

	return nil
}
