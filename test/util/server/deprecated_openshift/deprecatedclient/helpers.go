package deprecatedclient

import (
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
)

func GetClientConfig(kubeConfigFile string, overrides *configapi.ClientConnectionOverrides) (*rest.Config, error) {
	kubeConfigBytes, err := ioutil.ReadFile(kubeConfigFile)
	if err != nil {
		return nil, err
	}
	kubeConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfigBytes)
	if err != nil {
		return nil, err
	}
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	applyClientConnectionOverrides(overrides, clientConfig)
	clientConfig.WrapTransport = defaultClientTransport

	return clientConfig, nil
}

// applyClientConnectionOverrides updates a kubeConfig with the overrides from the config.
func applyClientConnectionOverrides(overrides *configapi.ClientConnectionOverrides, kubeConfig *rest.Config) {
	if overrides == nil {
		return
	}
	kubeConfig.QPS = overrides.QPS
	kubeConfig.Burst = int(overrides.Burst)
	kubeConfig.ContentConfig.AcceptContentTypes = overrides.AcceptContentTypes
	kubeConfig.ContentConfig.ContentType = overrides.ContentType
}

// defaultClientTransport sets defaults for a client Transport that are suitable
// for use by infrastructure components.
func defaultClientTransport(rt http.RoundTripper) http.RoundTripper {
	transport, ok := rt.(*http.Transport)
	if !ok {
		return rt
	}

	// TODO: this should be configured by the caller, not in this method.
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport.Dial = dialer.Dial
	// Hold open more internal idle connections
	// TODO: this should be configured by the caller, not in this method.
	transport.MaxIdleConnsPerHost = 100
	return transport
}
