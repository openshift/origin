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

// FakePolicyBindings implements PolicyBindingInterface
type FakePolicyBindings struct {
	Fake *FakeAuthorizationV1
	ns   string
}

var policybindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "policybindings"}

var policybindingsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "v1", Kind: "PolicyBinding"}

func (c *FakePolicyBindings) Create(policyBinding *v1.PolicyBinding) (result *v1.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(policybindingsResource, c.ns, policyBinding), &v1.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PolicyBinding), err
}

func (c *FakePolicyBindings) Update(policyBinding *v1.PolicyBinding) (result *v1.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(policybindingsResource, c.ns, policyBinding), &v1.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PolicyBinding), err
}

func (c *FakePolicyBindings) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(policybindingsResource, c.ns, name), &v1.PolicyBinding{})

	return err
}

func (c *FakePolicyBindings) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(policybindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.PolicyBindingList{})
	return err
}

func (c *FakePolicyBindings) Get(name string, options meta_v1.GetOptions) (result *v1.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(policybindingsResource, c.ns, name), &v1.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PolicyBinding), err
}

func (c *FakePolicyBindings) List(opts meta_v1.ListOptions) (result *v1.PolicyBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(policybindingsResource, policybindingsKind, c.ns, opts), &v1.PolicyBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.PolicyBindingList{}
	for _, item := range obj.(*v1.PolicyBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested policyBindings.
func (c *FakePolicyBindings) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(policybindingsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched policyBinding.
func (c *FakePolicyBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.PolicyBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(policybindingsResource, c.ns, name, data, subresources...), &v1.PolicyBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PolicyBinding), err
}
