package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
)

// Route encapsulates the inputs needed to connect an alias to endpoints.
type Route struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Spec is the desired behavior of the route
	Spec RouteSpec
	// Status describes the current observed state of the route
	Status RouteStatus
}

// RouteSpec describes the desired behavior of a route.
type RouteSpec struct {
	// Host is an alias/DNS that points to the service. Optional
	// Must follow DNS952 subdomain conventions.
	Host string
	// Path that the router watches for, to route traffic for to the service. Optional
	Path string

	// An object the route points to. Only the Service kind is allowed, and it will
	// be defaulted to Service.
	To kapi.ObjectReference

	// If specified, the port to be used by the router. Most routers will use all
	// endpoints exposed by the service by default - set this value to instruct routers
	// which port to use.
	Port *RoutePort

	//TLS provides the ability to configure certificates and termination for the route
	TLS *TLSConfig
}

// RoutePort defines a port mapping from a router to an endpoint in the service endpoints.
type RoutePort struct {
	// The target port on pods selected by the service this route points to.
	// If this is a string, it will be looked up as a named port in the target
	// endpoints port list. Required
	TargetPort util.IntOrString
}

// RouteStatus provides relevant info about the status of a route, including which routers
// acknowledge it.
type RouteStatus struct {
}

// RouteList is a collection of Routes.
type RouteList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of routes
	Items []Route
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
