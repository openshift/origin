package router

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

// Config contains all the necessary bits for client configuration
type Config struct {
	// CommonConfig is the shared base config for both the OpenShift config and Kubernetes config
	CommonConfig restclient.Config
	// Namespace is the namespace to act in
	Namespace string

	clientConfig clientcmd.ClientConfig
}

// NewConfig returns a new configuration
func NewConfig() *Config {
	return &Config{}
}

// Bind binds configuration values to the passed flagset
func (cfg *Config) Bind(flags *pflag.FlagSet) {
	cfg.clientConfig = DefaultClientConfig(flags)
}

// Clients returns an OpenShift and a Kubernetes client from a given configuration
func (cfg *Config) Clients() (kclientset.Interface, error) {
	config, _, err := cfg.KubeConfig()
	if err != nil {
		return nil, fmt.Errorf("Unable to configure Kubernetes client: %v", err)
	}

	kubeClientset, err := kclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Unable to configure Kubernetes client: %v", err)
	}

	return kubeClientset, nil
}

// KubeConfig returns the Kubernetes configuration
func (cfg *Config) KubeConfig() (*restclient.Config, string, error) {
	clientConfig, err := cfg.clientConfig.ClientConfig()
	if err != nil {
		return nil, "", err
	}
	namespace, _, err := cfg.clientConfig.Namespace()
	if err != nil {
		return nil, "", err
	}
	return clientConfig, namespace, nil
}

func DefaultClientConfig(flags *pflag.FlagSet) kclientcmd.ClientConfig {
	loadingRules := kclientcmd.NewDefaultClientConfigLoadingRules()
	flags.StringVar(&loadingRules.ExplicitPath, genericclioptions.OpenShiftKubeConfigFlagName, "", "Path to the config file to use for CLI requests.")
	cobra.MarkFlagFilename(flags, genericclioptions.OpenShiftKubeConfigFlagName)

	// set our explicit defaults
	defaultOverrides := &kclientcmd.ConfigOverrides{ClusterDefaults: kclientcmdapi.Cluster{Server: os.Getenv("KUBERNETES_MASTER")}}
	loadingRules.DefaultClientConfig = kclientcmd.NewDefaultClientConfig(kclientcmdapi.Config{}, defaultOverrides)

	overrides := &kclientcmd.ConfigOverrides{ClusterDefaults: defaultOverrides.ClusterDefaults}
	overrideFlags := kclientcmd.RecommendedConfigOverrideFlags("")
	overrideFlags.ContextOverrideFlags.Namespace.ShortName = "n"
	overrideFlags.AuthOverrideFlags.Username.LongName = ""
	overrideFlags.AuthOverrideFlags.Password.LongName = ""
	kclientcmd.BindOverrideFlags(overrides, flags, overrideFlags)
	cobra.MarkFlagFilename(flags, overrideFlags.AuthOverrideFlags.ClientCertificate.LongName)
	cobra.MarkFlagFilename(flags, overrideFlags.AuthOverrideFlags.ClientKey.LongName)
	cobra.MarkFlagFilename(flags, overrideFlags.ClusterOverrideFlags.CertificateAuthority.LongName)

	clientConfig := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	return clientConfig
}
