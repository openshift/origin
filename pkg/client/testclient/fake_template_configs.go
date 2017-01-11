package testclient

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	templateapi "github.com/openshift/origin/pkg/template/api"
)

// FakeTemplateConfigs implements TemplateConfigsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeTemplateConfigs struct {
	Fake      *Fake
	Namespace string
}

var templateConfigsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "templateconfigs"}

func (c *FakeTemplateConfigs) Create(inObj *templateapi.Template) (*templateapi.Template, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(templateConfigsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*templateapi.Template), err
}
