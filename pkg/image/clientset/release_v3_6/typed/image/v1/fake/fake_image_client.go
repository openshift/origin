package fake

import (
	v1 "github.com/openshift/origin/pkg/image/clientset/release_v3_6/typed/image/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeImageV1 struct {
	*core.Fake
}

func (c *FakeImageV1) Images() v1.ImageResourceInterface {
	return &FakeImages{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeImageV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
