package fake

import (
	v1 "github.com/openshift/origin/pkg/authorization/clientset/release_v3_6/typed/authorization/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeAuthorizationV1 struct {
	*core.Fake
}

func (c *FakeAuthorizationV1) Policies(namespace string) v1.PolicyInterface {
	return &FakePolicies{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeAuthorizationV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
