package fake

import (
	internalversion "github.com/openshift/origin/pkg/build/clientset/internalclientset/typed/build/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeBuild struct {
	*core.Fake
}

func (c *FakeBuild) Builds(namespace string) internalversion.BuildResourceInterface {
	return &FakeBuilds{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeBuild) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
