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

// FakeBrokerTemplateInstances implements BrokerTemplateInstanceInterface
type FakeBrokerTemplateInstances struct {
	Fake *FakeTemplate
}

var brokertemplateinstancesResource = schema.GroupVersionResource{Group: "template.openshift.io", Version: "", Resource: "brokertemplateinstances"}

var brokertemplateinstancesKind = schema.GroupVersionKind{Group: "template.openshift.io", Version: "", Kind: "BrokerTemplateInstance"}

// Get takes name of the brokerTemplateInstance, and returns the corresponding brokerTemplateInstance object, and an error if there is any.
func (c *FakeBrokerTemplateInstances) Get(name string, options v1.GetOptions) (result *template.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(brokertemplateinstancesResource, name), &template.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*template.BrokerTemplateInstance), err
}

// List takes label and field selectors, and returns the list of BrokerTemplateInstances that match those selectors.
func (c *FakeBrokerTemplateInstances) List(opts v1.ListOptions) (result *template.BrokerTemplateInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(brokertemplateinstancesResource, brokertemplateinstancesKind, opts), &template.BrokerTemplateInstanceList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &template.BrokerTemplateInstanceList{}
	for _, item := range obj.(*template.BrokerTemplateInstanceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested brokerTemplateInstances.
func (c *FakeBrokerTemplateInstances) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(brokertemplateinstancesResource, opts))
}

// Create takes the representation of a brokerTemplateInstance and creates it.  Returns the server's representation of the brokerTemplateInstance, and an error, if there is any.
func (c *FakeBrokerTemplateInstances) Create(brokerTemplateInstance *template.BrokerTemplateInstance) (result *template.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(brokertemplateinstancesResource, brokerTemplateInstance), &template.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*template.BrokerTemplateInstance), err
}

// Update takes the representation of a brokerTemplateInstance and updates it. Returns the server's representation of the brokerTemplateInstance, and an error, if there is any.
func (c *FakeBrokerTemplateInstances) Update(brokerTemplateInstance *template.BrokerTemplateInstance) (result *template.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(brokertemplateinstancesResource, brokerTemplateInstance), &template.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*template.BrokerTemplateInstance), err
}

// Delete takes name of the brokerTemplateInstance and deletes it. Returns an error if one occurs.
func (c *FakeBrokerTemplateInstances) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(brokertemplateinstancesResource, name), &template.BrokerTemplateInstance{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeBrokerTemplateInstances) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(brokertemplateinstancesResource, listOptions)

	_, err := c.Fake.Invokes(action, &template.BrokerTemplateInstanceList{})
	return err
}

// Patch applies the patch and returns the patched brokerTemplateInstance.
func (c *FakeBrokerTemplateInstances) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *template.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(brokertemplateinstancesResource, name, data, subresources...), &template.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*template.BrokerTemplateInstance), err
}
