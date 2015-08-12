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

	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/capabilities"
	"k8s.io/kubernetes/pkg/kubelet"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/version"
)

type MasterOptions struct {
	MasterArgs *MasterArgs

	CreateCertificates bool
	ConfigFile         string
	Output             io.Writer
	DisabledFeatures   []string
}

const masterLong = `Start a master server

This command helps you launch a master server.  Running

  $ %[1]s start master

will start a master listening on all interfaces, launch an etcd server to store
persistent data, and launch the Kubernetes system components. The server will run in the
foreground until you terminate the process.

Note: starting the master without passing the --master address will attempt to find the IP
address that will be visible inside running Docker containers. This is not always successful,
so if you have problems tell the master what public address it should use via --master=<ip>.

You may also pass an optional argument to the start command to start in one of the
following roles:

  // Launches the server and control plane. You may pass a list of the node
  // hostnames you want to use, or create nodes via the REST API or '%[1]s kube'.
  $ %[1]s start master --nodes=<host1,host2,host3,...>

You may also pass --etcd=<address> to connect to an external etcd server.

You may also pass --kubeconfig=<path> to connect to an external Kubernetes cluster.`

// NewCommandStartMaster provides a CLI handler for 'start master' command
func NewCommandStartMaster(fullName string, out io.Writer) (*cobra.Command, *MasterOptions) {
	options := &MasterOptions{Output: out}

	switch fullName {
	case "atomic-enterprise":
		options.DisabledFeatures = configapi.AtomicDisabledFeatures
	}

	cmd := &cobra.Command{
		Use:   "master",
		Short: "Launch OpenShift master",
		Long:  fmt.Sprintf(masterLong, fullName),
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
						fmt.Fprintf(options.Output, "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(options.Output, "  %s: %s\n", cause.Field, cause.Message)
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

	// autocompletion hints
	cmd.MarkFlagFilename("write-config")
	cmd.MarkFlagFilename("config", "yaml", "yml")

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

	go daemon.SdNotify("READY=1")
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

	// Inject disabled feature flags based on distribution being used and
	// regardless of configuration. They aren't written to config file to
	// prevent upgrade path issues.
	masterConfig.DisabledFeatures.Add(o.DisabledFeatures...)
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
	glog.Infof("Starting master on %s (%s)", openshiftMasterConfig.ServingInfo.BindAddress, version.Get().String())
	glog.Infof("Public master address is %s", openshiftMasterConfig.AssetConfig.MasterPublicURL)
	if len(openshiftMasterConfig.DisabledFeatures) > 0 {
		glog.V(4).Infof("Disabled features: %s", strings.Join(openshiftMasterConfig.DisabledFeatures, ", "))
	}

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

	go func() {
		openshiftConfig.ControllerPlug.WaitForStop()
		glog.Fatalf("Master shutdown requested")
	}()

	// Must start policy caching immediately
	openshiftConfig.RunGroupCache()
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

	var kubeConfig *kubernetes.MasterConfig
	if openshiftMasterConfig.KubernetesMasterConfig != nil {
		kubeConfig, err = kubernetes.BuildKubernetesMasterConfig(*openshiftMasterConfig, openshiftConfig.RequestContextMapper, openshiftConfig.KubeClient())
		if err != nil {
			return err
		}

		openshiftConfig.Run([]origin.APIInstaller{kubeConfig}, unprotectedInstallers)

	} else {
		_, kubeConfig, err := configapi.GetKubeClient(openshiftMasterConfig.MasterClients.ExternalKubernetesKubeConfig)
		if err != nil {
			return err
		}

		proxy := &kubernetes.ProxyConfig{
			ClientConfig: kubeConfig,
		}

		openshiftConfig.Run([]origin.APIInstaller{proxy}, unprotectedInstallers)
	}

	glog.Infof("Using images from %q", openshiftConfig.ImageFor("<component>"))

	if standaloneAssetConfig != nil {
		standaloneAssetConfig.Run()
	}
	if openshiftMasterConfig.DNSConfig != nil {
		openshiftConfig.RunDNSServer()
	}

	openshiftConfig.RunProjectAuthorizationCache()

	if openshiftMasterConfig.Controllers != configapi.ControllersDisabled {
		go func() {
			openshiftConfig.ControllerPlug.WaitForStart()
			glog.Infof("Master controllers starting (%s)", openshiftMasterConfig.Controllers)

			// Start these first, because they provide credentials for other controllers' clients
			openshiftConfig.RunServiceAccountsController()
			openshiftConfig.RunServiceAccountTokensController()
			// used by admission controllers
			openshiftConfig.RunServiceAccountPullSecretsControllers()
			openshiftConfig.RunSecurityAllocationController()

			if kubeConfig != nil {
				_, rcClient, err := openshiftConfig.GetServiceAccountClients(openshiftConfig.ReplicationControllerServiceAccount)
				if err != nil {
					glog.Fatalf("Could not get client for replication controller: %v", err)
				}

				// called by admission control
				kubeConfig.RunResourceQuotaManager()

				// no special order
				kubeConfig.RunNodeController()
				kubeConfig.RunScheduler()
				kubeConfig.RunReplicationController(rcClient)
				kubeConfig.RunEndpointController()
				kubeConfig.RunNamespaceController()
				kubeConfig.RunPersistentVolumeClaimBinder()
				kubeConfig.RunPersistentVolumeClaimRecycler(openshiftConfig.ImageFor("deployer"))
			}

			// no special order
			openshiftConfig.RunBuildController()
			openshiftConfig.RunBuildPodController()
			openshiftConfig.RunBuildImageChangeTriggerController()
			openshiftConfig.RunDeploymentController()
			openshiftConfig.RunDeployerPodController()
			openshiftConfig.RunDeploymentConfigController()
			openshiftConfig.RunDeploymentConfigChangeController()
			openshiftConfig.RunDeploymentImageChangeTriggerController()
			openshiftConfig.RunImageImportController()
			openshiftConfig.RunOriginNamespaceController()
			openshiftConfig.RunSDNController()
		}()
	}

	return nil
}

func (o MasterOptions) IsWriteConfigOnly() bool {
	return o.MasterArgs.ConfigDir.Provided()
}

func (o MasterOptions) IsRunFromConfig() bool {
	return (len(o.ConfigFile) > 0)
}
