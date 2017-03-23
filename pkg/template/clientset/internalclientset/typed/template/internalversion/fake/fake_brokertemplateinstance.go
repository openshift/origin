package fake

import (
	api "github.com/openshift/origin/pkg/template/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeBrokerTemplateInstances implements BrokerTemplateInstanceInterface
type FakeBrokerTemplateInstances struct {
	Fake *FakeTemplate
}

var brokertemplateinstancesResource = unversioned.GroupVersionResource{Group: "template.openshift.io", Version: "", Resource: "brokertemplateinstances"}

func (c *FakeBrokerTemplateInstances) Create(brokerTemplateInstance *api.BrokerTemplateInstance) (result *api.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootCreateAction(brokertemplateinstancesResource, brokerTemplateInstance), &api.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) Update(brokerTemplateInstance *api.BrokerTemplateInstance) (result *api.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateAction(brokertemplateinstancesResource, brokerTemplateInstance), &api.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewRootDeleteAction(brokertemplateinstancesResource, name), &api.BrokerTemplateInstance{})
	return err
}

func (c *FakeBrokerTemplateInstances) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewRootDeleteCollectionAction(brokertemplateinstancesResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.BrokerTemplateInstanceList{})
	return err
}

func (c *FakeBrokerTemplateInstances) Get(name string) (result *api.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootGetAction(brokertemplateinstancesResource, name), &api.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.BrokerTemplateInstance), err
}

func (c *FakeBrokerTemplateInstances) List(opts pkg_api.ListOptions) (result *api.BrokerTemplateInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootListAction(brokertemplateinstancesResource, opts), &api.BrokerTemplateInstanceList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
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
func (c *FakeBrokerTemplateInstances) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewRootWatchAction(brokertemplateinstancesResource, opts))
}

// Patch applies the patch and returns the patched brokerTemplateInstance.
func (c *FakeBrokerTemplateInstances) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.BrokerTemplateInstance, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootPatchSubresourceAction(brokertemplateinstancesResource, name, data, subresources...), &api.BrokerTemplateInstance{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.BrokerTemplateInstance), err
}
