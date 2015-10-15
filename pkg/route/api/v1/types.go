package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/util"
)

// Route encapsulates the inputs needed to connect an alias to endpoints.
type Route struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// Spec is the desired state of the route
	Spec RouteSpec `json:"spec" description:"desired state of the route"`
	// Status is the current state of the route
	Status RouteStatus `json:"status" description:"current state of the route"`
}

// RouteList is a collection of Routes.
type RouteList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of routes
	Items []Route `json:"items" description:"list of routes"`
}

// RouteSpec describes the route the user wishes to exist.
type RouteSpec struct {
	// Ports are the ports that the user wishes to expose.
	//Ports []RoutePort `json:"ports,omitempty"`

	// Host is an alias/DNS that points to the service. Optional
	// Must follow DNS952 subdomain conventions.
	Host string `json:"host" description:"optional: alias/dns that points to the service, must follow DNS 952 subdomain conventions"`
	// Path that the router watches for, to route traffic for to the service. Optional
	Path string `json:"path,omitempty" description:"optional: path that the router watches to route traffic to the service"`

	// To is an object the route points to. Only the Service kind is allowed, and it will
	// be defaulted to Service.
	To kapi.ObjectReference `json:"to" description:"an object the route points to.  only the service kind is allowed, and it will be defaulted to a service."`

	// If specified, the port to be used by the router. Most routers will use all
	// endpoints exposed by the service by default - set this value to instruct routers
	// which port to use.
	Port *RoutePort `json:"port,omitempty" description:"port that should be used by the router; this is a hint to control which pod endpoint port is used; if empty routers may use all endpoints and ports"`

	// TLS provides the ability to configure certificates and termination for the route
	TLS *TLSConfig `json:"tls,omitempty" description:"provides the ability to configure certificates and termination for the route"`
}

// RoutePort defines a port mapping from a router to an endpoint in the service endpoints.
type RoutePort struct {
	// The target port on pods selected by the service this route points to.
	// If this is a string, it will be looked up as a named port in the target
	// endpoints port list. Required
	TargetPort util.IntOrString `json:"targetPort" description:"the target port on the endpoints for the service; if this is a string must match the named port, if an integer, must match the port number"`
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
	// ShardName uniquely identifies a router shard in the "set" of
	// routers used for routing traffic to the services.
	ShardName string `json:"shardName" description:"uniquely identifies a router shard in the set of routers used for routing traffic to the services"`

	// DNSSuffix for the shard ala: shard-1.v3.openshift.com
	DNSSuffix string `json:"dnsSuffix" description:"DNS suffix for the shard (i.e. shard-1.v3.openshift.com)"`
}

// TLSConfig defines config used to secure a route and provide termination
type TLSConfig struct {
	// Termination indicates termination type.  If termination type is not set, any termination config will be ignored
	Termination TLSTerminationType `json:"termination,omitempty" description:"indicates termination type.  if not set, any termination config will be ignored"`

	// Certificate provides certificate contents
	Certificate string `json:"certificate,omitempty" description:"provides certificate contents"`

	// Key provides key file contents
	Key string `json:"key,omitempty" description:"provides key file contents"`

	// CACertificate provides the cert authority certificate contents
	CACertificate string `json:"caCertificate,omitempty" description:"provides the cert authority certificate contents"`

	// DestinationCACertificate provides the contents of the ca certificate of the final destination.  When using reencrypt
	// termination this file should be provided in order to have routers use it for health checks on the secure connection
	DestinationCACertificate string `json:"destinationCACertificate,omitempty" description:"provides the contents of the ca certificate of the final destination.  When using re-encrypt termination this file should be provided in order to have routers use it for health checks on the secure connection"`
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
