package client

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	templateapi "github.com/openshift/origin/pkg/template/api"
)

// FakeTemplateConfigs implements TemplateConfigsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeTemplateConfigs struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeTemplateConfigs) Create(template *templateapi.Template) (*kapi.List, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-template-config", Value: template}, &kapi.List{})
	return obj.(*kapi.List), err
}
