package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// FakePolicyBindings implements PolicyBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakePolicyBindings struct {
	Fake      *Fake
	Namespace string
}

var policyBindingsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "policybindings"}
var policyBindingsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "PolicyBinding"}

func (c *FakePolicyBindings) Get(name string, options metav1.GetOptions) (*authorizationapi.PolicyBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(policyBindingsResource, c.Namespace, name), &authorizationapi.PolicyBinding{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.PolicyBinding), err
}

func (c *FakePolicyBindings) List(opts metav1.ListOptions) (*authorizationapi.PolicyBindingList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(policyBindingsResource, policyBindingsKind, c.Namespace, opts), &authorizationapi.PolicyBindingList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.PolicyBindingList), err
}

func (c *FakePolicyBindings) Create(inObj *authorizationapi.PolicyBinding) (*authorizationapi.PolicyBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(policyBindingsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.PolicyBinding), err
}

func (c *FakePolicyBindings) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(policyBindingsResource, c.Namespace, name), &authorizationapi.PolicyBinding{})
	return err
}

func (c *FakePolicyBindings) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewWatchAction(policyBindingsResource, c.Namespace, opts))
}
