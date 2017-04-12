package testutil

import (
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry/server/oapi"
)

type fakeRegistryClient struct {
	oapi.RegistryClient

	client client.Interface
}

func NewRegistryClient(c client.Interface) oapi.RegistryClient {
	return &fakeRegistryClient{
		RegistryClient: oapi.NewRegistryClient(nil),
		client:         c,
	}
}

func (c *fakeRegistryClient) Client() (oapi.ClientInterface, error) {
	return oapi.NewAPIClient(c.client, nil), nil
}
