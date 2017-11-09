package origin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	restclient "k8s.io/client-go/rest"
)

// A ServiceResolver knows how to get a URL given a service.
type ServiceResolver interface {
	ResolveEndpoint(namespace, name string) (*url.URL, error)
}

// proxyHandler provides a http.Handler which will proxy traffic to locations
// specified by items implementing Redirector.
type serviceProxyHandler struct {
	serviceName      string
	serviceNamespace string

	// Endpoints based routing to map from cluster IP to routable IP
	serviceResolver ServiceResolver

	// proxyRoundTripper is the re-useable portion of the transport.  It does not vary with any request.
	proxyRoundTripper http.RoundTripper

	restConfig *restclient.Config
}

// NewServiceProxyHandler is a simple proxy that doesn't handle upgrades, passes headers directly through, and doesn't assert any identity.
func NewServiceProxyHandler(serviceName string, serviceNamespace string, serviceResolver ServiceResolver, caBundle []byte) (*serviceProxyHandler, error) {
	restConfig := &restclient.Config{
		TLSClientConfig: restclient.TLSClientConfig{
			ServerName: serviceName + "." + serviceNamespace + ".svc",
			CAData:     caBundle,
		},
	}
	proxyRoundTripper, err := restclient.TransportFor(restConfig)
	if err != nil {
		return nil, err
	}

	return &serviceProxyHandler{
		serviceName:       serviceName,
		serviceNamespace:  serviceNamespace,
		serviceResolver:   serviceResolver,
		proxyRoundTripper: proxyRoundTripper,
		restConfig:        restConfig,
	}, nil
}

func (r *serviceProxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// write a new location based on the existing request pointed at the target service
	location := &url.URL{}
	location.Scheme = "https"
	rloc, err := r.serviceResolver.ResolveEndpoint(r.serviceNamespace, r.serviceName)
	if errors.IsNotFound(err) {
		http.Error(w, fmt.Sprintf("missing service (%s)", err.Error()), http.StatusNotFound)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("missing route (%s)", err.Error()), http.StatusInternalServerError)
		return
	}
	location.Host = rloc.Host
	location.Path = req.URL.Path
	location.RawQuery = req.URL.Query().Encode()

	// WithContext creates a shallow clone of the request with the new context.
	newReq := req.WithContext(context.Background())
	newReq.Header = utilnet.CloneHeader(req.Header)
	newReq.URL = location

	handler := proxy.NewUpgradeAwareHandler(location, r.proxyRoundTripper, false, false, &responder{w: w})
	handler.ServeHTTP(w, newReq)
}

// responder implements rest.Responder for assisting a connector in writing objects or errors.
type responder struct {
	w http.ResponseWriter
}

// TODO this should properly handle content type negotiation
// if the caller asked for protobuf and you write JSON bad things happen.
func (r *responder) Object(statusCode int, obj runtime.Object) {
	responsewriters.WriteRawJSON(statusCode, obj, r.w)
}

func (r *responder) Error(_ http.ResponseWriter, _ *http.Request, err error) {
	http.Error(r.w, err.Error(), http.StatusInternalServerError)
}
