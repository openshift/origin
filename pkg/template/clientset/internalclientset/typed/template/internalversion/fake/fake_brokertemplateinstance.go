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

// FakeBrokerTemplateInstances implements BrokerTemplateInstanceInterface
type FakeBrokerTemplateInstances struct {
	Fake *FakeTemplate
}

var brokertemplateinstancesResource = schema.GroupVersionResource{Group: "template.openshift.io", Version: "", Resource: "brokertemplateinstances"}

func (c *FakeBrokerTemplateInstances) Create(brokerTemplateInstance *api.BrokerTemplateInstance) (result *api.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(brokertemplateinstancesResource, brokerTemplateInstance), &api.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) Update(brokerTemplateInstance *api.BrokerTemplateInstance) (result *api.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(brokertemplateinstancesResource, brokerTemplateInstance), &api.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(brokertemplateinstancesResource, name), &api.BrokerTemplateInstance{})
	return err
}

func (c *FakeBrokerTemplateInstances) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(brokertemplateinstancesResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.BrokerTemplateInstanceList{})
	return err
}

func (c *FakeBrokerTemplateInstances) Get(name string, options v1.GetOptions) (result *api.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(brokertemplateinstancesResource, name), &api.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) List(opts v1.ListOptions) (result *api.BrokerTemplateInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(brokertemplateinstancesResource, opts), &api.BrokerTemplateInstanceList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.BrokerTemplateInstanceList{}
	for _, item := range obj.(*api.BrokerTemplateInstanceList).Items {
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

// Patch applies the patch and returns the patched brokerTemplateInstance.
func (c *FakeBrokerTemplateInstances) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(brokertemplateinstancesResource, name, data, subresources...), &api.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.BrokerTemplateInstance), err
}
