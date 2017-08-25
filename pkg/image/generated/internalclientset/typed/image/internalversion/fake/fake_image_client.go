package fake

import (
	internalversion "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeImage struct {
	*testing.Fake
}

func (c *FakeImage) Images() internalversion.ImageResourceInterface {
	return &FakeImages{c}
}

func (c *FakeImage) ImageSignatures() internalversion.ImageSignatureInterface {
	return &FakeImageSignatures{c}
}

func (c *FakeImage) ImageStreams(namespace string) internalversion.ImageStreamInterface {
	return &FakeImageStreams{c, namespace}
}

func (c *FakeImage) ImageStreamImages(namespace string) internalversion.ImageStreamImageInterface {
	return &FakeImageStreamImages{c, namespace}
}

func (c *FakeImage) ImageStreamImports(namespace string) internalversion.ImageStreamImportInterface {
	return &FakeImageStreamImports{c, namespace}
}

func (c *FakeImage) ImageStreamMappings(namespace string) internalversion.ImageStreamMappingInterface {
	return &FakeImageStreamMappings{c, namespace}
}

func (c *FakeImage) ImageStreamTags(namespace string) internalversion.ImageStreamTagInterface {
	return &FakeImageStreamTags{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeImage) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
