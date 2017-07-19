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

// FakeBrokerTemplateInstances implements BrokerTemplateInstanceInterface
type FakeBrokerTemplateInstances struct {
	Fake *FakeTemplateV1
}

var brokertemplateinstancesResource = schema.GroupVersionResource{Group: "template.openshift.io", Version: "v1", Resource: "brokertemplateinstances"}

var brokertemplateinstancesKind = schema.GroupVersionKind{Group: "template.openshift.io", Version: "v1", Kind: "BrokerTemplateInstance"}

func (c *FakeBrokerTemplateInstances) Create(brokerTemplateInstance *v1.BrokerTemplateInstance) (result *v1.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(brokertemplateinstancesResource, brokerTemplateInstance), &v1.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) Update(brokerTemplateInstance *v1.BrokerTemplateInstance) (result *v1.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(brokertemplateinstancesResource, brokerTemplateInstance), &v1.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(brokertemplateinstancesResource, name), &v1.BrokerTemplateInstance{})
	return err
}

func (c *FakeBrokerTemplateInstances) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(brokertemplateinstancesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.BrokerTemplateInstanceList{})
	return err
}

func (c *FakeBrokerTemplateInstances) Get(name string, options meta_v1.GetOptions) (result *v1.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(brokertemplateinstancesResource, name), &v1.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) List(opts meta_v1.ListOptions) (result *v1.BrokerTemplateInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(brokertemplateinstancesResource, brokertemplateinstancesKind, opts), &v1.BrokerTemplateInstanceList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.BrokerTemplateInstanceList{}
	for _, item := range obj.(*v1.BrokerTemplateInstanceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested brokerTemplateInstances.
func (c *FakeBrokerTemplateInstances) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(brokertemplateinstancesResource, opts))
}

// Patch applies the patch and returns the patched brokerTemplateInstance.
func (c *FakeBrokerTemplateInstances) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(brokertemplateinstancesResource, name, data, subresources...), &v1.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BrokerTemplateInstance), err
}
