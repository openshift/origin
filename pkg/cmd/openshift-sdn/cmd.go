package openshift_sdn

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"k8s.io/klog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
	"k8s.io/kubernetes/pkg/util/interrupt"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/network/networkconfig"
	sdnnode "github.com/openshift/origin/pkg/network/node"
	sdnproxy "github.com/openshift/origin/pkg/network/proxy"
	"github.com/openshift/origin/pkg/version"
)

// OpenShiftSDN stores the variables needed to initialize the real networking
// processess from the command line.
type OpenShiftSDN struct {
	ConfigFilePath            string
	KubeConfigFilePath        string
	URLOnlyKubeConfigFilePath string

	NodeConfig  *legacyconfigv1.NodeConfig
	ProxyConfig *kubeproxyconfig.KubeProxyConfiguration

	informers   *informers
	OsdnNode    *sdnnode.OsdnNode
	sdnRecorder record.EventRecorder
	OsdnProxy   *sdnproxy.OsdnProxy
}

var networkLong = `
Start OpenShift SDN node components. This includes the service proxy.

This will also read the node name from the environment variable K8S_NODE_NAME.`

func NewOpenShiftSDNCommand(basename string, errout io.Writer) *cobra.Command {
	sdn := &OpenShiftSDN{}

	cmd := &cobra.Command{
		Use:   basename,
		Short: "Start OpenShiftSDN",
		Long:  networkLong,
		Run: func(c *cobra.Command, _ []string) {
			ch := make(chan struct{})
			interrupt.New(func(s os.Signal) {
				fmt.Fprintf(errout, "interrupt: Gracefully shutting down ...\n")
				close(ch)
			}).Run(func() error {
				sdn.Run(c, errout, ch)
				return nil
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&sdn.ConfigFilePath, "config", "", "Location of the node configuration file to run from (required)")
	flags.StringVar(&sdn.KubeConfigFilePath, "kubeconfig", "", "Path to the kubeconfig file to use for requests to the Kubernetes API. Optional. When omitted, will use the in-cluster config")
	flags.StringVar(&sdn.URLOnlyKubeConfigFilePath, "url-only-kubeconfig", "", "Path to a kubeconfig file to use, but only to determine the URL to the apiserver. The in-cluster credentials will be used. Cannot use with --kubeconfig.")

	return cmd
}

// Run starts the network process. Does not return.
func (sdn *OpenShiftSDN) Run(c *cobra.Command, errout io.Writer, stopCh chan struct{}) {
	err := injectKubeAPIEnv(sdn.URLOnlyKubeConfigFilePath)
	if err != nil {
		klog.Fatal(err)
	}

	// Parse config file, build config objects
	err = sdn.ValidateAndParse()
	if err != nil {
		if kerrors.IsInvalid(err) {
			if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
				fmt.Fprintf(errout, "Invalid %s %s\n", details.Kind, details.Name)
				for _, cause := range details.Causes {
					fmt.Fprintf(errout, "  %s: %s\n", cause.Field, cause.Message)
				}
				os.Exit(255)
			}
		}
		klog.Fatal(err)
	}

	// Set up a watch on our config file; if it changes, we should exit -
	// (we don't have the ability to dynamically reload config changes).
	if err := watchForChanges(sdn.ConfigFilePath, stopCh); err != nil {
		klog.Fatalf("unable to setup configuration watch: %v", err)
	}

	// Build underlying network objects
	err = sdn.Init()
	if err != nil {
		klog.Fatalf("Failed to initialize sdn: %v", err)
	}

	err = sdn.Start(stopCh)
	if err != nil {
		klog.Fatalf("Failed to start sdn: %v", err)
	}

	<-stopCh
	time.Sleep(500 * time.Millisecond) // gracefully shut down
}

// ValidateAndParse validates the command line options, parses the node
// configuration, and builds the upstream proxy configuration.
func (sdn *OpenShiftSDN) ValidateAndParse() error {
	if len(sdn.ConfigFilePath) == 0 {
		return errors.New("--config is required")
	}

	if len(sdn.KubeConfigFilePath) > 0 && len(sdn.URLOnlyKubeConfigFilePath) > 0 {
		return errors.New("cannot pass --kubeconfig and --url-only-kubeconfig")
	}

	klog.V(2).Infof("Reading node configuration from %s", sdn.ConfigFilePath)
	var err error
	sdn.NodeConfig, err = readAndResolveNodeConfig(sdn.ConfigFilePath)
	if err != nil {
		return err
	}

	if len(sdn.KubeConfigFilePath) > 0 {
		sdn.NodeConfig.MasterKubeConfig = sdn.KubeConfigFilePath
	}

	// Get the nodename from the environment, if available
	if len(sdn.NodeConfig.NodeName) == 0 {
		sdn.NodeConfig.NodeName = os.Getenv("K8S_NODE_NAME")
	}

	// Validate the node config
	validationResults := networkconfig.ValidateInClusterNetworkNodeConfig(sdn.NodeConfig, nil)

	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			klog.Warningf("Warning: %v, node start will continue.", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		klog.V(4).Infof("Configuration is invalid: %#v", sdn.NodeConfig)
		return kerrors.NewInvalid(schema.GroupKind{Group: "", Kind: "NodeConfig"}, sdn.ConfigFilePath, validationResults.Errors)
	}

	sdn.ProxyConfig, err = ProxyConfigFromNodeConfig(*sdn.NodeConfig)
	if err != nil {
		klog.V(4).Infof("Unable to build proxy config: %v", err)
		return err
	}

	return nil
}

// Init builds the underlying structs for the network processes.
func (sdn *OpenShiftSDN) Init() error {
	// Build the informers
	var err error
	err = sdn.buildInformers()
	if err != nil {
		return fmt.Errorf("failed to build informers: %v", err)
	}

	// Configure SDN
	err = sdn.initSDN()
	if err != nil {
		return fmt.Errorf("failed to initialize SDN: %v", err)
	}

	// Configure the proxy
	err = sdn.initProxy()
	if err != nil {
		return fmt.Errorf("failed to initialize proxy: %v", err)
	}

	return nil
}

// Start starts the network, proxy, and informers, then returns.
func (sdn *OpenShiftSDN) Start(stopCh <-chan struct{}) error {
	klog.Infof("Starting node networking (%s)", version.Get().String())

	serviceability.StartProfiler()
	err := sdn.runSDN()
	if err != nil {
		return err
	}
	proxyInitChan := make(chan bool)
	sdn.runProxy(proxyInitChan)
	sdn.informers.start(stopCh)

	klog.V(2).Infof("openshift-sdn network plugin waiting for proxy startup to comlete")
	<-proxyInitChan
	klog.V(2).Infof("openshift-sdn network plugin registering startup")
	if err := sdn.writeConfigFile(); err != nil {
		klog.Fatal(err)
	}
	klog.V(2).Infof("openshift-sdn network plugin ready")
	return nil
}

// injectKubeAPIEnv consumes the url-only-kubeconfig and re-injects it as
// environment variables. We need to do this because we cannot use the
// apiserver service ip (since we set it up!), but we want to use the in-cluster
// configuration. So, take the server URL from the kubelet kubeconfig.
func injectKubeAPIEnv(kcPath string) error {
	if kcPath != "" {
		kubeconfig, err := clientcmd.LoadFromFile(kcPath)
		if err != nil {
			return err
		}
		clusterName := kubeconfig.Contexts[kubeconfig.CurrentContext].Cluster
		apiURL := kubeconfig.Clusters[clusterName].Server

		url, err := url.Parse(apiURL)
		if err != nil {
			return err
		}

		// The kubernetes in-cluster functions don't let you override the apiserver
		// directly; gotta "pass" it via environment vars.
		klog.V(2).Infof("Overriding kubernetes api to %s", apiURL)
		os.Setenv("KUBERNETES_SERVICE_HOST", url.Hostname())
		os.Setenv("KUBERNETES_SERVICE_PORT", url.Port())
	}
	return nil
}

// watchForChanges closes stopCh if the configuration file changed.
func watchForChanges(configPath string, stopCh chan struct{}) error {
	configPath, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Watch all symlinks for changes
	p := configPath
	maxdepth := 100
	for depth := 0; depth < maxdepth; depth++ {
		if err := watcher.Add(p); err != nil {
			return err
		}
		klog.V(2).Infof("Watching config file %s for changes", p)

		stat, err := os.Lstat(p)
		if err != nil {
			return err
		}

		// configmaps are usually symlinks
		if stat.Mode()&os.ModeSymlink > 0 {
			p, err = filepath.EvalSymlinks(p)
			if err != nil {
				return err
			}
		} else {
			break
		}
	}

	go func() {
		for {
			select {
			case <-stopCh:
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				klog.V(2).Infof("Configuration file %s changed, exiting...", event.Name)
				close(stopCh)
				return
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				klog.V(4).Infof("fsnotify error %v", err)
			}
		}
	}()
	return nil
}
