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
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
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

func (o *MasterOptions) DefaultsFromName(basename string) {
	switch basename {
	case "atomic-enterprise":
		o.DisabledFeatures = configapi.AtomicDisabledFeatures
	}
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

You may also pass --etcd=<address> to connect to an external etcd server.

You may also pass --kubeconfig=<path> to connect to an external Kubernetes cluster.`

// NewCommandStartMaster provides a CLI handler for 'start master' command
func NewCommandStartMaster(basename string, out io.Writer) (*cobra.Command, *MasterOptions) {
	options := &MasterOptions{Output: out}
	options.DefaultsFromName(basename)

	cmd := &cobra.Command{
		Use:   "master",
		Short: "Launch a master",
		Long:  fmt.Sprintf(masterLong, basename),
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

			if err := options.StartMaster(); err != nil {
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

	options.MasterArgs = NewDefaultMasterArgs()
	options.MasterArgs.StartAPI = true
	options.MasterArgs.StartControllers = true

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

	startControllers, _ := NewCommandStartMasterControllers("controllers", basename, out)
	startAPI, _ := NewCommandStartMasterAPI("api", basename, out)
	cmd.AddCommand(startAPI)
	cmd.AddCommand(startControllers)

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

	// TODO: this should be encapsulated by RunMaster, but StartAllInOne has no
	// way to communicate whether RunMaster should block.
	go daemon.SdNotify("READY=1")
	select {}
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

	if o.MasterArgs.OverrideConfig != nil {
		if err := o.MasterArgs.OverrideConfig(masterConfig); err != nil {
			return err
		}
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

	if !o.MasterArgs.StartControllers {
		masterConfig.Controllers = configapi.ControllersDisabled
	}

	m := &master{
		config:      masterConfig,
		api:         o.MasterArgs.StartAPI,
		controllers: o.MasterArgs.StartControllers,
	}
	return m.Start()
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

func buildKubernetesMasterConfig(openshiftConfig *origin.MasterConfig) (*kubernetes.MasterConfig, error) {
	if openshiftConfig.Options.KubernetesMasterConfig == nil {
		return nil, nil
	}
	kubeConfig, err := kubernetes.BuildKubernetesMasterConfig(openshiftConfig.Options, openshiftConfig.RequestContextMapper, openshiftConfig.KubeClient())
	return kubeConfig, err
}

// master encapsulates starting the components of the master
type master struct {
	config      *configapi.MasterConfig
	controllers bool
	api         bool
}

// Start launches a master. It will error if possible, but some background processes may still
// be running and the process should exit after it finishes.
func (m *master) Start() error {
	// Allow privileged containers
	// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged:    true,
		HostNetworkSources: []string{kubelet.ApiserverSource, kubelet.FileSource},
	})

	openshiftConfig, err := origin.BuildMasterConfig(*m.config)
	if err != nil {
		return err
	}

	kubeMasterConfig, err := buildKubernetesMasterConfig(openshiftConfig)
	if err != nil {
		return err
	}

	if m.api {
		glog.Infof("Starting master on %s (%s)", m.config.ServingInfo.BindAddress, version.Get().String())
		glog.Infof("Public master address is %s", m.config.AssetConfig.MasterPublicURL)
		if len(m.config.DisabledFeatures) > 0 {
			glog.V(4).Infof("Disabled features: %s", strings.Join(m.config.DisabledFeatures, ", "))
		}
		glog.Infof("Using images from %q", openshiftConfig.ImageFor("<component>"))

		if err := startAPI(openshiftConfig, kubeMasterConfig); err != nil {
			return err
		}
		if m.controllers {
			// run controllers asynchronously (not required to be "ready")
			go func() {
				if err := startControllers(openshiftConfig, kubeMasterConfig); err != nil {
					glog.Fatal(err)
				}
			}()
		}
		return nil
	}

	if m.controllers {
		glog.Infof("Starting controllers on %s (%s)", m.config.ServingInfo.BindAddress, version.Get().String())
		if len(m.config.DisabledFeatures) > 0 {
			glog.V(4).Infof("Disabled features: %s", strings.Join(m.config.DisabledFeatures, ", "))
		}
		glog.Infof("Using images from %q", openshiftConfig.ImageFor("<component>"))

		if err := startHealth(openshiftConfig); err != nil {
			return err
		}
		if err := startControllers(openshiftConfig, kubeMasterConfig); err != nil {
			return err
		}
	}

	return nil
}

func startHealth(openshiftConfig *origin.MasterConfig) error {
	openshiftConfig.RunHealth()
	return nil
}

// startAPI starts the components of the master that are considered part of the API - the Kubernetes
// API and core controllers, the Origin API, the group, policy, project, and authorization caches,
// etcd, the asset server (for the UI), the OAuth server endpoints, and the DNS server.
// TODO: allow to be more granularly targeted
func startAPI(oc *origin.MasterConfig, kc *kubernetes.MasterConfig) error {
	// start etcd
	if oc.Options.EtcdConfig != nil {
		etcd.RunEtcd(oc.Options.EtcdConfig)
	}

	// verify we can connect to etcd with the provided config
	if err := etcd.TestEtcdClient(oc.EtcdClient); err != nil {
		return err
	}

	// Must start policy caching immediately
	oc.RunGroupCache()
	oc.RunPolicyCache()
	oc.RunProjectCache()

	unprotectedInstallers := []origin.APIInstaller{}

	if oc.Options.OAuthConfig != nil {
		authConfig, err := origin.BuildAuthConfig(oc.Options)
		if err != nil {
			return err
		}
		unprotectedInstallers = append(unprotectedInstallers, authConfig)
	}

	var standaloneAssetConfig *origin.AssetConfig
	if oc.WebConsoleEnabled() {
		config, err := origin.BuildAssetConfig(*oc.Options.AssetConfig)
		if err != nil {
			return err
		}

		if oc.Options.AssetConfig.ServingInfo.BindAddress == oc.Options.ServingInfo.BindAddress {
			unprotectedInstallers = append(unprotectedInstallers, config)
		} else {
			standaloneAssetConfig = config
		}
	}

	if kc != nil {
		oc.Run([]origin.APIInstaller{kc}, unprotectedInstallers)
	} else {
		_, kubeClientConfig, err := configapi.GetKubeClient(oc.Options.MasterClients.ExternalKubernetesKubeConfig)
		if err != nil {
			return err
		}
		proxy := &kubernetes.ProxyConfig{
			ClientConfig: kubeClientConfig,
		}
		oc.Run([]origin.APIInstaller{proxy}, unprotectedInstallers)
	}

	oc.InitializeObjects()

	if standaloneAssetConfig != nil {
		standaloneAssetConfig.Run()
	}

	if oc.Options.DNSConfig != nil {
		oc.RunDNSServer()
	}

	oc.RunProjectAuthorizationCache()
	return nil
}

// startControllers launches the controllers
func startControllers(oc *origin.MasterConfig, kc *kubernetes.MasterConfig) error {
	if oc.Options.Controllers == configapi.ControllersDisabled {
		return nil
	}

	go func() {
		oc.ControllerPlugStart()
		// when a manual shutdown (DELETE /controllers) or lease lost occurs, the process should exit
		// this ensures no code is still running as a controller, and allows a process manager to reset
		// the controller to come back into a candidate state and compete for the lease
		oc.ControllerPlug.WaitForStop()
		glog.Fatalf("Controller shutdown requested")
	}()

	oc.ControllerPlug.WaitForStart()
	glog.Infof("Controllers starting (%s)", oc.Options.Controllers)

	// Start these first, because they provide credentials for other controllers' clients
	oc.RunServiceAccountsController()
	oc.RunServiceAccountTokensController()
	// used by admission controllers
	oc.RunServiceAccountPullSecretsControllers()
	oc.RunSecurityAllocationController()

	if kc != nil {
		_, rcClient, err := oc.GetServiceAccountClients(oc.ReplicationControllerServiceAccount)
		if err != nil {
			glog.Fatalf("Could not get client for replication controller: %v", err)
		}

		// called by admission control
		kc.RunResourceQuotaManager()

		// no special order
		kc.RunNodeController()
		kc.RunScheduler()
		kc.RunReplicationController(rcClient)
		kc.RunEndpointController()
		kc.RunNamespaceController()
		kc.RunPersistentVolumeClaimBinder()
		kc.RunPersistentVolumeClaimRecycler(oc.ImageFor("deployer"))
	}

	// no special order
	if configapi.IsBuildEnabled(&oc.Options) {
		oc.RunBuildController()
		oc.RunBuildPodController()
		oc.RunBuildConfigChangeController()
		oc.RunBuildImageChangeTriggerController()
	}
	oc.RunBuildController()
	oc.RunBuildPodController()
	oc.RunBuildConfigChangeController()
	oc.RunBuildImageChangeTriggerController()
	oc.RunDeploymentController()
	oc.RunDeployerPodController()
	oc.RunDeploymentConfigController()
	oc.RunDeploymentConfigChangeController()
	oc.RunDeploymentImageChangeTriggerController()
	oc.RunImageImportController()
	oc.RunOriginNamespaceController()
	oc.RunSDNController()

	return nil
}

func (o MasterOptions) IsWriteConfigOnly() bool {
	return o.MasterArgs.ConfigDir.Provided()
}

func (o MasterOptions) IsRunFromConfig() bool {
	return (len(o.ConfigFile) > 0)
}
