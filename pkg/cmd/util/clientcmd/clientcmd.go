package clientcmd

import (
	"fmt"
	"path"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/spf13/pflag"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/util"
)

const ConfigSyntax = " --master=<addr>"

type Config struct {
	MasterAddr     flagtypes.Addr
	KubernetesAddr flagtypes.Addr
	// ClientConfig is the shared base config for both the openshift config and kubernetes config
	CommonConfig kclient.Config
}

func NewConfig() *Config {
	return &Config{
		MasterAddr:     flagtypes.Addr{Value: "localhost:8080", DefaultScheme: "http", DefaultPort: 8080, AllowPrefix: true}.Default(),
		KubernetesAddr: flagtypes.Addr{Value: "localhost:8080", DefaultScheme: "http", DefaultPort: 8080}.Default(),
		CommonConfig:   kclient.Config{},
	}
}

// BindClientConfig adds flags for the supplied client config
func BindClientConfigSecurityFlags(config *kclient.Config, flags *pflag.FlagSet) {
	flags.BoolVar(&config.Insecure, "insecure-skip-tls-verify", config.Insecure, "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure.")
	flags.StringVar(&config.CertFile, "client-certificate", config.CertFile, "Path to a client key file for TLS.")
	flags.StringVar(&config.KeyFile, "client-key", config.KeyFile, "Path to a client key file for TLS.")
	flags.StringVar(&config.CAFile, "certificate-authority", config.CAFile, "Path to a cert. file for the certificate authority")
	flags.StringVar(&config.BearerToken, "token", config.BearerToken, "If present, the bearer token for this request.")
}

func (cfg *Config) Bind(flags *pflag.FlagSet) {
	flags.Var(&cfg.MasterAddr, "master", "The address the master can be reached on (host, host:port, or URL).")
	flags.Var(&cfg.KubernetesAddr, "kubernetes", "The address of the Kubernetes server (host, host:port, or URL). If omitted defaults to the master.")

	BindClientConfigSecurityFlags(&cfg.CommonConfig, flags)
}

func SecretVolumeFromConfig(name string, mountPath string, config *kclient.Config) (*api.Secret, *api.Volume, *api.VolumeMount, []api.EnvVar, error) {
	if err := kclient.LoadTLSFiles(config); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("Error loading credential files: %v", err)
	}

	const (
		root = "root.crt"
		cert = "cert.crt"
		key  = "key.key"
	)
	var (
		root_path = path.Join(mountPath, root)
		cert_path = path.Join(mountPath, cert)
		key_path  = path.Join(mountPath, key)
	)

	secret := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name: name,
		},
		Data: map[string][]byte{
			root: config.CAData,
			cert: config.CertData,
			key:  config.KeyData,
		},
	}

	volume := &api.Volume{
		Name: name,
		VolumeSource: api.VolumeSource{
			Secret: &api.SecretVolumeSource{
				SecretName: name,
			},
		},
	}

	volumeMount := &api.VolumeMount{
		Name:      name,
		MountPath: mountPath,
		ReadOnly:  true,
	}

	envs := []api.EnvVar{
		{Name: "OPENSHIFT_MASTER", Value: config.Host},
		{Name: "OPENSHIFT_CA_BUNDLE", Value: root_path},
		{Name: "OPENSHIFT_CLIENT_CERT", Value: cert_path},
		{Name: "OPENSHIFT_CLIENT_KEY", Value: key_path},
	}
	if config.Insecure {
		envs = append(envs, api.EnvVar{Name: "OPENSHIFT_INSECURE", Value: "true"})
	}

	return secret, volume, volumeMount, envs, nil
}

func (cfg *Config) bindEnv() {
	if value, ok := util.GetEnv("KUBERNETES_MASTER"); ok && !cfg.KubernetesAddr.Provided {
		cfg.KubernetesAddr.Set(value)
	}
	if value, ok := util.GetEnv("OPENSHIFT_MASTER"); ok && !cfg.MasterAddr.Provided {
		cfg.MasterAddr.Set(value)
	}
	if value, ok := util.GetEnv("BEARER_TOKEN"); ok && len(cfg.CommonConfig.BearerToken) == 0 {
		cfg.CommonConfig.BearerToken = value
	}

	if value, ok := util.GetEnv("OPENSHIFT_CA_BUNDLE"); ok && len(cfg.CommonConfig.CAFile) == 0 {
		cfg.CommonConfig.CAFile = value
	}

	if value, ok := util.GetEnv("OPENSHIFT_CLIENT_CERT"); ok && len(cfg.CommonConfig.CertFile) == 0 {
		cfg.CommonConfig.CertFile = value
	}
	if value, ok := util.GetEnv("OPENSHIFT_CLIENT_KEY"); ok && len(cfg.CommonConfig.KeyFile) == 0 {
		cfg.CommonConfig.KeyFile = value
	}

	if value, ok := util.GetEnv("OPENSHIFT_INSECURE"); ok && len(value) != 0 {
		cfg.CommonConfig.Insecure = value == "true"
	}
}

func (cfg *Config) KubeConfig() *kclient.Config {
	cfg.bindEnv()

	kaddr := cfg.KubernetesAddr
	if !kaddr.Provided {
		kaddr = cfg.MasterAddr
	}

	kConfig := cfg.CommonConfig
	kConfig.Host = kaddr.URL.String()

	return &kConfig
}

func (cfg *Config) OpenShiftConfig() *kclient.Config {
	cfg.bindEnv()

	osConfig := cfg.CommonConfig
	osConfig.Host = cfg.MasterAddr.String()

	return &osConfig
}

func (cfg *Config) Clients() (osclient.Interface, kclient.Interface, error) {
	cfg.bindEnv()

	kubeClient, err := kclient.New(cfg.KubeConfig())
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to configure Kubernetes client: %v", err)
	}

	osClient, err := osclient.New(cfg.OpenShiftConfig())
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to configure OpenShift client: %v", err)
	}

	return osClient, kubeClient, nil
}
