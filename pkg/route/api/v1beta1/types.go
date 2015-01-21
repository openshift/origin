package v1beta1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
)

// Route encapsulates the inputs needed to connect a DNS/alias to a service proxy.
type Route struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Required: Alias/DNS that points to the service
	// Can be host or host:port
	// host and port are combined to follow the net/url URL struct
	Host string `json:"host"`
	// Optional: Path that the router watches for, to route traffic for to the service
	Path string `json:"path,omitempty"`

	// the name of the service that this route points to
	ServiceName string `json:"serviceName"`
}

// RouteList is a collection of Routes.
type RouteList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []Route `json:"items"`
}
