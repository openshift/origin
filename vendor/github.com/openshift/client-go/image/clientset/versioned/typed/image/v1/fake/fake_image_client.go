package fake

import (
	v1 "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeImageV1 struct {
	*testing.Fake
}

func (c *FakeImageV1) Images() v1.ImageInterface {
	return &FakeImages{c}
}

func (c *FakeImageV1) ImageSignatures() v1.ImageSignatureInterface {
	return &FakeImageSignatures{c}
}

func (c *FakeImageV1) ImageStreams(namespace string) v1.ImageStreamInterface {
	return &FakeImageStreams{c, namespace}
}

func (c *FakeImageV1) ImageStreamImages(namespace string) v1.ImageStreamImageInterface {
	return &FakeImageStreamImages{c, namespace}
}

func (c *FakeImageV1) ImageStreamImports(namespace string) v1.ImageStreamImportInterface {
	return &FakeImageStreamImports{c, namespace}
}

func (c *FakeImageV1) ImageStreamMappings(namespace string) v1.ImageStreamMappingInterface {
	return &FakeImageStreamMappings{c, namespace}
}

func (c *FakeImageV1) ImageStreamTags(namespace string) v1.ImageStreamTagInterface {
	return &FakeImageStreamTags{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeImageV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
