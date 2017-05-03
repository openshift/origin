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

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeImage) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
