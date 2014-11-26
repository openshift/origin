package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

type RESTClient interface {
	Verb(verb string) *client.Request
	Put() *client.Request
	Post() *client.Request
	Delete() *client.Request
	Get() *client.Request
}

type ClientMappings map[string]struct {
	Kind   string
	Client RESTClient
	Codec  runtime.Codec
}
