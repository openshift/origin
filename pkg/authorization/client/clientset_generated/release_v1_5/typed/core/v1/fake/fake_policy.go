package fake

import (
	v1 "github.com/openshift/origin/pkg/authorization/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakePolicies implements PolicyInterface
type FakePolicies struct {
	Fake *FakeCore
	ns   string
}

var policiesResource = unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "policies"}

func (c *FakePolicies) Create(policy *v1.Policy) (result *v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(policiesResource, c.ns, policy), &v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Policy), err
}

func (c *FakePolicies) Update(policy *v1.Policy) (result *v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(policiesResource, c.ns, policy), &v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Policy), err
}

func (c *FakePolicies) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(policiesResource, c.ns, name), &v1.Policy{})

	return err
}

func (c *FakePolicies) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(policiesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.PolicyList{})
	return err
}

func (c *FakePolicies) Get(name string) (result *v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(policiesResource, c.ns, name), &v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Policy), err
}

func (c *FakePolicies) List(opts api.ListOptions) (result *v1.PolicyList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(policiesResource, c.ns, opts), &v1.PolicyList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.PolicyList{}
	for _, item := range obj.(*v1.PolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested policies.
func (c *FakePolicies) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(policiesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched policy.
func (c *FakePolicies) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(policiesResource, c.ns, name, data, subresources...), &v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Policy), err
}
