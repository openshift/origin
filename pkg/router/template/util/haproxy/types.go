package haproxy

import (
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

// BackendConfig is the haproxy backend config.
type BackendConfig struct {
	Name           string
	Host           string
	Path           string
	IsWildcard     bool
	Termination    routeapi.TLSTerminationType
	InsecurePolicy routeapi.InsecureEdgeTerminationPolicyType
	HasCertificate bool
}

// HAProxyMapEntry is a haproxy map entry.
type HAProxyMapEntry struct {
	Key   string
	Value string
}
