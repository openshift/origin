package fake

import (
	v1 "github.com/openshift/origin/pkg/security/generated/clientset/typed/security/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeSecurityV1 struct {
	*testing.Fake
}

func (c *FakeSecurityV1) SecurityContextConstraints() v1.SecurityContextConstraintsInterface {
	return &FakeSecurityContextConstraints{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeSecurityV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
