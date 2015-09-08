package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/testclient"

	templateapi "github.com/openshift/origin/pkg/template/api"
)

// FakeTemplateConfigs implements TemplateConfigsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeTemplateConfigs struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeTemplateConfigs) Create(inObj *templateapi.Template) (*templateapi.Template, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("templateconfigs", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*templateapi.Template), err
}
