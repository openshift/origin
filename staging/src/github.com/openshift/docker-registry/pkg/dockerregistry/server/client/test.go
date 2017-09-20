package client

import (
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	imageclientv1 "github.com/openshift/origin/pkg/image/generated/clientset/typed/image/v1"
)

type fakeRegistryClient struct {
	RegistryClient

	images imageclientv1.ImageV1Interface
}

func NewFakeRegistryClient(imageclient imageclientv1.ImageV1Interface) RegistryClient {
	return &fakeRegistryClient{
		RegistryClient: &registryClient{},
		images:         imageclient,
	}
}

func (c *fakeRegistryClient) Client() (Interface, error) {
	return newAPIClient(nil, nil, c.images, nil), nil
}

func NewFakeRegistryAPIClient(kc kcoreclient.CoreInterface, imageclient imageclientv1.ImageV1Interface) Interface {
	return newAPIClient(nil, nil, imageclient, nil)
}
