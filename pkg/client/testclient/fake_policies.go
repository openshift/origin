package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// FakePolicies implements PolicyInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakePolicies struct {
	Fake      *Fake
	Namespace string
}

var policiesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "policies"}
var policiesKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "Policy"}

func (c *FakePolicies) Get(name string, options metav1.GetOptions) (*authorizationapi.Policy, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(policiesResource, c.Namespace, name), &authorizationapi.Policy{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.Policy), err
}

func (c *FakePolicies) List(opts metav1.ListOptions) (*authorizationapi.PolicyList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(policiesResource, policiesKind, c.Namespace, opts), &authorizationapi.PolicyList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.PolicyList), err
}

func (c *FakePolicies) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(policiesResource, c.Namespace, name), &authorizationapi.Policy{})
	return err
}

func (c *FakePolicies) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewWatchAction(policiesResource, c.Namespace, opts))
}
