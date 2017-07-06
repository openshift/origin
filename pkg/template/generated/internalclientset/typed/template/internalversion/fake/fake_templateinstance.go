package fake

import (
	template "github.com/openshift/origin/pkg/template/apis/template"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeTemplateInstances implements TemplateInstanceInterface
type FakeTemplateInstances struct {
	Fake *FakeTemplate
	ns   string
}

var templateinstancesResource = schema.GroupVersionResource{Group: "template.openshift.io", Version: "", Resource: "templateinstances"}

var templateinstancesKind = schema.GroupVersionKind{Group: "template.openshift.io", Version: "", Kind: "TemplateInstance"}

func (c *FakeTemplateInstances) Create(templateInstance *template.TemplateInstance) (result *template.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(templateinstancesResource, c.ns, templateInstance), &template.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*template.TemplateInstance), err
}

func (c *FakeTemplateInstances) Update(templateInstance *template.TemplateInstance) (result *template.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(templateinstancesResource, c.ns, templateInstance), &template.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*template.TemplateInstance), err
}

func (c *FakeTemplateInstances) UpdateStatus(templateInstance *template.TemplateInstance) (*template.TemplateInstance, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(templateinstancesResource, "status", c.ns, templateInstance), &template.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*template.TemplateInstance), err
}

func (c *FakeTemplateInstances) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(templateinstancesResource, c.ns, name), &template.TemplateInstance{})

	return err
}

func (c *FakeTemplateInstances) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(templateinstancesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &template.TemplateInstanceList{})
	return err
}

func (c *FakeTemplateInstances) Get(name string, options v1.GetOptions) (result *template.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(templateinstancesResource, c.ns, name), &template.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*template.TemplateInstance), err
}

func (c *FakeTemplateInstances) List(opts v1.ListOptions) (result *template.TemplateInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(templateinstancesResource, templateinstancesKind, c.ns, opts), &template.TemplateInstanceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &template.TemplateInstanceList{}
	for _, item := range obj.(*template.TemplateInstanceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested templateInstances.
func (c *FakeTemplateInstances) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(templateinstancesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched templateInstance.
func (c *FakeTemplateInstances) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *template.TemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(templateinstancesResource, c.ns, name, data, subresources...), &template.TemplateInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*template.TemplateInstance), err
}
