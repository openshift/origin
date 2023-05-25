package backend

import (
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type ProtocolType string

const (
	ProtocolHTTP1 ProtocolType = "http1"
	ProtocolHTTP2 ProtocolType = "http2"
)

type LoadBalancerType string

const (
	ExternalLoadBalancerType LoadBalancerType = "external-lb"
	InternalLoadBalancerType LoadBalancerType = "internal-lb"
	ServiceNetworkType       LoadBalancerType = "service-network"
)

// TestDescriptor describes a backend disruption test
type TestDescriptor interface {
	Name() string
	DisruptionLocator() string
	ShutdownLocator() string
	GetLoadBalancerType() LoadBalancerType
	GetProtocol() ProtocolType
	GetConnectionType() monitorapi.BackendConnectionType
	GetTargetServerName() string
}
