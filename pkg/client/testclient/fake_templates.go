package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	templateapi "github.com/openshift/origin/pkg/template/api"
)

// FakeTemplates implements TemplateInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeTemplates struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeTemplates) Get(name string) (*templateapi.Template, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("templates", c.Namespace, name), &templateapi.Template{})
	if obj == nil {
		return nil, err
	}

	return obj.(*templateapi.Template), err
}

func (c *FakeTemplates) List(label labels.Selector, field fields.Selector) (*templateapi.TemplateList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("templates", c.Namespace, label, field), &templateapi.TemplateList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*templateapi.TemplateList), err
}

func (c *FakeTemplates) Create(inObj *templateapi.Template) (*templateapi.Template, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("templates", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*templateapi.Template), err
}

func (c *FakeTemplates) Update(inObj *templateapi.Template) (*templateapi.Template, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewUpdateAction("templates", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*templateapi.Template), err
}

func (c *FakeTemplates) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("templates", c.Namespace, name), &templateapi.Template{})
	return err
}

func (c *FakeTemplates) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewWatchAction("templates", c.Namespace, label, field, resourceVersion))
}
