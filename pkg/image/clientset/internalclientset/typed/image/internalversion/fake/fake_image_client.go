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

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeImage) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
