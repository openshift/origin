package client

import (
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeConfigOrInClusterConfig loads in-cluster config if kubeConfigFile is empty or the file if not,
// then applies overrides.
func GetKubeConfigOrInClusterConfig(kubeConfigFile string, overrides *ClientConnectionOverrides) (*rest.Config, error) {
	if len(kubeConfigFile) > 0 {
		return GetClientConfig(kubeConfigFile, overrides)
	}

	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	applyClientConnectionOverrides(overrides, clientConfig)
	clientConfig.WrapTransport = defaultClientTransport

	return clientConfig, nil
}

// GetClientConfig returns the rest.Config for a kubeconfig file
func GetClientConfig(kubeConfigFile string, overrides *ClientConnectionOverrides) (*rest.Config, error) {
	kubeConfigBytes, err := ioutil.ReadFile(kubeConfigFile)
	if err != nil {
		return nil, err
	}
	// TODO after 1.11 rebase, restore this
	//kubeConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfigBytes)
	//if err != nil {
	//	return nil, err
	//}
	config, err := clientcmd.Load(kubeConfigBytes)
	if err != nil {
		return nil, err
	}
	kubeConfig := clientcmd.NewNonInteractiveClientConfig(*config, "", &clientcmd.ConfigOverrides{}, nil)

	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	applyClientConnectionOverrides(overrides, clientConfig)
	clientConfig.WrapTransport = defaultClientTransport

	return clientConfig, nil
}

// applyClientConnectionOverrides updates a kubeConfig with the overrides from the config.
func applyClientConnectionOverrides(overrides *ClientConnectionOverrides, kubeConfig *rest.Config) {
	if overrides == nil {
		return
	}
	if overrides.QPS > 0 {
		kubeConfig.QPS = overrides.QPS
	}
	if overrides.Burst > 0 {
		kubeConfig.Burst = int(overrides.Burst)
	}
	if len(overrides.AcceptContentTypes) > 0 {
		kubeConfig.ContentConfig.AcceptContentTypes = overrides.AcceptContentTypes
	}
	if len(overrides.ContentType) > 0 {
		kubeConfig.ContentConfig.ContentType = overrides.ContentType
	}
}

// defaultClientTransport sets defaults for a client Transport that are suitable for use by infrastructure components.
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

// ClientConnectionOverrides allows overriding values for rest.Config not held in a kubeconfig.  Most commonly used
// for QPS.  Empty values are not used.
type ClientConnectionOverrides struct {
	// AcceptContentTypes defines the Accept header sent by clients when connecting to a server, overriding the
	// default value of 'application/json'. This field will control all connections to the server used by a particular
	// client.
	AcceptContentTypes string
	// ContentType is the content type used when sending data to the server from this client.
	ContentType string

	// QPS controls the number of queries per second allowed for this connection.
	QPS float32
	// Burst allows extra queries to accumulate when a client is exceeding its rate.
	Burst int32
}
