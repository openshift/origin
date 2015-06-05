package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// Route encapsulates the inputs needed to connect an alias to endpoints.
type Route struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Host is an alias/DNS that points to the service. Required
	// Can be host or host:port
	// host and port are combined to follow the net/url URL struct
	Host string `json:"host"`
	// Path that the router watches for, to route traffic for to the service. Optional
	Path string `json:"path,omitempty"`

	// ServiceName is the name of the service that this route points to
	ServiceName string `json:"serviceName"`

	//TLS provides the ability to configure certificates and termination for the route
	TLS *TLSConfig `json:"tls,omitempty"`
}

// RouteList is a collection of Routes.
type RouteList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of routes
	Items []Route `json:"items"`
}

// RouterShard has information of a routing shard and is used to
// generate host names and routing table entries when a routing shard is
// allocated for a specific route.
type RouterShard struct {
	// ShardName uniquely identifies a router shard in the "set" of
	// routers used for routing traffic to the services.
	ShardName string

	// DNSSuffix for the shard ala: shard-1.v3.openshift.com
	DNSSuffix string
}

// TLSConfig defines config used to secure a route and provide termination
type TLSConfig struct {
	// Termination indicates termination type.  If termination type is not set, any termination config will be ignored
	Termination TLSTerminationType `json:"termination,omitempty"`

	// Certificate provides certificate contents
	Certificate string `json:"certificate,omitempty"`

	// Key provides key file contents
	Key string `json:"key,omitempty"`

	// CACertificate provides the cert authority certificate contents
	CACertificate string `json:"caCertificate,omitempty"`

	// DestinationCACertificate provides the contents of the ca certificate of the final destination.  When using reencrypt
	// termination this file should be provided in order to have routers use it for health checks on the secure connection
	DestinationCACertificate string `json:"destinationCACertificate,omitempty"`
}

// TLSTerminationType dictates where the secure communication will stop
type TLSTerminationType string

const (
	// TLSTerminationEdge terminate encryption at the edge router.
	TLSTerminationEdge TLSTerminationType = "edge"
	// TLSTerminationPassthrough terminate encryption at the destination, the destination is responsible for decrypting traffic
	TLSTerminationPassthrough TLSTerminationType = "passthrough"
	// TLSTerminationReencrypt terminate encryption at the edge router and re-encrypt it with a new certificate supplied by the destination
	TLSTerminationReencrypt TLSTerminationType = "reencrypt"
)
