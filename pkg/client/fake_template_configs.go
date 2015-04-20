package client

import (
	configapi "github.com/openshift/origin/pkg/config/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

// FakeTemplateConfigs implements TemplateConfigsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeTemplateConfigs struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeTemplateConfigs) Create(template *templateapi.Template) (*configapi.Config, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-template-config", Value: template}, &configapi.Config{})
	return obj.(*configapi.Config), err
}
