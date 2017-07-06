package fake

import (
	v1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePolicies implements PolicyInterface
type FakePolicies struct {
	Fake *FakeAuthorizationV1
	ns   string
}

var policiesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "policies"}

var policiesKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "Policy"}

func (c *FakePolicies) Create(policy *v1.Policy) (result *v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(policiesResource, c.ns, policy), &v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Policy), err
}

func (c *FakePolicies) Update(policy *v1.Policy) (result *v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(policiesResource, c.ns, policy), &v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Policy), err
}

func (c *FakePolicies) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(policiesResource, c.ns, name), &v1.Policy{})

	return err
}

func (c *FakePolicies) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(policiesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.PolicyList{})
	return err
}

func (c *FakePolicies) Get(name string, options meta_v1.GetOptions) (result *v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(policiesResource, c.ns, name), &v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Policy), err
}

func (c *FakePolicies) List(opts meta_v1.ListOptions) (result *v1.PolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(policiesResource, policiesKind, c.ns, opts), &v1.PolicyList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
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
func (c *FakePolicies) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(policiesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched policy.
func (c *FakePolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Policy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(policiesResource, c.ns, name, data, subresources...), &v1.Policy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Policy), err
}
