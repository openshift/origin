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

// FakeTemplateInstances implements TemplateInstanceInterface
type FakeTemplateInstances struct {
	Fake *FakeTemplateV1
	ns   string
}

var templateinstancesResource = schema.GroupVersionResource{Group: "template.openshift.io", Version: "v1", Resource: "templateinstances"}

var templateinstancesKind = schema.GroupVersionKind{Group: "template.openshift.io", Version: "v1", Kind: "TemplateInstance"}

func (c *FakeTemplateInstances) Create(templateInstance *v1.TemplateInstance) (result *v1.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(templateinstancesResource, c.ns, templateInstance), &v1.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.TemplateInstance), err
}

func (c *FakeTemplateInstances) Update(templateInstance *v1.TemplateInstance) (result *v1.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(templateinstancesResource, c.ns, templateInstance), &v1.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.TemplateInstance), err
}

func (c *FakeTemplateInstances) UpdateStatus(templateInstance *v1.TemplateInstance) (*v1.TemplateInstance, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(templateinstancesResource, "status", c.ns, templateInstance), &v1.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.TemplateInstance), err
}

func (c *FakeTemplateInstances) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(templateinstancesResource, c.ns, name), &v1.TemplateInstance{})

	return err
}

func (c *FakeTemplateInstances) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(templateinstancesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.TemplateInstanceList{})
	return err
}

func (c *FakeTemplateInstances) Get(name string, options meta_v1.GetOptions) (result *v1.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(templateinstancesResource, c.ns, name), &v1.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.TemplateInstance), err
}

func (c *FakeTemplateInstances) List(opts meta_v1.ListOptions) (result *v1.TemplateInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(templateinstancesResource, templateinstancesKind, c.ns, opts), &v1.TemplateInstanceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
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
func (c *FakeTemplateInstances) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(templateinstancesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched templateInstance.
func (c *FakeTemplateInstances) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(templateinstancesResource, c.ns, name, data, subresources...), &v1.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.TemplateInstance), err
}
