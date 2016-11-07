package start

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/policy"
	"k8s.io/kubernetes/pkg/capabilities"
	"k8s.io/kubernetes/pkg/client/typed/dynamic"
	clientadapter "k8s.io/kubernetes/pkg/client/unversioned/adapters/internalclientset"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/etcd/etcdserver"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	override "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
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
	if cmdutil.GetProductName(basename) == cmdutil.ProductAtomicEnterprise {
		o.DisabledFeatures = configapi.AtomicDisabledFeatures
	}
}

var masterLong = templates.LongDesc(`
	Start a master server

	This command helps you launch a master server.  Running

	    %[1]s start master

	will start a master listening on all interfaces, launch an etcd server to store
	persistent data, and launch the Kubernetes system components. The server will run in the
	foreground until you terminate the process.

	Note: starting the master without passing the --master address will attempt to find the IP
	address that will be visible inside running Docker containers. This is not always successful,
	so if you have problems tell the master what public address it should use via --master=<ip>.

	You may also pass --etcd=<address> to connect to an external etcd server.

	You may also pass --kubeconfig=<path> to connect to an external Kubernetes cluster.`)

// NewCommandStartMaster provides a CLI handler for 'start master' command
func NewCommandStartMaster(basename string, out, errout io.Writer) (*cobra.Command, *MasterOptions) {
	options := &MasterOptions{Output: out}
	options.DefaultsFromName(basename)

	cmd := &cobra.Command{
		Use:   "master",
		Short: "Launch a master",
		Long:  fmt.Sprintf(masterLong, basename),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete())
			kcmdutil.CheckErr(options.Validate(args))

			startProfiler()

			if err := options.StartMaster(); err != nil {
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
		},
	}

	options.MasterArgs = NewDefaultMasterArgs()
	options.MasterArgs.StartAPI = true
	options.MasterArgs.StartControllers = true
	options.MasterArgs.OverrideConfig = func(config *configapi.MasterConfig) error {
		if config.KubernetesMasterConfig != nil && options.MasterArgs.MasterAddr.Provided {
			if ip := net.ParseIP(options.MasterArgs.MasterAddr.Host); ip != nil {
				glog.V(2).Infof("Using a masterIP override %q", ip)
				config.KubernetesMasterConfig.MasterIP = ip.String()
			}
		}
		return nil
	}

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

	startControllers, _ := NewCommandStartMasterControllers("controllers", basename, out, errout)
	startAPI, _ := NewCommandStartMasterAPI("api", basename, out, errout)
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

	if o.MasterArgs.OverrideConfig != nil {
		if err := o.MasterArgs.OverrideConfig(masterConfig); err != nil {
			return err
		}
	}

	// Inject disabled feature flags based on distribution being used and
	// regardless of configuration. They aren't written to config file to
	// prevent upgrade path issues.
	masterConfig.DisabledFeatures.Add(o.DisabledFeatures...)
	validationResults := validation.ValidateMasterConfig(masterConfig, nil)
	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("Warning: %v, master start will continue.", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return kerrors.NewInvalid(configapi.Kind("MasterConfig"), o.ConfigFile, validationResults.Errors)
	}

	if !o.MasterArgs.StartControllers {
		masterConfig.Controllers = configapi.ControllersDisabled
	}

	m := &Master{
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
		APIServerCAFiles:   o.MasterArgs.APIServerCAFiles,
		CABundleFile:       admin.DefaultCABundleFile(o.MasterArgs.ConfigDir.Value()),
		PublicAPIServerURL: publicMasterAddr.String(),
		Output:             cmdutil.NewGLogWriterV(3),
	}
	if err := mintAllCertsOptions.Validate(nil); err != nil {
		return err
	}
	if err := mintAllCertsOptions.CreateMasterCerts(); err != nil {
		return err
	}

	return nil
}

func BuildKubernetesMasterConfig(openshiftConfig *origin.MasterConfig) (*kubernetes.MasterConfig, error) {
	if openshiftConfig.Options.KubernetesMasterConfig == nil {
		return nil, nil
	}
	kubeConfig, err := kubernetes.BuildKubernetesMasterConfig(openshiftConfig.Options, openshiftConfig.RequestContextMapper, openshiftConfig.KubeClient(), openshiftConfig.Informers, openshiftConfig.KubeAdmissionControl, openshiftConfig.Authenticator)
	return kubeConfig, err
}

// Master encapsulates starting the components of the master
type Master struct {
	config      *configapi.MasterConfig
	controllers bool
	api         bool
}

// NewMaster create a master launcher
func NewMaster(config *configapi.MasterConfig, controllers, api bool) *Master {
	return &Master{
		config:      config,
		controllers: controllers,
		api:         api,
	}
}

// Start launches a master. It will error if possible, but some background processes may still
// be running and the process should exit after it finishes.
func (m *Master) Start() error {
	// Allow privileged containers
	// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged: true,
		PrivilegedSources: capabilities.PrivilegedSources{
			HostNetworkSources: []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostPIDSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostIPCSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
		},
	})

	openshiftConfig, err := origin.BuildMasterConfig(*m.config)
	if err != nil {
		return err
	}

	kubeMasterConfig, err := BuildKubernetesMasterConfig(openshiftConfig)
	if err != nil {
		return err
	}

	// any controller that uses a core informer must be initialized *before* the API server starts core informers
	// the API server adds its controllers at the correct time, but if the controllers are running, they need to be
	// kicked separately

	switch {
	case m.api:
		glog.Infof("Starting master on %s (%s)", m.config.ServingInfo.BindAddress, version.Get().String())
		glog.Infof("Public master address is %s", m.config.AssetConfig.MasterPublicURL)
		if len(m.config.DisabledFeatures) > 0 {
			glog.V(4).Infof("Disabled features: %s", strings.Join(m.config.DisabledFeatures, ", "))
		}
		glog.Infof("Using images from %q", openshiftConfig.ImageFor("<component>"))

		if err := StartAPI(openshiftConfig, kubeMasterConfig); err != nil {
			return err
		}

	case m.controllers:
		glog.Infof("Starting controllers on %s (%s)", m.config.ServingInfo.BindAddress, version.Get().String())
		if len(m.config.DisabledFeatures) > 0 {
			glog.V(4).Infof("Disabled features: %s", strings.Join(m.config.DisabledFeatures, ", "))
		}
		glog.Infof("Using images from %q", openshiftConfig.ImageFor("<component>"))

		if err := startHealth(openshiftConfig); err != nil {
			return err
		}
	}

	if m.controllers {
		// run controllers asynchronously (not required to be "ready")
		go func() {
			if err := startControllers(openshiftConfig, kubeMasterConfig); err != nil {
				glog.Fatal(err)
			}

			openshiftConfig.Informers.Start(utilwait.NeverStop)
			openshiftConfig.Informers.StartCore(utilwait.NeverStop)
		}()
	} else {
		openshiftConfig.Informers.Start(utilwait.NeverStop)

	}

	return nil
}

func startHealth(openshiftConfig *origin.MasterConfig) error {
	openshiftConfig.RunHealth()
	return nil
}

// StartAPI starts the components of the master that are considered part of the API - the Kubernetes
// API and core controllers, the Origin API, the group, policy, project, and authorization caches,
// etcd, the asset server (for the UI), the OAuth server endpoints, and the DNS server.
// TODO: allow to be more granularly targeted
func StartAPI(oc *origin.MasterConfig, kc *kubernetes.MasterConfig) error {
	// start etcd
	if oc.Options.EtcdConfig != nil {
		etcdserver.RunEtcd(oc.Options.EtcdConfig)
	}

	// verify we can connect to etcd with the provided config
	if _, err := etcd.GetAndTestEtcdClient(oc.Options.EtcdClientInfo); err != nil {
		return err
	}

	// Must start policy caching immediately
	oc.Informers.StartCore(utilwait.NeverStop)
	oc.RunClusterQuotaMappingController()
	oc.RunGroupCache()
	oc.RunProjectCache()

	unprotectedInstallers := []origin.APIInstaller{}

	if oc.Options.OAuthConfig != nil {
		authConfig, err := origin.BuildAuthConfig(oc)
		if err != nil {
			return err
		}
		unprotectedInstallers = append(unprotectedInstallers, authConfig)
	}

	var standaloneAssetConfig *origin.AssetConfig
	if oc.WebConsoleEnabled() {
		overrideConfig, err := getResourceOverrideConfig(oc)
		if err != nil {
			return err
		}
		config, err := origin.NewAssetConfig(*oc.Options.AssetConfig, overrideConfig)
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
		_, kubeClientConfig, err := configapi.GetKubeClient(oc.Options.MasterClients.ExternalKubernetesKubeConfig, oc.Options.MasterClients.ExternalKubernetesClientConnectionOverrides)
		if err != nil {
			return err
		}
		proxy := &kubernetes.ProxyConfig{
			ClientConfig: kubeClientConfig,
		}
		oc.Run([]origin.APIInstaller{proxy}, unprotectedInstallers)
	}

	// start up the informers that we're trying to use in the API server
	oc.Informers.Start(utilwait.NeverStop)
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

// getResourceOverrideConfig looks in two potential places where ClusterResourceOverrideConfig can be specified
func getResourceOverrideConfig(oc *origin.MasterConfig) (*overrideapi.ClusterResourceOverrideConfig, error) {
	overrideConfig, err := checkForOverrideConfig(oc.Options.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	if overrideConfig != nil {
		return overrideConfig, nil
	}
	if oc.Options.KubernetesMasterConfig == nil { // external kube gets you a nil pointer here
		return nil, nil
	}
	overrideConfig, err = checkForOverrideConfig(oc.Options.KubernetesMasterConfig.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	return overrideConfig, nil
}

// checkForOverrideConfig looks for ClusterResourceOverrideConfig plugin cfg in the admission PluginConfig
func checkForOverrideConfig(ac configapi.AdmissionConfig) (*overrideapi.ClusterResourceOverrideConfig, error) {
	overridePluginConfigFile, err := pluginconfig.GetPluginConfigFile(ac.PluginConfig, overrideapi.PluginName, "")
	if err != nil {
		return nil, err
	}
	if overridePluginConfigFile == "" {
		return nil, nil
	}
	configFile, err := os.Open(overridePluginConfigFile)
	if err != nil {
		return nil, err
	}
	overrideConfig, err := override.ReadConfig(configFile)
	if err != nil {
		return nil, err
	}
	return overrideConfig, nil
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
		if err := oc.ControllerPlug.WaitForStop(); err != nil {
			glog.Fatalf("Controller shutdown due to lease being lost: %v", err)
		}
		glog.Fatalf("Controller graceful shutdown requested")
	}()

	oc.ControllerPlug.WaitForStart()
	glog.Infof("Controllers starting (%s)", oc.Options.Controllers)

	// Get configured options (or defaults) for k8s controllers
	controllerManagerOptions := cmapp.NewCMServer()
	if kc != nil && kc.ControllerManager != nil {
		controllerManagerOptions = kc.ControllerManager
	}

	// Start these first, because they provide credentials for other controllers' clients
	oc.RunServiceAccountsController()
	oc.RunServiceAccountTokensController(controllerManagerOptions)
	// used by admission controllers
	oc.RunServiceAccountPullSecretsControllers()
	oc.RunSecurityAllocationController()

	if kc != nil {
		_, _, rcClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraReplicationControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for replication controller: %v", err)
		}
		_, _, rsClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraReplicaSetControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for replication controller: %v", err)
		}
		_, _, deploymentClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraDeploymentControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for deployment controller: %v", err)
		}
		jobConfig, _, jobClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraJobControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for job controller: %v", err)
		}
		_, hpaOClient, hpaKClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraHPAControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for HPA controller: %v", err)
		}

		_, _, binderClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraPersistentVolumeBinderControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for persistent volume binder controller: %v", err)
		}

		_, _, attachDetachControllerClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraPersistentVolumeAttachDetachControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for attach detach controller: %v", err)
		}

		_, _, daemonSetClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraDaemonSetControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for daemonset controller: %v", err)
		}

		_, _, disruptionClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraDisruptionControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for disruption budget controller: %v", err)
		}

		_, _, gcClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraGCControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for pod gc controller: %v", err)
		}

		_, _, serviceLoadBalancerClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraServiceLoadBalancerControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for pod gc controller: %v", err)
		}

		_, _, petSetClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraPetSetControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for pet set controller: %v", err)
		}

		namespaceControllerClientConfig, _, namespaceControllerKubeClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraNamespaceControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for namespace controller: %v", err)
		}
		namespaceControllerClientSet := clientadapter.FromUnversionedClient(namespaceControllerKubeClient)
		namespaceControllerClientPool := dynamic.NewClientPool(namespaceControllerClientConfig, dynamic.LegacyAPIPathResolverFunc)

		_, _, endpointControllerClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraEndpointControllerServiceAccountName)
		if err != nil {
			glog.Fatalf("Could not get client for endpoint controller: %v", err)
		}

		// no special order
		kc.RunNodeController()
		kc.RunScheduler()
		kc.RunReplicationController(rcClient)
		kc.RunReplicaSetController(rsClient)
		kc.RunDeploymentController(deploymentClient)

		extensionsEnabled := len(configapi.GetEnabledAPIVersionsForGroup(kc.Options, extensions.GroupName)) > 0

		batchEnabled := len(configapi.GetEnabledAPIVersionsForGroup(kc.Options, batch.GroupName)) > 0
		if extensionsEnabled || batchEnabled {
			kc.RunJobController(jobClient)
		}
		if batchEnabled {
			kc.RunScheduledJobController(jobConfig)
		}
		// TODO: enable this check once the HPA controller can use the autoscaling API if the extensions API is disabled
		autoscalingEnabled := len(configapi.GetEnabledAPIVersionsForGroup(kc.Options, autoscaling.GroupName)) > 0
		if extensionsEnabled || autoscalingEnabled {
			kc.RunHPAController(hpaOClient, hpaKClient, oc.Options.PolicyConfig.OpenShiftInfrastructureNamespace)
		}
		if extensionsEnabled {
			kc.RunDaemonSetsController(daemonSetClient)
		}

		policyEnabled := len(configapi.GetEnabledAPIVersionsForGroup(kc.Options, policy.GroupName)) > 0
		if policyEnabled {
			kc.RunDisruptionBudgetController(disruptionClient)
		}

		kc.RunEndpointController(endpointControllerClient)
		kc.RunNamespaceController(namespaceControllerClientSet, namespaceControllerClientPool)
		kc.RunPersistentVolumeController(binderClient, oc.Options.PolicyConfig.OpenShiftInfrastructureNamespace, oc.ImageFor("recycler"), bootstrappolicy.InfraPersistentVolumeRecyclerControllerServiceAccountName)
		kc.RunPersistentVolumeAttachDetachController(attachDetachControllerClient)
		kc.RunGCController(gcClient)

		kc.RunServiceLoadBalancerController(serviceLoadBalancerClient)

		appsEnabled := len(configapi.GetEnabledAPIVersionsForGroup(kc.Options, apps.GroupName)) > 0
		if appsEnabled {
			kc.RunPetSetController(petSetClient)
		}

		glog.Infof("Started Kubernetes Controllers")
	}

	// no special order
	if configapi.IsBuildEnabled(&oc.Options) {
		err := oc.RunBuildController(oc.Informers)
		if err != nil {
			glog.Fatalf("Could not start build controller: %v", err)
			return err
		}
		oc.RunBuildPodController()
		oc.RunBuildConfigChangeController()
		oc.RunBuildImageChangeTriggerController()
	}
	oc.RunDeploymentController()
	oc.RunDeploymentConfigController()
	oc.RunDeploymentTriggerController()
	oc.RunImageImportController()
	oc.RunOriginNamespaceController()
	oc.RunSDNController()

	// initializes quota docs used by admission
	oc.RunResourceQuotaManager(nil)
	oc.RunClusterQuotaReconciliationController()
	oc.RunClusterQuotaMappingController()

	_, _, serviceServingCertClient, err := oc.GetServiceAccountClients(bootstrappolicy.ServiceServingCertServiceAccountName)
	if err != nil {
		glog.Fatalf("Could not get client: %v", err)
	}
	oc.RunServiceServingCertController(serviceServingCertClient)
	oc.RunUnidlingController()

	_, _, ingressIPClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraServiceIngressIPControllerServiceAccountName)
	if err != nil {
		glog.Fatalf("Could not get client: %v", err)
	}
	oc.RunIngressIPController(ingressIPClient)

	glog.Infof("Started Origin Controllers")

	return nil
}

func (o MasterOptions) IsWriteConfigOnly() bool {
	return o.MasterArgs.ConfigDir.Provided()
}

func (o MasterOptions) IsRunFromConfig() bool {
	return (len(o.ConfigFile) > 0)
}
