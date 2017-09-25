package fake

import (
	internalversion "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeProject struct {
	*testing.Fake
}

func (c *FakeProject) Projects() internalversion.ProjectResourceInterface {
	return &FakeProjects{c}
}

func (c *FakeProject) ProjectRequests() internalversion.ProjectRequestInterface {
	return &FakeProjectRequests{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeProject) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
