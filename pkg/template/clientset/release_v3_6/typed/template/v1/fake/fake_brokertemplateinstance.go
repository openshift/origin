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

// FakeBrokerTemplateInstances implements BrokerTemplateInstanceInterface
type FakeBrokerTemplateInstances struct {
	Fake *FakeTemplateV1
}

var brokertemplateinstancesResource = unversioned.GroupVersionResource{Group: "template.openshift.io", Version: "v1", Resource: "brokertemplateinstances"}

func (c *FakeBrokerTemplateInstances) Create(brokerTemplateInstance *v1.BrokerTemplateInstance) (result *v1.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootCreateAction(brokertemplateinstancesResource, brokerTemplateInstance), &v1.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) Update(brokerTemplateInstance *v1.BrokerTemplateInstance) (result *v1.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateAction(brokertemplateinstancesResource, brokerTemplateInstance), &v1.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) Delete(name string, options *api_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewRootDeleteAction(brokertemplateinstancesResource, name), &v1.BrokerTemplateInstance{})
	return err
}

func (c *FakeBrokerTemplateInstances) DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error {
	action := core.NewRootDeleteCollectionAction(brokertemplateinstancesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.BrokerTemplateInstanceList{})
	return err
}

func (c *FakeBrokerTemplateInstances) Get(name string) (result *v1.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootGetAction(brokertemplateinstancesResource, name), &v1.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) List(opts api_v1.ListOptions) (result *v1.BrokerTemplateInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootListAction(brokertemplateinstancesResource, opts), &v1.BrokerTemplateInstanceList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
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
func (c *FakeBrokerTemplateInstances) Watch(opts api_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewRootWatchAction(brokertemplateinstancesResource, opts))
}

// Patch applies the patch and returns the patched brokerTemplateInstance.
func (c *FakeBrokerTemplateInstances) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootPatchSubresourceAction(brokertemplateinstancesResource, name, data, subresources...), &v1.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.BrokerTemplateInstance), err
}
