package fake

import (
	internalversion "github.com/openshift/origin/pkg/authorization/clientset/internalclientset/typed/authorization/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeAuthorization struct {
	*core.Fake
}

func (c *FakeAuthorization) Policies(namespace string) internalversion.PolicyInterface {
	return &FakePolicies{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeAuthorization) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
