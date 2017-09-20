package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// FakeClusterRoleBindings implements ClusterRoleBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterRoleBindings struct {
	Fake *Fake
}

var clusterRoleBindingsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusterrolebindings"}
var clusterRoleBindingsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "ClusterRoleBinding"}

func (c *FakeClusterRoleBindings) Get(name string, options metav1.GetOptions) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(clusterRoleBindingsResource, name), &authorizationapi.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) List(opts metav1.ListOptions) (*authorizationapi.ClusterRoleBindingList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(clusterRoleBindingsResource, clusterRoleBindingsKind, opts), &authorizationapi.ClusterRoleBindingList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBindingList), err
}

func (c *FakeClusterRoleBindings) Create(inObj *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(clusterRoleBindingsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Update(inObj *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(clusterRoleBindingsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(clusterRoleBindingsResource, name), &authorizationapi.ClusterRoleBinding{})
	return err
}
