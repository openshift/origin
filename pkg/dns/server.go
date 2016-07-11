package dns

import (
	"github.com/golang/glog"

	"github.com/skynetservices/skydns/metrics"
	"github.com/skynetservices/skydns/server"

	client "k8s.io/kubernetes/pkg/client/unversioned"
)

// NewServerDefaults returns the default SkyDNS server configuration for a DNS server.
func NewServerDefaults() (*server.Config, error) {
	config := &server.Config{
		Domain:  "cluster.local.",
		Local:   "openshift.default.svc.cluster.local.",
		Verbose: bool(glog.V(4)),
	}
	return config, server.SetDefaults(config)
}

type Server struct {
	Config      *server.Config
	Services    ServiceAccessor
	Endpoints   client.EndpointsNamespacer
	MetricsName string

	Stop chan struct{}
}

// NewServer creates a server from the provided config and client.
func NewServer(config *server.Config, client *client.Client) *Server {
	stop := make(chan struct{})
	return &Server{
		Config:    config,
		Services:  NewCachedServiceAccessor(client, stop),
		Endpoints: client,
		Stop:      stop,
	}
}

// ListenAndServe starts a DNS server that exposes services and values stored in etcd (if etcdclient
// is not nil). It will block until the server exits.
func (s *Server) ListenAndServe() error {
	resolver := NewServiceResolver(s.Config, s.Services, s.Endpoints, openshiftFallback)
	resolvers := server.FirstBackend{resolver}
	if len(s.MetricsName) > 0 {
		metrics.RegisterPrometheusMetrics(s.MetricsName, "")
	}
	dns := server.New(resolvers, s.Config)
	if s.Stop != nil {
		defer close(s.Stop)
	}
	return dns.Run()
}

func openshiftFallback(name string, exact bool) (string, bool) {
	if name == "openshift.default.svc" {
		return "kubernetes.default.svc.", true
	}
	if name == "_endpoints.openshift.default.svc" {
		return "_endpoints.kubernetes.default.", true
	}
	return "", false
}
