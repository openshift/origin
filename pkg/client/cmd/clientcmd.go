package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/client/config"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/util"
)

const ConfigSyntax = " --master=<addr>"

// Config contains all the necessary bits for client configuration
type Config struct {
	// MasterAddr is the address the master can be reached on (host, host:port, or URL).
	MasterAddr flagtypes.Addr
	// KubernetesAddr is the address of the Kubernetes server (host, host:port, or URL).
	// If omitted defaults to the master.
	KubernetesAddr flagtypes.Addr
	// CommonConfig is the shared base config for both the OpenShift config and Kubernetes config
	CommonConfig restclient.Config
	// Namespace is the namespace to act in
	Namespace string

	// If set, allow kubeconfig file loading
	FromFile bool
	// If true, no environment is loaded (for testing, primarily)
	SkipEnv      bool
	clientConfig clientcmd.ClientConfig
}

// NewConfig returns a new configuration
func NewConfig() *Config {
	return &Config{
		MasterAddr:     flagtypes.Addr{Value: "localhost:8080", DefaultScheme: "http", DefaultPort: 8080, AllowPrefix: true}.Default(),
		KubernetesAddr: flagtypes.Addr{Value: "localhost:8080", DefaultScheme: "http", DefaultPort: 8080}.Default(),
		CommonConfig:   restclient.Config{},
	}
}

// Bind binds configuration values to the passed flagset
func (cfg *Config) Bind(flags *pflag.FlagSet) {
	flags.Var(&cfg.MasterAddr, "master", "The address the master can be reached on (host, host:port, or URL).")
	flags.Var(&cfg.KubernetesAddr, "kubernetes", "The address of the Kubernetes server (host, host:port, or URL). If omitted defaults to the master.")

	if cfg.FromFile {
		cfg.clientConfig = DefaultClientConfig(flags)
	} else {
		BindClientConfigSecurityFlags(&cfg.CommonConfig, flags)
	}
}

// BindToFile is used when this config will not be bound to flags, but should load the config file
// from disk if available.
func (cfg *Config) BindToFile() *Config {
	cfg.clientConfig = DefaultClientConfig(pflag.NewFlagSet("empty", pflag.ContinueOnError))
	return cfg
}

// OpenShiftConfig returns the OpenShift configuration
func (cfg *Config) OpenShiftConfig() *restclient.Config {
	err := cfg.bindEnv()
	if err != nil {
		glog.Error(err)
	}

	osConfig := cfg.CommonConfig
	if len(osConfig.Host) == 0 || cfg.MasterAddr.Provided {
		osConfig.Host = cfg.MasterAddr.String()
	}

	return &osConfig
}

// Clients returns an OpenShift and a Kubernetes client from a given configuration
func (cfg *Config) Clients() (kclientset.Interface, error) {
	cfg.bindEnv()

	kubeClientset, err := kclientset.NewForConfig(cfg.KubeConfig())
	if err != nil {
		return nil, fmt.Errorf("Unable to configure Kubernetes client: %v", err)
	}

	return kubeClientset, nil
}

// BindClientConfigSecurityFlags adds flags for the supplied client config
func BindClientConfigSecurityFlags(config *restclient.Config, flags *pflag.FlagSet) {
	flags.BoolVar(&config.Insecure, "insecure-skip-tls-verify", config.Insecure, "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure.")
	flags.StringVar(&config.CertFile, "client-certificate", config.CertFile, "Path to a client certificate file for TLS.")
	flags.StringVar(&config.KeyFile, "client-key", config.KeyFile, "Path to a client key file for TLS.")
	flags.StringVar(&config.CAFile, "certificate-authority", config.CAFile, "Path to a cert. file for the certificate authority")
	flags.StringVar(&config.BearerToken, "token", config.BearerToken, "If present, the bearer token for this request.")
}

// KubeConfig returns the Kubernetes configuration
func (cfg *Config) KubeConfig() *restclient.Config {
	err := cfg.bindEnv()
	if err != nil {
		glog.Error(err)
	}

	kaddr := cfg.KubernetesAddr
	if !kaddr.Provided {
		kaddr = cfg.MasterAddr
	}

	kConfig := cfg.CommonConfig
	kConfig.Host = kaddr.URL.String()

	return &kConfig
}

func (cfg *Config) bindEnv() error {
	// bypass loading from env
	if cfg.SkipEnv {
		return nil
	}
	var err error

	// callers may not use the config file if they have specified a master directly, for backwards
	// compatibility with components that used to use env, switch to service account token, and have
	// config defined in env.
	_, masterSet := util.GetEnv("OPENSHIFT_MASTER")
	specifiedMaster := masterSet || cfg.MasterAddr.Provided

	if cfg.clientConfig != nil && !specifiedMaster {
		clientConfig, err := cfg.clientConfig.ClientConfig()
		if err != nil {
			return err
		}
		cfg.CommonConfig = *clientConfig
		cfg.Namespace, _, err = cfg.clientConfig.Namespace()
		if err != nil {
			return err
		}

		if !cfg.MasterAddr.Provided {
			cfg.MasterAddr.Set(cfg.CommonConfig.Host)
		}
		if !cfg.KubernetesAddr.Provided {
			cfg.KubernetesAddr.Set(cfg.CommonConfig.Host)
		}
		return nil
	}

	// Legacy path - preserve env vars set on pods that previously were honored.
	if value, ok := util.GetEnv("KUBERNETES_MASTER"); ok && !cfg.KubernetesAddr.Provided {
		cfg.KubernetesAddr.Set(value)
	}
	if value, ok := util.GetEnv("OPENSHIFT_MASTER"); ok && !cfg.MasterAddr.Provided {
		cfg.MasterAddr.Set(value)
	}
	if value, ok := util.GetEnv("BEARER_TOKEN"); ok && len(cfg.CommonConfig.BearerToken) == 0 {
		cfg.CommonConfig.BearerToken = value
	}
	if value, ok := util.GetEnv("BEARER_TOKEN_FILE"); ok && len(cfg.CommonConfig.BearerToken) == 0 {
		if tokenData, tokenErr := ioutil.ReadFile(value); tokenErr == nil {
			cfg.CommonConfig.BearerToken = strings.TrimSpace(string(tokenData))
			if len(cfg.CommonConfig.BearerToken) == 0 {
				err = fmt.Errorf("BEARER_TOKEN_FILE %q was empty", value)
			}
		} else {
			err = fmt.Errorf("Error reading BEARER_TOKEN_FILE %q: %v", value, tokenErr)
		}
	}

	if value, ok := util.GetEnv("OPENSHIFT_CA_FILE"); ok && len(cfg.CommonConfig.CAFile) == 0 {
		cfg.CommonConfig.CAFile = value
	} else if value, ok := util.GetEnv("OPENSHIFT_CA_DATA"); ok && len(cfg.CommonConfig.CAData) == 0 {
		cfg.CommonConfig.CAData = []byte(value)
	}

	if value, ok := util.GetEnv("OPENSHIFT_CERT_FILE"); ok && len(cfg.CommonConfig.CertFile) == 0 {
		cfg.CommonConfig.CertFile = value
	} else if value, ok := util.GetEnv("OPENSHIFT_CERT_DATA"); ok && len(cfg.CommonConfig.CertData) == 0 {
		cfg.CommonConfig.CertData = []byte(value)
	}

	if value, ok := util.GetEnv("OPENSHIFT_KEY_FILE"); ok && len(cfg.CommonConfig.KeyFile) == 0 {
		cfg.CommonConfig.KeyFile = value
	} else if value, ok := util.GetEnv("OPENSHIFT_KEY_DATA"); ok && len(cfg.CommonConfig.KeyData) == 0 {
		cfg.CommonConfig.KeyData = []byte(value)
	}

	if value, ok := util.GetEnv("OPENSHIFT_INSECURE"); ok && len(value) != 0 {
		cfg.CommonConfig.Insecure = value == "true"
	}

	return err
}

func DefaultClientConfig(flags *pflag.FlagSet) kclientcmd.ClientConfig {
	loadingRules := config.NewOpenShiftClientConfigLoadingRules()
	flags.StringVar(&loadingRules.ExplicitPath, config.OpenShiftConfigFlagName, "", "Path to the config file to use for CLI requests.")
	cobra.MarkFlagFilename(flags, config.OpenShiftConfigFlagName)

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
