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

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/openshift-controller-manager"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	aggregatorinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/capabilities"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/etcd/etcdserver"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/version"
)

type MasterOptions struct {
	MasterArgs *MasterArgs

	CreateCertificates bool
	ExpireDays         int
	SignerExpireDays   int
	ConfigFile         string
	Output             io.Writer
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

	You may also pass --etcd=<address> to connect to an external etcd server.`)

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

			origin.StartProfiler()

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
		if options.MasterArgs.MasterAddr.Provided {
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

	validationResults := validation.ValidateMasterConfig(masterConfig, nil)
	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("Warning: %v, master start will continue.", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return kerrors.NewInvalid(configapi.Kind("MasterConfig"), o.ConfigFile, validationResults.Errors)
	}

	m := &Master{
		config:      masterConfig,
		api:         o.MasterArgs.StartAPI,
		controllers: o.MasterArgs.StartControllers,
	}
	return m.Start()
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

	// install aggregator types into the scheme so that "normal" RESTOptionsGetters can work for us.
	// done in Start() prior to doing any other initialization so we don't mutate the scheme after it is being used by clients in other goroutines.
	// TODO: make scheme threadsafe and do this as part of aggregator config building
	aggregatorinstall.Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)

	controllersEnabled := m.controllers && len(m.config.ControllerConfig.Controllers) > 0
	if controllersEnabled {
		privilegedLoopbackConfig, err := configapi.GetClientConfig(m.config.MasterClients.OpenShiftLoopbackKubeConfig, m.config.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
		if err != nil {
			return err
		}

		go runEmbeddedScheduler(
			m.config.MasterClients.OpenShiftLoopbackKubeConfig,
			m.config.KubernetesMasterConfig.SchedulerConfigFile,
			privilegedLoopbackConfig.QPS,
			privilegedLoopbackConfig.Burst,
			m.config.KubernetesMasterConfig.SchedulerArguments,
		)

		go func() {
			kubeControllerConfigShallowCopy := *m.config
			// this creates using 0700
			kubeControllerConfigDir, err := ioutil.TempDir("", "openshift-kube-controller-manager-config-")
			if err != nil {
				glog.Fatal(err)
			}
			defer func() {
				os.RemoveAll(kubeControllerConfigDir)
			}()
			if m.config.ControllerConfig.ServiceServingCert.Signer != nil && len(m.config.ControllerConfig.ServiceServingCert.Signer.CertFile) > 0 {
				caBytes, err := ioutil.ReadFile(m.config.ControllerConfig.ServiceServingCert.Signer.CertFile)
				if err != nil {
					glog.Fatal(err)
				}
				serviceServingCertSignerCAFile := path.Join(kubeControllerConfigDir, "service-signer.crt")
				if err := ioutil.WriteFile(serviceServingCertSignerCAFile, caBytes, 0644); err != nil {
					glog.Fatal(err)
				}

				// we need to tweak the master config file with a relative ref, but to do that we need to copy it
				kubeControllerConfigShallowCopy.ControllerConfig.ServiceServingCert.Signer = &configapi.CertInfo{CertFile: "service-signer.crt"}
			}
			kubeControllerConfigBytes, err := configapilatest.WriteYAML(&kubeControllerConfigShallowCopy)
			if err != nil {
				glog.Fatal(err)
			}
			masterConfigFile := path.Join(kubeControllerConfigDir, "master-config.yaml")
			if err := ioutil.WriteFile(masterConfigFile, kubeControllerConfigBytes, 0644); err != nil {
				glog.Fatal(err)
			}

			runEmbeddedKubeControllerManager(
				m.config.MasterClients.OpenShiftLoopbackKubeConfig,
				m.config.ServiceAccountConfig.PrivateKeyFile,
				m.config.ServiceAccountConfig.MasterCA,
				m.config.KubernetesMasterConfig.PodEvictionTimeout,
				masterConfigFile,
				m.config.VolumeConfig.DynamicProvisioningEnabled,
				privilegedLoopbackConfig.QPS,
				privilegedLoopbackConfig.Burst,
			)
		}()

		openshiftControllerConfig := openshift_controller_manager.ConvertMasterConfigToOpenshiftControllerConfig(m.config)
		// if we're starting the API, then this one isn't supposed to serve
		if m.api {
			openshiftControllerConfig.ServingInfo = nil
		}

		if err := openshift_controller_manager.RunOpenShiftControllerManager(openshiftControllerConfig, privilegedLoopbackConfig); err != nil {
			return err
		}

	}

	if m.api {
		// informers are shared amongst all the various api components we build
		// TODO the needs of the apiserver and the controllers are drifting. We should consider two different skins here
		clientConfig, err := configapi.GetClientConfig(m.config.MasterClients.OpenShiftLoopbackKubeConfig, m.config.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
		if err != nil {
			return err
		}
		informers, err := origin.NewInformers(clientConfig)
		if err != nil {
			return err
		}
		if err := informers.AddUserIndexes(); err != nil {
			return err
		}

		openshiftConfig, err := origin.BuildMasterConfig(*m.config, informers)
		if err != nil {
			return err
		}

		glog.Infof("Starting master on %s (%s)", m.config.ServingInfo.BindAddress, version.Get().String())
		glog.Infof("Public master address is %s", m.config.MasterPublicURL)
		imageTemplate := variable.NewDefaultImageTemplate()
		imageTemplate.Format = m.config.ImageConfig.Format
		imageTemplate.Latest = m.config.ImageConfig.Latest
		glog.Infof("Using images from %q", imageTemplate.ExpandOrDie("<component>"))

		if err := StartAPI(openshiftConfig); err != nil {
			return err
		}
	}

	return nil
}

// StartAPI starts the components of the master that are considered part of the API - the Kubernetes
// API and core controllers, the Origin API, the group, policy, project, and authorization caches,
// etcd, the asset server (for the UI), the OAuth server endpoints, and the DNS server.
// TODO: allow to be more granularly targeted
func StartAPI(oc *origin.MasterConfig) error {
	// start etcd
	if oc.Options.EtcdConfig != nil {
		etcdserver.RunEtcd(oc.Options.EtcdConfig)
	}

	// verify we can connect to etcd with the provided config
	// TODO remove when this becomes a health check in 3.8
	if err := testEtcdConnectivity(oc.Options.EtcdClientInfo); err != nil {
		return err
	}

	// start DNS before the informers are started because it adds a ClusterIP index.
	if oc.Options.DNSConfig != nil {
		oc.RunDNSServer()
	}

	if err := oc.Run(utilwait.NeverStop); err != nil {
		return err
	}

	return nil
}

func testEtcdConnectivity(etcdClientInfo configapi.EtcdConnectionInfo) error {
	// try etcd3 otherwise
	etcdClient3, err := etcd.MakeEtcdClientV3(etcdClientInfo)
	if err != nil {
		return err
	}
	if err := etcd.TestEtcdClientV3(etcdClient3); err != nil {
		return err
	}

	return nil
}

func (o MasterOptions) IsWriteConfigOnly() bool {
	return o.MasterArgs.ConfigDir.Provided()
}

func (o MasterOptions) IsRunFromConfig() bool {
	return (len(o.ConfigFile) > 0)
}
