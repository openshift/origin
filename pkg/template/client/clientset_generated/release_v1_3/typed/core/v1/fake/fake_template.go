package fake

import (
	v1 "github.com/openshift/origin/pkg/template/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeTemplates implements TemplateInterface
type FakeTemplates struct {
	Fake *FakeCore
	ns   string
}

var templatesResource = unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "templates"}

func (c *FakeTemplates) Create(template *v1.Template) (result *v1.Template, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(templatesResource, c.ns, template), &v1.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Template), err
}

func (c *FakeTemplates) Update(template *v1.Template) (result *v1.Template, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(templatesResource, c.ns, template), &v1.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Template), err
}

func (c *FakeTemplates) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(templatesResource, c.ns, name), &v1.Template{})

	return err
}

func (c *FakeTemplates) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(templatesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.TemplateList{})
	return err
}

func (c *FakeTemplates) Get(name string) (result *v1.Template, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(templatesResource, c.ns, name), &v1.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Template), err
}

func (c *FakeTemplates) List(opts api.ListOptions) (result *v1.TemplateList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(templatesResource, c.ns, opts), &v1.TemplateList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
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
func (c *FakeTemplates) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(templatesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched template.
func (c *FakeTemplates) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Template, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(templatesResource, c.ns, name, data, subresources...), &v1.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Template), err
}
