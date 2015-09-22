package dns

import (
	"k8s.io/kubernetes/pkg/client"

	"github.com/coreos/go-etcd/etcd"
	"github.com/prometheus/client_golang/prometheus"
	backendetcd "github.com/skynetservices/skydns/backends/etcd"
	"github.com/skynetservices/skydns/server"
)

// NewServerDefaults returns the default SkyDNS server configuration for a DNS server.
func NewServerDefaults() (*server.Config, error) {
	config := &server.Config{
		Domain: "cluster.local.",
		Local:  "openshift.default.svc.cluster.local.",
	}
	return config, server.SetDefaults(config)
}

// ListenAndServe starts a DNS server that exposes services and values stored in etcd (if etcdclient
// is not nil). It will block until the server exits.
// TODO: hoist the service accessor out of this package so it can be reused.
func ListenAndServe(config *server.Config, client *client.Client, etcdclient *etcd.Client) error {
	stop := make(chan struct{})
	accessor := NewCachedServiceAccessor(client, stop)
	resolver := NewServiceResolver(config, accessor, client, openshiftFallback)
	resolvers := server.FirstBackend{resolver}
	if etcdclient != nil {
		resolvers = append(resolvers, backendetcd.NewBackend(etcdclient, &backendetcd.Config{
			Ttl:      config.Ttl,
			Priority: config.Priority,
		}))
	}

	server.RegisterMetrics("", "")
	s := server.New(resolvers, config)
	defer close(stop)
	return s.Run()
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

// counter is a SkyDNS compatible Counter
type counter struct {
	prometheus.Counter
}

// newCounter registers a prometheus counter and wraps it to match SkyDNS
func newCounter(c prometheus.Counter) server.Counter {
	prometheus.MustRegister(c)
	return counter{c}
}

// Inc increases the counter with the given value
func (c counter) Inc(val int64) {
	c.Counter.Add(float64(val))
}
