package fake

import (
	v1 "github.com/openshift/origin/pkg/build/clientset/release_v3_6/typed/build/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeBuildV1 struct {
	*core.Fake
}

func (c *FakeBuildV1) Builds(namespace string) v1.BuildResourceInterface {
	return &FakeBuilds{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeBuildV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
