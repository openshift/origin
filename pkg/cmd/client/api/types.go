package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

type RESTClient interface {
	Verb(verb string) *client.Request
}

type ClientMappings map[string]struct {
	Kind   string
	Client RESTClient
	Codec  runtime.Codec
}
