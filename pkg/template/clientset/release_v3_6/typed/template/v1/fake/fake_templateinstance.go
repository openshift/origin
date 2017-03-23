package fake

import (
	v1 "github.com/openshift/origin/pkg/template/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeTemplateInstances implements TemplateInstanceInterface
type FakeTemplateInstances struct {
	Fake *FakeTemplateV1
	ns   string
}

var templateinstancesResource = unversioned.GroupVersionResource{Group: "template.openshift.io", Version: "v1", Resource: "templateinstances"}

func (c *FakeTemplateInstances) Create(templateInstance *v1.TemplateInstance) (result *v1.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(templateinstancesResource, c.ns, templateInstance), &v1.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.TemplateInstance), err
}

func (c *FakeTemplateInstances) Update(templateInstance *v1.TemplateInstance) (result *v1.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(templateinstancesResource, c.ns, templateInstance), &v1.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.TemplateInstance), err
}

func (c *FakeTemplateInstances) UpdateStatus(templateInstance *v1.TemplateInstance) (*v1.TemplateInstance, error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateSubresourceAction(templateinstancesResource, "status", c.ns, templateInstance), &v1.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.TemplateInstance), err
}

func (c *FakeTemplateInstances) Delete(name string, options *api_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(templateinstancesResource, c.ns, name), &v1.TemplateInstance{})

	return err
}

func (c *FakeTemplateInstances) DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error {
	action := core.NewDeleteCollectionAction(templateinstancesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.TemplateInstanceList{})
	return err
}

func (c *FakeTemplateInstances) Get(name string) (result *v1.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(templateinstancesResource, c.ns, name), &v1.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.TemplateInstance), err
}

func (c *FakeTemplateInstances) List(opts api_v1.ListOptions) (result *v1.TemplateInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(templateinstancesResource, c.ns, opts), &v1.TemplateInstanceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.TemplateInstanceList{}
	for _, item := range obj.(*v1.TemplateInstanceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested templateInstances.
func (c *FakeTemplateInstances) Watch(opts api_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(templateinstancesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched templateInstance.
func (c *FakeTemplateInstances) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(templateinstancesResource, c.ns, name, data, subresources...), &v1.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.TemplateInstance), err
}
