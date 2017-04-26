package fake

import (
	api "github.com/openshift/origin/pkg/template/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeTemplates implements TemplateResourceInterface
type FakeTemplates struct {
	Fake *FakeTemplate
	ns   string
}

var templatesResource = schema.GroupVersionResource{Group: "template.openshift.io", Version: "", Resource: "templates"}

func (c *FakeTemplates) Create(template *api.Template) (result *api.Template, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(templatesResource, c.ns, template), &api.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Template), err
}

func (c *FakeTemplates) Update(template *api.Template) (result *api.Template, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(templatesResource, c.ns, template), &api.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Template), err
}

func (c *FakeTemplates) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(templatesResource, c.ns, name), &api.Template{})

	return err
}

func (c *FakeTemplates) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(templatesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.TemplateList{})
	return err
}

func (c *FakeTemplates) Get(name string, options v1.GetOptions) (result *api.Template, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(templatesResource, c.ns, name), &api.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Template), err
}

func (c *FakeTemplates) List(opts v1.ListOptions) (result *api.TemplateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(templatesResource, c.ns, opts), &api.TemplateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.TemplateList{}
	for _, item := range obj.(*api.TemplateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested templates.
func (c *FakeTemplates) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(templatesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched template.
func (c *FakeTemplates) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.Template, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(templatesResource, c.ns, name, data, subresources...), &api.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Template), err
}
