package userspace

import (
	"net"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/proxy"
)

// LoadBalancer is an interface for distributing incoming requests to service endpoints.
type LoadBalancer interface {
	// NextEndpoint returns the endpoint to handle a request for the given
	// service-port and source address.
	NextEndpoint(service proxy.ServicePortName, srcAddr net.Addr, sessionAffinityReset bool) (string, error)
	NewService(service proxy.ServicePortName, sessionAffinityType api.ServiceAffinity, stickyMaxAgeMinutes int) error
	CleanupStaleStickySessions(service proxy.ServicePortName)
	ServiceHasEndpoints(service proxy.ServicePortName) bool
}
