package fake

import (
	v1 "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeTemplateV1 struct {
	*testing.Fake
}

func (c *FakeTemplateV1) BrokerTemplateInstances() v1.BrokerTemplateInstanceInterface {
	return &FakeBrokerTemplateInstances{c}
}

func (c *FakeTemplateV1) Templates(namespace string) v1.TemplateInterface {
	return &FakeTemplates{c, namespace}
}

func (c *FakeTemplateV1) TemplateInstances(namespace string) v1.TemplateInstanceInterface {
	return &FakeTemplateInstances{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeTemplateV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
