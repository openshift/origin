package fake

import (
	internalversion "github.com/openshift/origin/pkg/image/clientset/internalclientset/typed/image/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeImage struct {
	*core.Fake
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

func (c *FakeImage) ImageStreamImages() internalversion.ImageStreamImageInterface {
	return &FakeImageStreamImages{c}
}

func (c *FakeImage) ImageStreamMappings(namespace string) internalversion.ImageStreamMappingInterface {
	return &FakeImageStreamMappings{c, namespace}
}

func (c *FakeImage) ImageStreamTags(namespace string) internalversion.ImageStreamTagInterface {
	return &FakeImageStreamTags{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeImage) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
