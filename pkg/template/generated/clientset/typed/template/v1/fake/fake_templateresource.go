package fake

import (
	v1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeTemplates implements TemplateResourceInterface
type FakeTemplates struct {
	Fake *FakeTemplateV1
	ns   string
}

var templatesResource = schema.GroupVersionResource{Group: "template.openshift.io", Version: "v1", Resource: "templates"}

var templatesKind = schema.GroupVersionKind{Group: "template.openshift.io", Version: "v1", Kind: "Template"}

func (c *FakeTemplates) Create(templateResource *v1.Template) (result *v1.Template, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(templatesResource, c.ns, templateResource), &v1.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Template), err
}

func (c *FakeTemplates) Update(templateResource *v1.Template) (result *v1.Template, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(templatesResource, c.ns, templateResource), &v1.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Template), err
}

func (c *FakeTemplates) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(templatesResource, c.ns, name), &v1.Template{})

	return err
}

func (c *FakeTemplates) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(templatesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.TemplateList{})
	return err
}

func (c *FakeTemplates) Get(name string, options meta_v1.GetOptions) (result *v1.Template, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(templatesResource, c.ns, name), &v1.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Template), err
}

func (c *FakeTemplates) List(opts meta_v1.ListOptions) (result *v1.TemplateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(templatesResource, templatesKind, c.ns, opts), &v1.TemplateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.TemplateList{}
	for _, item := range obj.(*v1.TemplateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested templates.
func (c *FakeTemplates) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(templatesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched templateResource.
func (c *FakeTemplates) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Template, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(templatesResource, c.ns, name, data, subresources...), &v1.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Template), err
}
