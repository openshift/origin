package fake

import (
	internalversion "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeBuild struct {
	*testing.Fake
}

func (c *FakeBuild) Builds(namespace string) internalversion.BuildResourceInterface {
	return &FakeBuilds{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeBuild) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
