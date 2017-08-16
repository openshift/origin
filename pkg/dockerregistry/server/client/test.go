package client

import (
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	"github.com/openshift/origin/pkg/client"
	imageclientv1 "github.com/openshift/origin/pkg/image/generated/clientset/typed/image/v1"
)

type fakeRegistryClient struct {
	RegistryClient

	client client.Interface
	images imageclientv1.ImageV1Interface
}

func NewFakeRegistryClient(c client.Interface, imageclient imageclientv1.ImageV1Interface) RegistryClient {
	return &fakeRegistryClient{
		RegistryClient: &registryClient{},
		client:         c,
		images:         imageclient,
	}
}

func (c *fakeRegistryClient) Client() (Interface, error) {
	return newAPIClient(c.client, nil, nil, c.images, nil), nil
}

func NewFakeRegistryAPIClient(c client.Interface, kc kcoreclient.CoreInterface, imageclient imageclientv1.ImageV1Interface) Interface {
	return newAPIClient(c, nil, nil, imageclient, nil)
}
