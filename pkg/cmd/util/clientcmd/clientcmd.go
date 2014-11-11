package clientcmd

import (
	"fmt"

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
	BearerToken    string
}

func NewConfig() *Config {
	return &Config{
		MasterAddr:     flagtypes.Addr{Value: "localhost:8080", DefaultScheme: "http", DefaultPort: 8080, AllowPrefix: true}.Default(),
		KubernetesAddr: flagtypes.Addr{Value: "localhost:8080", DefaultScheme: "http", DefaultPort: 8080}.Default(),
		BearerToken:    "",
	}
}

func (cfg *Config) Bind(flag *pflag.FlagSet) {
	flag.Var(&cfg.MasterAddr, "master", "The address the master can be reached on (host, host:port, or URL).")
	flag.Var(&cfg.KubernetesAddr, "kubernetes", "The address of the Kubernetes server (host, host:port, or URL). If omitted defaults to the master.")
	flag.StringVar(&cfg.BearerToken, "token", "", "If present, the bearer token for this request.")
}

func (cfg *Config) bindEnv() {
	if value, ok := util.GetEnv("KUBERNETES_MASTER"); ok && !cfg.KubernetesAddr.Provided {
		cfg.KubernetesAddr.Set(value)
	}
	if value, ok := util.GetEnv("OPENSHIFT_MASTER"); ok && !cfg.MasterAddr.Provided {
		cfg.MasterAddr.Set(value)
	}
	if value, ok := util.GetEnv("BEARER_TOKEN"); ok && len(cfg.BearerToken) == 0 {
		cfg.BearerToken = value
	}
}

func (cfg *Config) Clients() (*kclient.Client, *osclient.Client, error) {
	cfg.bindEnv()

	kaddr := cfg.KubernetesAddr
	if !kaddr.Provided {
		kaddr = cfg.MasterAddr
	}

	config := &kclient.Config{Host: cfg.MasterAddr.String(), BearerToken: cfg.BearerToken}
	kubeClient, err := kclient.New(&kclient.Config{Host: kaddr.URL.String(), BearerToken: cfg.BearerToken})
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to configure Kubernetes client: %v", err)
	}

	osClient, err := osclient.New(config)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to configure OpenShift client: %v", err)
	}

	return kubeClient, osClient, nil
}
