package fake

import (
	internalversion "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeTemplate struct {
	*testing.Fake
}

func (c *FakeTemplate) BrokerTemplateInstances() internalversion.BrokerTemplateInstanceInterface {
	return &FakeBrokerTemplateInstances{c}
}

func (c *FakeTemplate) Templates(namespace string) internalversion.TemplateResourceInterface {
	return &FakeTemplates{c, namespace}
}

func (c *FakeTemplate) TemplateInstances(namespace string) internalversion.TemplateInstanceInterface {
	return &FakeTemplateInstances{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeTemplate) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
