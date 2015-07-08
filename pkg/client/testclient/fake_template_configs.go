package testclient

import (
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	templateapi "github.com/openshift/origin/pkg/template/api"
)

// FakeTemplateConfigs implements TemplateConfigsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeTemplateConfigs struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeTemplateConfigs) Create(template *templateapi.Template) (*templateapi.Template, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "create-template-config", Value: template}, &templateapi.Template{})
	return obj.(*templateapi.Template), err
}
