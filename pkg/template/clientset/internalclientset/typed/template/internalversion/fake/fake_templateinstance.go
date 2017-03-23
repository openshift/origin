package fake

import (
	api "github.com/openshift/origin/pkg/template/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeTemplateInstances implements TemplateInstanceInterface
type FakeTemplateInstances struct {
	Fake *FakeTemplate
	ns   string
}

var templateinstancesResource = unversioned.GroupVersionResource{Group: "template.openshift.io", Version: "", Resource: "templateinstances"}

func (c *FakeTemplateInstances) Create(templateInstance *api.TemplateInstance) (result *api.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(templateinstancesResource, c.ns, templateInstance), &api.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.TemplateInstance), err
}

func (c *FakeTemplateInstances) Update(templateInstance *api.TemplateInstance) (result *api.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(templateinstancesResource, c.ns, templateInstance), &api.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.TemplateInstance), err
}

func (c *FakeTemplateInstances) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(templateinstancesResource, c.ns, name), &api.TemplateInstance{})

	return err
}

func (c *FakeTemplateInstances) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(templateinstancesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.TemplateInstanceList{})
	return err
}

func (c *FakeTemplateInstances) Get(name string) (result *api.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(templateinstancesResource, c.ns, name), &api.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.TemplateInstance), err
}

func (c *FakeTemplateInstances) List(opts pkg_api.ListOptions) (result *api.TemplateInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(templateinstancesResource, c.ns, opts), &api.TemplateInstanceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.TemplateInstanceList{}
	for _, item := range obj.(*api.TemplateInstanceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested templateInstances.
func (c *FakeTemplateInstances) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(templateinstancesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched templateInstance.
func (c *FakeTemplateInstances) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(templateinstancesResource, c.ns, name, data, subresources...), &api.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.TemplateInstance), err
}
