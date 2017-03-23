package fake

import (
	v1 "github.com/openshift/origin/pkg/user/clientset/release_v3_6/typed/user/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeUserV1 struct {
	*core.Fake
}

func (c *FakeUserV1) Users(namespace string) v1.UserResourceInterface {
	return &FakeUsers{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeUserV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
