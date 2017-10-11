package dns

import (
	"net"
	"strings"

	"github.com/golang/glog"

	"github.com/skynetservices/skydns/metrics"
	"github.com/skynetservices/skydns/server"
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
	Endpoints   EndpointsAccessor
	MetricsName string

	Stop chan struct{}
}

// NewServer creates a server.
func NewServer(config *server.Config, services ServiceAccessor, endpoints EndpointsAccessor, metricsName string) *Server {
	stop := make(chan struct{})
	return &Server{
		Config:      config,
		Services:    services,
		Endpoints:   endpoints,
		MetricsName: metricsName,
		Stop:        stop,
	}
}

// ListenAndServe starts a DNS server that exposes services and values stored in etcd (if etcdclient
// is not nil). It will block until the server exits.
func (s *Server) ListenAndServe() error {
	monitorDnsmasq(s.Config, s.MetricsName)

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

// monitorDnsmasq attempts to start the dnsmasq monitoring goroutines to keep dnsmasq
// in sync with this server. It will take no action if the current config DnsAddr does
// not point to port 53 (dnsmasq does not support alternate upstream ports). It will
// convert the bind address from 0.0.0.0 to the BindNetwork appropriate listen address.
func monitorDnsmasq(config *server.Config, metricsName string) {
	if host, port, err := net.SplitHostPort(config.DnsAddr); err == nil && port == "53" {
		if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
			if config.BindNetwork == "ipv6" {
				host = "::1"
			} else {
				host = "127.0.0.1"
			}
		}
		monitor := &dnsmasqMonitor{
			metricsName: metricsName,
			dnsIP:       host,
			dnsDomain:   strings.TrimSuffix(config.Domain, "."),
		}
		if err := monitor.Start(); err != nil {
			glog.Warningf("Unable to start dnsmasq monitor: %v", err)
		} else {
			glog.V(2).Infof("Monitoring dnsmasq to point cluster queries to %s", host)
		}
	} else {
		glog.Warningf("Unable to keep dnsmasq up to date, %s must point to port 53", config.DnsAddr)
	}
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
