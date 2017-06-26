package fake

import (
	internalversion "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeSecurity struct {
	*testing.Fake
}

func (c *FakeSecurity) SecurityContextConstraints() internalversion.SecurityContextConstraintsInterface {
	return &FakeSecurityContextConstraints{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeSecurity) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
