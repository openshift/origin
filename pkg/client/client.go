package client

import (
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
)

// Interface exposes methods on OpenShift resources.
type Interface interface {
}

// Client is an OpenShift client object
type Client struct {
	*kubeclient.RESTClient
}

// New creates and returns a new Client.
func New(host string, auth *kubeclient.AuthInfo) (*Client, error) {
	restClient, err := kubeclient.NewRESTClient(host, auth, "/osapi/v1beta1")
	if err != nil {
		return nil, err
	}
	return &Client{restClient}, nil
}
