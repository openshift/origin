package start

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/capabilities"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

type MasterOptions struct {
	MasterArgs *MasterArgs

	CreateCertificates bool
	ConfigFile         string
	Output             io.Writer
}

const master_long = `Start an OpenShift master.

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

  // Launches the server and control plane for OpenShift. You may pass a list of the node
  // hostnames you want to use, or create nodes via the REST API or 'openshift kube'.
  $ openshift start master --nodes=<host1,host2,host3,...>

You may also pass --etcd=<address> to connect to an external etcd server.

You may also pass --kubeconfig=<path> to connect to an external Kubernetes cluster.`

// NewCommandStartMaster provides a CLI handler for 'start' command
func NewCommandStartMaster(out io.Writer) (*cobra.Command, *MasterOptions) {
	options := &MasterOptions{Output: out}

	cmd := &cobra.Command{
		Use:   "master",
		Short: "Launch OpenShift master",
		Long:  master_long,
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

			if err := options.StartMaster(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(options.Output, "Invalid %s %s\n", details.Kind, details.ID)
						for _, cause := range details.Causes {
							fmt.Fprintln(options.Output, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	options.MasterArgs = NewDefaultMasterArgs()

	flags := cmd.Flags()

	flags.Var(options.MasterArgs.ConfigDir, "write-config", "Directory to write an initial config into.  After writing, exit without starting the server.")
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the master configuration file to run from. When running from a configuration file, all other command-line arguments are ignored.")
	flags.BoolVar(&options.CreateCertificates, "create-certs", true, "Indicates whether missing certs should be created")

	BindMasterArgs(options.MasterArgs, flags, "")
	BindListenArg(options.MasterArgs.ListenArg, flags, "")
	BindImageFormatArgs(options.MasterArgs.ImageFormatArgs, flags, "")
	BindKubeConnectionArgs(options.MasterArgs.KubeConnectionArgs, flags, "")
	BindNetworkArgs(options.MasterArgs.NetworkArgs, flags, "")

	return cmd, options
}

func (o MasterOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported for start master")
	}
	if o.IsWriteConfigOnly() {
		if o.IsRunFromConfig() {
			return errors.New("--config may not be set if --write-config is set")
		}
	}

	if len(o.MasterArgs.ConfigDir.Value()) == 0 {
		return errors.New("configDir must have a value")
	}

	// if we are not starting up using a config file, run the argument validation
	if !o.IsRunFromConfig() {
		if err := o.MasterArgs.Validate(); err != nil {
			return err
		}

	}

	return nil
}

func (o *MasterOptions) Complete() error {
	if !o.MasterArgs.ConfigDir.Provided() {
		o.MasterArgs.ConfigDir.Default("openshift.local.config/master")
	}

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

	if o.IsWriteConfigOnly() {
		return nil
	}

	select {}

	return nil
}

// RunMaster takes the options and:
// 1.  Creates certs if needed
// 2.  Reads fully specified master config OR builds a fully specified master config from the args
// 3.  Writes the fully specified master config and exits if needed
// 4.  Starts the master based on the fully specified config
func (o MasterOptions) RunMaster() error {
	startUsingConfigFile := !o.IsWriteConfigOnly() && o.IsRunFromConfig()

	if !startUsingConfigFile && o.CreateCertificates {
		glog.V(2).Infof("Generating master configuration")
		if err := o.CreateCerts(); err != nil {
			return err
		}
		if err := o.CreateBootstrapPolicy(); err != nil {
			return err
		}
	}

	var masterConfig *configapi.MasterConfig
	var err error
	if startUsingConfigFile {
		masterConfig, err = configapilatest.ReadAndResolveMasterConfig(o.ConfigFile)
	} else {
		masterConfig, err = o.MasterArgs.BuildSerializeableMasterConfig()
	}
	if err != nil {
		return err
	}

	if o.IsWriteConfigOnly() {
		// Resolve relative to CWD
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := configapi.ResolveMasterConfigPaths(masterConfig, cwd); err != nil {
			return err
		}

		// Relativize to config file dir
		base, err := cmdutil.MakeAbs(filepath.Dir(o.MasterArgs.GetConfigFileToWrite()), cwd)
		if err != nil {
			return err
		}
		if err := configapi.RelativizeMasterConfigPaths(masterConfig, base); err != nil {
			return err
		}

		content, err := configapilatest.WriteYAML(masterConfig)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(path.Dir(o.MasterArgs.GetConfigFileToWrite()), os.FileMode(0755)); err != nil {
			return err
		}
		if err := ioutil.WriteFile(o.MasterArgs.GetConfigFileToWrite(), content, 0644); err != nil {
			return err
		}

		fmt.Fprintf(o.Output, "Wrote master config to: %s\n", o.MasterArgs.GetConfigFileToWrite())

		return nil
	}

	validationResults := validation.ValidateMasterConfig(masterConfig)
	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("%v", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return kerrors.NewInvalid("MasterConfig", o.ConfigFile, validationResults.Errors)
	}

	if err := StartMaster(masterConfig); err != nil {
		return err
	}

	return nil
}

func (o MasterOptions) CreateBootstrapPolicy() error {
	writeBootstrapPolicy := admin.CreateBootstrapPolicyFileOptions{
		File: o.MasterArgs.GetPolicyFile(),
		OpenShiftSharedResourcesNamespace: bootstrappolicy.DefaultOpenShiftSharedResourcesNamespace,
	}

	return writeBootstrapPolicy.CreateBootstrapPolicyFile()
}

func (o MasterOptions) CreateCerts() error {
	masterAddr, err := o.MasterArgs.GetMasterAddress()
	if err != nil {
		return err
	}
	publicMasterAddr, err := o.MasterArgs.GetMasterPublicAddress()
	if err != nil {
		return err
	}

	signerName := admin.DefaultSignerName()
	hostnames, err := o.MasterArgs.GetServerCertHostnames()
	if err != nil {
		return err
	}
	mintAllCertsOptions := admin.CreateMasterCertsOptions{
		CertDir:            o.MasterArgs.ConfigDir.Value(),
		SignerName:         signerName,
		Hostnames:          hostnames.List(),
		APIServerURL:       masterAddr.String(),
		PublicAPIServerURL: publicMasterAddr.String(),
		Output:             o.Output,
	}
	if err := mintAllCertsOptions.Validate(nil); err != nil {
		return err
	}
	if err := mintAllCertsOptions.CreateMasterCerts(); err != nil {
		return err
	}

	return nil
}

func StartMaster(openshiftMasterConfig *configapi.MasterConfig) error {
	glog.Infof("Starting an OpenShift master, reachable at %s (etcd: %v)", openshiftMasterConfig.ServingInfo.BindAddress, openshiftMasterConfig.EtcdClientInfo.URLs)
	glog.Infof("OpenShift master public address is %s", openshiftMasterConfig.AssetConfig.MasterPublicURL)

	if openshiftMasterConfig.EtcdConfig != nil {
		etcd.RunEtcd(openshiftMasterConfig.EtcdConfig)
	}

	// Allow privileged containers
	// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged:    true,
		HostNetworkSources: []string{kubelet.ApiserverSource, kubelet.FileSource},
	})

	openshiftConfig, err := origin.BuildMasterConfig(*openshiftMasterConfig)
	if err != nil {
		return err
	}
	// Must start policy caching immediately
	openshiftConfig.RunPolicyCache()
	openshiftConfig.RunProjectCache()

	unprotectedInstallers := []origin.APIInstaller{}

	if openshiftMasterConfig.OAuthConfig != nil {
		authConfig, err := origin.BuildAuthConfig(*openshiftMasterConfig)
		if err != nil {
			return err
		}
		unprotectedInstallers = append(unprotectedInstallers, authConfig)
	}

	var standaloneAssetConfig *origin.AssetConfig
	if openshiftMasterConfig.AssetConfig != nil {
		config, err := origin.BuildAssetConfig(*openshiftMasterConfig.AssetConfig)
		if err != nil {
			return err
		}

		if openshiftMasterConfig.AssetConfig.ServingInfo.BindAddress == openshiftMasterConfig.ServingInfo.BindAddress {
			unprotectedInstallers = append(unprotectedInstallers, config)
		} else {
			standaloneAssetConfig = config
		}
	}

	if openshiftMasterConfig.KubernetesMasterConfig != nil {
		kubeConfig, err := kubernetes.BuildKubernetesMasterConfig(*openshiftMasterConfig, openshiftConfig.RequestContextMapper, openshiftConfig.KubeClient())
		if err != nil {
			return err
		}
		kubeConfig.EnsurePortalFlags()

		openshiftConfig.Run([]origin.APIInstaller{kubeConfig}, unprotectedInstallers)
		go daemon.SdNotify("READY=1")

		kubeConfig.RunScheduler()
		kubeConfig.RunReplicationController()
		kubeConfig.RunEndpointController()
		kubeConfig.RunNodeController()
		kubeConfig.RunResourceQuotaManager()
		kubeConfig.RunNamespaceController()
		kubeConfig.RunPersistentVolumeClaimBinder()
	} else {
		_, kubeConfig, err := configapi.GetKubeClient(openshiftMasterConfig.MasterClients.ExternalKubernetesKubeConfig)
		if err != nil {
			return err
		}

		proxy := &kubernetes.ProxyConfig{
			ClientConfig: kubeConfig,
		}

		openshiftConfig.Run([]origin.APIInstaller{proxy}, unprotectedInstallers)
		go daemon.SdNotify("READY=1")
	}

	glog.Infof("Using images from %q", openshiftConfig.ImageFor("<component>"))

	if standaloneAssetConfig != nil {
		standaloneAssetConfig.Run()
	}
	if openshiftMasterConfig.DNSConfig != nil {
		openshiftConfig.RunDNSServer()
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
	openshiftConfig.RunImageImportController()
	openshiftConfig.RunOriginNamespaceController()
	openshiftConfig.RunProjectAuthorizationCache()
	openshiftConfig.RunSecurityAllocationController()
	openshiftConfig.RunServiceAccountsController()
	openshiftConfig.RunServiceAccountTokensController()
	openshiftConfig.RunServiceAccountPullSecretsControllers()

	openshiftConfig.RunSDNController()

	return nil
}

func (o MasterOptions) IsWriteConfigOnly() bool {
	return o.MasterArgs.ConfigDir.Provided()
}

func (o MasterOptions) IsRunFromConfig() bool {
	return (len(o.ConfigFile) > 0)
}
