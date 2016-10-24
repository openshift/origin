package fake

import (
	api "github.com/openshift/origin/pkg/template/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
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

var templatesResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "templates"}

func (c *FakeTemplates) Create(template *api.Template) (result *api.Template, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(templatesResource, c.ns, template), &api.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Template), err
}

func (c *FakeTemplates) Update(template *api.Template) (result *api.Template, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(templatesResource, c.ns, template), &api.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Template), err
}

func (c *FakeTemplates) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(templatesResource, c.ns, name), &api.Template{})

	return err
}

func (c *FakeTemplates) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(templatesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.TemplateList{})
	return err
}

func (c *FakeTemplates) Get(name string) (result *api.Template, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(templatesResource, c.ns, name), &api.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Template), err
}

func (c *FakeTemplates) List(opts pkg_api.ListOptions) (result *api.TemplateList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(templatesResource, c.ns, opts), &api.TemplateList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
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
func (c *FakeTemplates) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(templatesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched template.
func (c *FakeTemplates) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Template, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(templatesResource, c.ns, name, data, subresources...), &api.Template{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Template), err
}
