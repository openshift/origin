package transport

import (
	"fmt"
	"net"
	"net/http"
	"time"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

// FromRestConfig constructs a http.RoundTripper transport from the
// given rest Config object.
//
//	config: the given rest.Config object
//	reuseConnection: true if the underlying TCP connection should be
//	  reused for multiple requests
//	timeout: transport timeout
//	http1: if true the transport will be configured to use HTTP/1.x
//	  protocol, otherwise http/2.0 will be used.
func FromRestConfig(config *rest.Config, reuseConnection bool, timeout time.Duration, http1 bool) (http.RoundTripper, error) {
	kubeTransportConfig, err := config.TransportConfig()
	if err != nil {
		return nil, err
	}
	tlsConfig, err := transport.TLSConfigFor(kubeTransportConfig)
	if err != nil {
		return nil, err
	}

	rt := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: -1, // this looks unnecessary to me, but it was set in other code.
		}).Dial,
		TLSClientConfig:       tlsConfig,
		DisableKeepAlives:     !reuseConnection, // this prevents connections from being reused
		TLSHandshakeTimeout:   timeout,
		IdleConnTimeout:       timeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: timeout,
		Proxy:                 http.ProxyFromEnvironment,
	}
	if !http1 {
		utilnet.SetTransportDefaults(rt)
	}
	if len(config.BearerToken) == 0 && len(config.BearerTokenFile) == 0 {
		return rt, nil
	}
	if tlsConfig == nil {
		return nil, fmt.Errorf("tls.Config is required if you have providing a token")
	}

	return transport.NewBearerAuthWithRefreshRoundTripper(config.BearerToken, config.BearerTokenFile, rt)
}
