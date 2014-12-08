package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

// RESTClient provides REST interface compatible with Kubernetes RESTClient
type RESTClient interface {
	Verb(verb string) *client.Request
	Put() *client.Request
	Post() *client.Request
	Delete() *client.Request
	Get() *client.Request
}

// ClientMappings stores mapping between kind and client and codec
// TODO: This struct is now obsoleted by kubectl
type ClientMappings map[string]struct {
	Kind   string
	Client RESTClient
	Codec  runtime.Codec
}
