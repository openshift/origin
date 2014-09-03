package client

import (
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
)

// Interface holds the methods for Clients of OpenShift.
type Interface interface {
	kubeclient.Interface
	OpenShiftInterface
}

// OpenShiftInterface holds the methods supported only by OpenShift.
type OpenShiftInterface interface{}

// Client encompasses all methods of the Kubernetes and OpenShift clients.
type Client struct {
	*kubeclient.Client
	*osClient
}

// New creates and returns a new Client.
func New(host string, auth *kubeclient.AuthInfo) *Client {
	osClient := newOSClient(host, auth)
	kClient := kubeclient.New(host, auth)

	return &Client{
		kClient,
		osClient,
	}
}

type osClient struct {
	*kubeclient.RESTClient
}

func newOSClient(host string, auth *kubeclient.AuthInfo) *osClient {
	return &osClient{kubeclient.NewRESTClient(host, auth, "/osapi/v1beta1")}
}
