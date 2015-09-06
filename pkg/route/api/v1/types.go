package v1

import (
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/util"
)

// Route encapsulates the inputs needed to connect an alias to endpoints.
type Route struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the route
	Spec RouteSpec `json:"spec" description:"desired state of the route"`
	// Status is the current state of the route
	Status RouteStatus `json:"status" description:"current state of the route"`
}

// RouteList is a collection of Routes.
type RouteList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

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

	// TLS provides the ability to configure certificates and termination for the route
	TLS *TLSConfig `json:"tls,omitempty" description:"provides the ability to configure certificates and termination for the route"`
}

/*
type RoutePort struct {
	// Name is the name of the port that is used by the router. Routers may require
	// this field be set. Routers may decide which names to expose.
	Name string `json:"name"`

	// TargetName is the name of the target endpoint port. Optional
	TargetName string `json:"targetName"`

	// TargetPort is the value of the target endpoint port to expose. May be omitted if
	// name is set, and vice versa. Optional
	TargetPort util.IntOrString `json:"targetPort"`
}
*/

// RouteStatus provides relevant info about the status of a route, including which routers
// acknowledge it.
type RouteStatus struct {
	// Ingress describes the places where the route may be exposed. The list of
	// ingress points may contain duplicate Host or RouterName values. Routes
	// are considered live once they are `Ready`
	Ingress []RouteIngress `json:"ingress,omitempty" description:"traffic reaches this route via these ingress paths"`
}

// RouteIngress holds information about the places where a route is exposed
type RouteIngress struct {
	// Host is the host string under which the route is exposed; this value is required
	Host string `json:"host,omitempty" description:"the host name this route is exposed to by the specified router"`
	// Name is a name chosen by the router to identify itself; this value is required
	RouterName string `json:"routerName,omitempty" description:"the name of the router exposing this route"`
	// Conditions is the state of the route, may be empty.
	Conditions []RouteIngressCondition `json:"conditions,omitempty" description:"the conditions that apply to this router" patchStrategy:"merge" patchMergeKey:"type"`
}

// RouteIngressConditionType is a valid value for RouteCondition
type RouteIngressConditionType string

// These are valid conditions of pod.
const (
	// RouteReady means the route is able to service requests for the provided Host
	RouteReady RouteIngressConditionType = "Ready"
	// TODO: add other route condition types
)

// RouteIngressCondition contains details for the current condition of this pod.
// TODO: add LastTransitionTime, Reason, Message to match NodeCondition api.
type RouteIngressCondition struct {
	// Type is the type of the condition.
	// Currently only Ready.
	Type RouteIngressConditionType `json:"type" description:"the type of the condition"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status kapi.ConditionStatus `json:"status" description:"status is the status of the condition; True, False, or Unknown"`
	// (brief) reason for the condition's last transition, and is usually a machine and human
	// readable constant
	Reason string `json:"reason,omitempty" description:"brief reason for the condition's last transition, machine readable constant"`
	// Human readable message indicating details about last transition.
	Message string `json:"message,omitempty" description:"human readable message indicating details about this condition"`
	// RFC 3339 date and time when this condition last transitioned
	LastTransitionTime *util.Time `json:"lastTransitionTime,omitempty" description:"the last time at which this condition transitioned to the current status"`
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
