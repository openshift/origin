package v1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
)

// Route encapsulates the inputs needed to connect an alias to endpoints.
type Route struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteSpec   `json:"spec" description:"desired state of the route"`
	Status RouteStatus `json:"status" description:"current state of the route"`
}

// RouteList is a collection of Routes.
type RouteList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []Route `json:"items" description:"list of routes"`
}

// RouteSpec describes the route the user wishes to exist.
type RouteSpec struct {
	// Ports are the ports that the user wishes to expose.
	//Ports []RoutePort `json:"ports,omitempty"`

	// Optional: Alias/DNS that points to the service
	// Can be host or host:port
	// host and port are combined to follow the net/url URL struct
	Host string `json:"host" description:"optional: alias/dns that points to the service, can be host or host:port"`
	// Optional: Path that the router watches for, to route traffic for to the service
	Path string `json:"path,omitempty" description:"optional: path that the router watches to route traffic to the service"`

	// An object the route points to. Only the Service kind is allowed, and it will
	// be defaulted to Service.
	To kapi.ObjectReference `json:"to" description:"an object the route points to.  only the service kind is allowed, and it will be defaulted to a service."`

	// TLS provides the ability to configure certificates and termination for the route
	TLS *TLSConfig `json:"tls,omitempty" description:"provides the ability to configure certificates and termination for the route"`
}

/*
type RoutePort struct {
	// Name is the name of the port that is used by the router. Routers may require
	// this field be set. Routers may decide which names to expose.
	Name string `json:"name"`

	// Optional: the name of the target endpoint port.
	TargetName string `json:"targetName"`

	// Optional: the value of the target endpoint port to expose. May be omitted if
	// name is set, and vice versa.
	TargetPort util.IntOrString `json:"targetPort"`
}
*/

// RouteStatus describes the current state of this route.
type RouteStatus struct{}

// RouterShard has information of a routing shard and is used to
// generate host names and routing table entries when a routing shard is
// allocated for a specific route.
// Caveat: This is WIP and will likely undergo modifications when sharding
//         support is added.
type RouterShard struct {
	// Shard name uniquely identifies a router shard in the "set" of
	// routers used for routing traffic to the services.
	ShardName string `json:"shardName" description:"uniquely identifies a router shard in the set of routers used for routing traffic to the services"`

	// The DNS suffix for the shard ala: shard-1.v3.openshift.com
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
