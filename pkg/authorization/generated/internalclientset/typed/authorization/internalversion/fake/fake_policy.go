package fake

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePolicies implements PolicyInterface
type FakePolicies struct {
	Fake *FakeAuthorization
	ns   string
}

var policiesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "policies"}

var policiesKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "", Kind: "Policy"}

func (c *FakePolicies) Create(policy *authorization.Policy) (result *authorization.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(policiesResource, c.ns, policy), &authorization.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.Policy), err
}

func (c *FakePolicies) Update(policy *authorization.Policy) (result *authorization.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(policiesResource, c.ns, policy), &authorization.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.Policy), err
}

func (c *FakePolicies) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(policiesResource, c.ns, name), &authorization.Policy{})

	return err
}

func (c *FakePolicies) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(policiesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &authorization.PolicyList{})
	return err
}

func (c *FakePolicies) Get(name string, options v1.GetOptions) (result *authorization.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(policiesResource, c.ns, name), &authorization.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.Policy), err
}

func (c *FakePolicies) List(opts v1.ListOptions) (result *authorization.PolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(policiesResource, policiesKind, c.ns, opts), &authorization.PolicyList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &authorization.PolicyList{}
	for _, item := range obj.(*authorization.PolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested policies.
func (c *FakePolicies) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(policiesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched policy.
func (c *FakePolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(policiesResource, c.ns, name, data, subresources...), &authorization.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.Policy), err
}
