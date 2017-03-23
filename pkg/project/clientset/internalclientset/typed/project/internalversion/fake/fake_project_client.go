package fake

import (
	internalversion "github.com/openshift/origin/pkg/project/clientset/internalclientset/typed/project/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeProject struct {
	*core.Fake
}

func (c *FakeProject) Projects() internalversion.ProjectResourceInterface {
	return &FakeProjects{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeProject) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
