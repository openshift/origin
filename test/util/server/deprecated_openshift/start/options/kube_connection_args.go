package options

import (
	"errors"
	"net/url"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/oc/pkg/helpers/flagtypes"
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

func (args KubeConnectionArgs) GetExternalKubernetesClientConfig() (*restclient.Config, bool, error) {
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
	return defaultAddress, nil
}
