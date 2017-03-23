package fake

import (
	v1 "github.com/openshift/origin/pkg/project/clientset/release_v3_6/typed/project/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeProjectV1 struct {
	*core.Fake
}

func (c *FakeProjectV1) Projects() v1.ProjectResourceInterface {
	return &FakeProjects{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeProjectV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
