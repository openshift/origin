package start

import (
	"errors"
	"net/url"

	"github.com/spf13/pflag"

	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
)

type KubeConnectionArgs struct {
	KubernetesAddr flagtypes.Addr

	// ClientConfig is used when connecting to Kubernetes from the master, or
	// when connecting to the master from a detached node. If StartKube is true,
	// this value is not used.
	ClientConfig clientcmd.ClientConfig
	// ClientConfigLoadingRules is the ruleset used to load the client config.
	// Only the CommandLinePath is expected to be used.
	ClientConfigLoadingRules clientcmd.ClientConfigLoadingRules
}

// BindKubeConnectionArgs binds values to the given arguments by using flags
func BindKubeConnectionArgs(args *KubeConnectionArgs, flags *pflag.FlagSet, prefix string) {
	// TODO remove entirely
	flags.Var(&args.KubernetesAddr, prefix+"kubernetes", "removed in favor of --"+prefix+"kubeconfig")
	flags.StringVar(&args.ClientConfigLoadingRules.ExplicitPath, prefix+"kubeconfig", "", "Path to the kubeconfig file to use for requests to the Kubernetes API.")
}

// NewDefaultKubeConnectionArgs returns a new set of default connection
// arguments for Kubernetes
func NewDefaultKubeConnectionArgs() *KubeConnectionArgs {
	config := &KubeConnectionArgs{}

	config.KubernetesAddr = flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default()
	config.ClientConfig = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&config.ClientConfigLoadingRules, &clientcmd.ConfigOverrides{})

	return config
}

func (args KubeConnectionArgs) Validate() error {
	if args.KubernetesAddr.Provided {
		return errors.New("--kubernetes is no longer allowed, try using --kubeconfig")
	}

	return nil
}

func (args KubeConnectionArgs) GetExternalKubernetesClientConfig() (*client.Config, bool, error) {
	if len(args.ClientConfigLoadingRules.ExplicitPath) == 0 || args.ClientConfig == nil {
		return nil, false, nil
	}
	clientConfig, err := args.ClientConfig.ClientConfig()
	if err != nil {
		return nil, false, err
	}
	return clientConfig, true, nil
}

func (args KubeConnectionArgs) GetKubernetesAddress(defaultAddress *url.URL) (*url.URL, error) {
	config, ok, err := args.GetExternalKubernetesClientConfig()
	if err != nil {
		return nil, err
	}
	if ok && len(config.Host) > 0 {
		return url.Parse(config.Host)
	}

	if defaultAddress == nil {
		return nil, errors.New("no default KubernetesAddress present")
	}
	return defaultAddress, nil
}
