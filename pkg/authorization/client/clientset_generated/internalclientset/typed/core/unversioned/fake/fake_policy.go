package fake

import (
	api "github.com/openshift/origin/pkg/authorization/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
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

var policiesResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "policies"}

func (c *FakePolicies) Create(policy *api.Policy) (result *api.Policy, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(policiesResource, c.ns, policy), &api.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Policy), err
}

func (c *FakePolicies) Update(policy *api.Policy) (result *api.Policy, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(policiesResource, c.ns, policy), &api.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Policy), err
}

func (c *FakePolicies) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(policiesResource, c.ns, name), &api.Policy{})

	return err
}

func (c *FakePolicies) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(policiesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.PolicyList{})
	return err
}

func (c *FakePolicies) Get(name string) (result *api.Policy, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(policiesResource, c.ns, name), &api.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Policy), err
}

func (c *FakePolicies) List(opts pkg_api.ListOptions) (result *api.PolicyList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(policiesResource, c.ns, opts), &api.PolicyList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
	if label == nil {
		label = labels.Everything()
	}
	list := &api.PolicyList{}
	for _, item := range obj.(*api.PolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested policies.
func (c *FakePolicies) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(policiesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched policy.
func (c *FakePolicies) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.Policy, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(policiesResource, c.ns, name, data, subresources...), &api.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Policy), err
}
