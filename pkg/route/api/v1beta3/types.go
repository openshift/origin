package v1beta3

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1beta3"
	"k8s.io/kubernetes/pkg/util"
)

// Route encapsulates the inputs needed to connect an alias to endpoints.
type Route struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	Spec   RouteSpec   `json:"spec"`
	Status RouteStatus `json:"status"`
}

// RouteList is a collection of Routes.
type RouteList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`
	Items                []Route `json:"items"`
}

// RouteSpec describes the route the user wishes to exist.
type RouteSpec struct {
	// Ports are the ports that the user wishes to expose.
	//Ports []RoutePort `json:"ports,omitempty"`

	// Host is an alias/DNS that points to the service. Optional
	// Must follow DNS952 subdomain conventions.
	Host string `json:"host"`
	// Optional: Path that the router watches for, to route traffic for to the service
	Path string `json:"path,omitempty"`

	// An object the route points to. Only the Service kind is allowed, and it will
	// be defaulted to Service.
	To kapi.ObjectReference `json:"to"`

	// If specified, the port to be used by the router. Most routers will use all
	// endpoints exposed by the service by default - set this value to instruct routers
	// which port to use.
	Port *RoutePort `json:"port,omitempty"`

	// TLS provides the ability to configure certificates and termination for the route
	TLS *TLSConfig `json:"tls,omitempty"`
}

// RoutePort defines a port mapping from a router to an endpoint in the service endpoints.
type RoutePort struct {
	// The target port on pods selected by the service this route points to.
	// If this is a string, it will be looked up as a named port in the target
	// endpoints port list. Required
	TargetPort util.IntOrString `json:"targetPort"`
}

// RouteStatus provides relevant info about the status of a route, including which routers
// acknowledge it.
type RouteStatus struct {
}

// RouterShard has information of a routing shard and is used to
// generate host names and routing table entries when a routing shard is
// allocated for a specific route.
// Caveat: This is WIP and will likely undergo modifications when sharding
//         support is added.
type RouterShard struct {
	// Shard name uniquely identifies a router shard in the "set" of
	// routers used for routing traffic to the services.
	ShardName string `json:"shardName"`

	// The DNS suffix for the shard ala: shard-1.v3.openshift.com
	DNSSuffix string `json:"dnsSuffix"`
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

	// InsecureEdgeTerminationPolicy indicates the desired behavior for
	// insecure connections to an edge-terminated route:
	//   disable, allow or redirect
	InsecureEdgeTerminationPolicy InsecureEdgeTerminationPolicyType `json:"insecureEdgeTerminationPolicy,omitempty"`
}

// TLSTerminationType dictates where the secure communication will stop
type TLSTerminationType string

// InsecureEdgeTerminationPolicyType dictates the behavior of insecure
// connections to an edge-terminated route.
type InsecureEdgeTerminationPolicyType string

const (
	// TLSTerminationEdge terminate encryption at the edge router.
	TLSTerminationEdge TLSTerminationType = "edge"
	// TLSTerminationPassthrough terminate encryption at the destination, the destination is responsible for decrypting traffic
	TLSTerminationPassthrough TLSTerminationType = "passthrough"
	// TLSTerminationReencrypt terminate encryption at the edge router and re-encrypt it with a new certificate supplied by the destination
	TLSTerminationReencrypt TLSTerminationType = "reencrypt"
)
