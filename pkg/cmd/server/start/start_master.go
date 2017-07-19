package start

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	aggregatorinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/capabilities"
	kinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/etcd/etcdserver"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	origincontrollers "github.com/openshift/origin/pkg/cmd/server/origin/controller"
	originrest "github.com/openshift/origin/pkg/cmd/server/origin/rest"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/plug"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	override "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	"github.com/openshift/origin/pkg/version"
)

type MasterOptions struct {
	MasterArgs *MasterArgs

	CreateCertificates bool
	ExpireDays         int
	SignerExpireDays   int
	ConfigFile         string
	Output             io.Writer
	DisabledFeatures   []string
}

func (o *MasterOptions) DefaultsFromName(basename string) {}

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
	options := &MasterOptions{
		ExpireDays:       crypto.DefaultCertificateLifetimeInDays,
		SignerExpireDays: crypto.DefaultCACertificateLifetimeInDays,
		Output:           out,
	}
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
	flags.IntVar(&options.ExpireDays, "expire-days", options.ExpireDays, "Validity of the certificates in days (defaults to 2 years). WARNING: extending this above default value is highly discouraged.")
	flags.IntVar(&options.SignerExpireDays, "signer-expire-days", options.SignerExpireDays, "Validity of the CA certificate in days (defaults to 5 years). WARNING: extending this above default value is highly discouraged.")

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

	if o.ExpireDays < 0 {
		return errors.New("expire-days must be valid number of days")
	}
	if o.SignerExpireDays < 0 {
		return errors.New("signer-expire-days must be valid number of days")
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
	go daemon.SdNotify(false, "READY=1")
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
		ExpireDays:         o.ExpireDays,
		SignerExpireDays:   o.SignerExpireDays,
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
	return kubernetes.BuildKubernetesMasterConfig(
		openshiftConfig.Options,
		openshiftConfig.RequestContextMapper,
		openshiftConfig.KubeClientsetExternal(),
		openshiftConfig.KubeClientsetInternal(),
		openshiftConfig.KubeAdmissionControl,
		openshiftConfig.Authenticator,
		openshiftConfig.Authorizer,
	)
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

	if m.config.KubernetesMasterConfig == nil {
		return fmt.Errorf("KubernetesMasterConfig is required to start this server - use of external Kubernetes is no longer supported.")
	}

	// install aggregator types into the scheme so that "normal" RESTOptionsGetters can work for us.
	// done in Start() prior to doing any other initialization so we don't mutate the scheme after it is being used by clients in other goroutines.
	// TODO: make scheme threadsafe and do this as part of aggregator config building
	aggregatorinstall.Install(kapi.GroupFactoryRegistry, kapi.Registry, kapi.Scheme)

	// we have a strange, optional linkage from controllers to the API server regarding the plug.  In the end, this should be structured
	// as a separate API server which can be chained as a delegate
	var controllerPlug plug.Plug

	controllersEnabled := m.controllers && m.config.Controllers != configapi.ControllersDisabled
	if controllersEnabled {
		// informers are shared amongst all the various controllers we build
		informers, err := NewInformers(*m.config)
		if err != nil {
			return err
		}
		kubeInternal, _, err := configapi.GetInternalKubeClient(m.config.MasterClients.OpenShiftLoopbackKubeConfig, m.config.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
		if err != nil {
			return err
		}

		// you can't double run healthz, so only do this next bit if we aren't starting the API
		if !m.api {
			imageTemplate := variable.NewDefaultImageTemplate()
			imageTemplate.Format = m.config.ImageConfig.Format
			imageTemplate.Latest = m.config.ImageConfig.Latest

			glog.Infof("Starting controllers on %s (%s)", m.config.ServingInfo.BindAddress, version.Get().String())
			if len(m.config.DisabledFeatures) > 0 {
				glog.V(4).Infof("Disabled features: %s", strings.Join(m.config.DisabledFeatures, ", "))
			}
			glog.Infof("Using images from %q", imageTemplate.ExpandOrDie("<component>"))

			if err := origin.RunControllerServer(m.config.ServingInfo, kubeInternal); err != nil {
				return err
			}
		}

		kubeControllerManagerConfig, cloudProvider, err := kubernetes.BuildControllerManagerServer(*m.config)
		if err != nil {
			return err
		}
		kubeExternal, _, err := configapi.GetExternalKubeClient(m.config.MasterClients.OpenShiftLoopbackKubeConfig, m.config.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
		if err != nil {
			return err
		}

		// run controllers asynchronously (not required to be "ready")
		var controllerPlugStart func()
		controllerPlug, controllerPlugStart, err = origin.NewLeaderElection(
			*m.config,
			kubeControllerManagerConfig.KubeControllerManagerConfiguration.LeaderElection,
			kubeExternal,
		)
		if err != nil {
			return err
		}

		// TODO refactor this controller so that it no longer relies on direct etcd access
		// these restoptions are used to directly access small keysets on etcd that do NOT overlap with access
		// by the main API server, so we aren't paying a large cost for the separation.
		restOptsGetter, err := originrest.StorageOptions(*m.config)
		if err != nil {
			return err
		}
		allocationController := origin.SecurityAllocationController{
			SecurityAllocator:          m.config.ProjectConfig.SecurityAllocator,
			OpenshiftRESTOptionsGetter: restOptsGetter,
			ExternalKubeInformers:      informers.GetExternalKubeInformers(),
			KubeExternalClient:         kubeExternal,
		}

		go func() {
			controllerPlugStart()
			// when a manual shutdown (DELETE /controllers) or lease lost occurs, the process should exit
			// this ensures no code is still running as a controller, and allows a process manager to reset
			// the controller to come back into a candidate state and compete for the lease
			if err := controllerPlug.WaitForStop(); err != nil {
				glog.Fatalf("Controller shutdown due to lease being lost: %v", err)
			}
			glog.Fatalf("Controller graceful shutdown requested")
		}()

		go func() {
			controllerPlug.WaitForStart()

			controllerContext, err := getControllerContext(*m.config, kubeControllerManagerConfig, cloudProvider, informers, utilwait.NeverStop)
			if err != nil {
				glog.Fatal(err)
			}

			if err := startControllers(*m.config, allocationController, informers, controllerContext); err != nil {
				glog.Fatal(err)
			}

			informers.Start(utilwait.NeverStop)
		}()
	}

	if m.api {
		// informers are shared amongst all the various api components we build
		informers, err := NewInformers(*m.config)
		if err != nil {
			return err
		}
		openshiftConfig, err := origin.BuildMasterConfig(*m.config, informers)
		if err != nil {
			return err
		}

		kubeMasterConfig, err := BuildKubernetesMasterConfig(openshiftConfig)
		if err != nil {
			return err
		}
		kubeMasterConfig.Master.GenericConfig.SharedInformerFactory = informers.GetClientGoKubeInformers()

		glog.Infof("Starting master on %s (%s)", m.config.ServingInfo.BindAddress, version.Get().String())
		glog.Infof("Public master address is %s", m.config.MasterPublicURL)
		if len(m.config.DisabledFeatures) > 0 {
			glog.V(4).Infof("Disabled features: %s", strings.Join(m.config.DisabledFeatures, ", "))
		}
		glog.Infof("Using images from %q", openshiftConfig.ImageFor("<component>"))

		if err := StartAPI(openshiftConfig, kubeMasterConfig, informers, controllerPlug); err != nil {
			return err
		}

		informers.Start(utilwait.NeverStop)
	}

	return nil
}

// StartAPI starts the components of the master that are considered part of the API - the Kubernetes
// API and core controllers, the Origin API, the group, policy, project, and authorization caches,
// etcd, the asset server (for the UI), the OAuth server endpoints, and the DNS server.
// TODO: allow to be more granularly targeted
func StartAPI(oc *origin.MasterConfig, kc *kubernetes.MasterConfig, informers *informers, controllerPlug plug.Plug) error {
	// start etcd
	if oc.Options.EtcdConfig != nil {
		etcdserver.RunEtcd(oc.Options.EtcdConfig)
	}

	// verify we can connect to etcd with the provided config
	if len(kc.Options.APIServerArguments) > 0 && len(kc.Options.APIServerArguments["storage-backend"]) > 0 && kc.Options.APIServerArguments["storage-backend"][0] == "etcd3" {
		if _, err := etcd.GetAndTestEtcdClientV3(oc.Options.EtcdClientInfo); err != nil {
			return err
		}
	} else {
		if _, err := etcd.GetAndTestEtcdClient(oc.Options.EtcdClientInfo); err != nil {
			return err
		}
	}

	// Must start policy and quota caching immediately
	oc.QuotaInformers.Start(utilwait.NeverStop)
	oc.AuthorizationInformers.Start(utilwait.NeverStop)
	clusterQuotaMapping := origincontrollers.ClusterQuotaMappingControllerConfig{
		ClusterQuotaMappingController: oc.ClusterQuotaMappingController,
	}
	clusterQuotaMapping.RunController(origincontrollers.ControllerContext{Stop: utilwait.NeverStop})
	oc.RunGroupCache()
	oc.RunProjectCache()

	var standaloneAssetConfig, embeddedAssetConfig *origin.AssetConfig
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
			embeddedAssetConfig = config
		} else {
			standaloneAssetConfig = config
		}
	}

	oc.Run(kc.Master, embeddedAssetConfig, controllerPlug, utilwait.NeverStop)

	// start DNS before the informers are started because it adds a ClusterIP index.
	if oc.Options.DNSConfig != nil {
		oc.RunDNSServer()
	}

	// start up the informers that we're trying to use in the API server
	informers.GetInternalKubeInformers().Start(utilwait.NeverStop)
	informers.GetExternalKubeInformers().Start(utilwait.NeverStop)
	oc.InitializeObjects()

	if standaloneAssetConfig != nil {
		standaloneAssetConfig.Run()
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

type GenericResourceInformer interface {
	ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error)
}

// genericInternalResourceInformerFunc will return an internal informer for any resource matching
// its group resource, instead of the external version. Only valid for use where the type is accessed
// via generic interfaces, such as the garbage collector with ObjectMeta.
type genericInternalResourceInformerFunc func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error)

func (fn genericInternalResourceInformerFunc) ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
	resource.Version = runtime.APIVersionInternal
	return fn(resource)
}

type genericInformers struct {
	kinformers.SharedInformerFactory
	generic []GenericResourceInformer
	// bias is a map that tries loading an informer from another GVR before using the original
	bias map[schema.GroupVersionResource]schema.GroupVersionResource
}

func (i genericInformers) ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
	if try, ok := i.bias[resource]; ok {
		if res, err := i.ForResource(try); err == nil {
			return res, nil
		}
	}

	informer, firstErr := i.SharedInformerFactory.ForResource(resource)
	if firstErr == nil {
		return informer, nil
	}
	for _, generic := range i.generic {
		if informer, err := generic.ForResource(resource); err == nil {
			return informer, nil
		}
	}
	glog.V(4).Infof("Couldn't find informer for %v", resource)
	return nil, firstErr
}

// startControllers launches the controllers
// allocation controller is passed in because it wants direct etcd access.  Naughty.
func startControllers(options configapi.MasterConfig, allocationController origin.SecurityAllocationController, informers *informers, controllerContext origincontrollers.ControllerContext) error {
	openshiftControllerConfig, err := origin.BuildOpenshiftControllerConfig(options, informers)
	if err != nil {
		return err
	}
	kubeControllerConfig, err := origin.BuildKubeControllerConfig(options)
	if err != nil {
		return err
	}

	// We need to start the serviceaccount-tokens controller first as it provides token
	// generation for other controllers.
	startSATokenController := openshiftControllerConfig.ServiceAccountContentControllerInit()
	if enabled, err := startSATokenController(controllerContext); err != nil {
		return fmt.Errorf("Error starting serviceaccount-token controller: %v", err)
	} else if !enabled {
		glog.Warningf("Skipping serviceaccount-token controller")
	} else {
		glog.Infof("Started serviceaccount-token controller")
	}

	// The service account controllers require informers in order to create service account tokens
	// for other controllers, which means we need to start their informers (which use the privileged
	// loopback client) before the other controllers will run.
	controllerContext.ExternalKubeInformers.Start(controllerContext.Stop)

	// right now we have controllers which are relying on the ability to make requests before the bootstrap policy is in place
	// In 3.7, we will be fixed by the post start hook that prevents readiness unless policy is in place
	// for 3.6, just make sure we don't proceed until the garbage collector can hit discovery
	// wait for bootstrap permissions to be established.  This check isn't perfect, but it ensures that at least the controllers checking discovery can succeed
	gcClientset := controllerContext.KubeControllerContext.ClientBuilder.ClientOrDie("generic-garbage-collector")
	err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
		result := gcClientset.Discovery().RESTClient().Get().AbsPath("/apis").Do()
		var statusCode int
		result.StatusCode(&statusCode)
		if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	//  the service account passed for the recyclable volume plugins needs to exist.  We want to do this via the init function, but its a kube init function
	// for the rebase, create that service account here
	// TODO make this a lot cleaner
	if _, err := controllerContext.ClientBuilder.Client(bootstrappolicy.InfraPersistentVolumeRecyclerControllerServiceAccountName); err != nil {
		return err
	}

	allocationController.RunSecurityAllocationController()

	kubernetesControllerInitializers, err := kubeControllerConfig.GetControllerInitializers()
	if err != nil {
		return err
	}
	openshiftControllerInitializers, err := openshiftControllerConfig.GetControllerInitializers()
	if err != nil {
		return err
	}
	// Add kubernetes controllers initialized from Origin
	for name, initFn := range kubernetesControllerInitializers {
		if _, exists := openshiftControllerInitializers[name]; exists {
			// don't overwrite, openshift takes priority
			continue
		}
		openshiftControllerInitializers[name] = origincontrollers.FromKubeInitFunc(initFn)
	}

	excludedControllers := getExcludedControllers(options)

	for controllerName, initFn := range openshiftControllerInitializers {
		// TODO remove this.  Only call one to start to prove the principle
		if excludedControllers.Has(controllerName) {
			glog.Warningf("%q is skipped", controllerName)
			continue
		}
		if !controllerContext.IsControllerEnabled(controllerName) {
			glog.Warningf("%q is disabled", controllerName)
			continue
		}

		glog.V(1).Infof("Starting %q", controllerName)
		started, err := initFn(controllerContext)
		if err != nil {
			glog.Fatalf("Error starting %q (%v)", controllerName, err)
			return err
		}
		if !started {
			glog.Warningf("Skipping %q", controllerName)
			continue
		}
		glog.Infof("Started %q", controllerName)
	}

	glog.Infof("Started Origin Controllers")

	return nil
}

func getExcludedControllers(options configapi.MasterConfig) sets.String {
	excludedControllers := sets.NewString(
		// not used in openshift.  Yet?
		"ttl",
		"bootstrapsigner",
		"tokencleaner",
	)
	if !configapi.IsBuildEnabled(&options) {
		excludedControllers.Insert("openshift.io/build")
		excludedControllers.Insert("openshift.io/build-config-change")
	}
	if options.TemplateServiceBrokerConfig == nil {
		excludedControllers.Insert("openshift.io/templateinstance")
	}

	return excludedControllers
}

func (o MasterOptions) IsWriteConfigOnly() bool {
	return o.MasterArgs.ConfigDir.Provided()
}

func (o MasterOptions) IsRunFromConfig() bool {
	return (len(o.ConfigFile) > 0)
}
