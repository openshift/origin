package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// FakeIdentities implements IdentitiesInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeIdentities struct {
	Fake *Fake
}

var identitiesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "identities"}
var identitiesKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "Identity"}

func (c *FakeIdentities) Get(name string, options metav1.GetOptions) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(identitiesResource, name), &userapi.Identity{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) List(opts metav1.ListOptions) (*userapi.IdentityList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(identitiesResource, identitiesKind, opts), &userapi.IdentityList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.IdentityList), err
}

func (c *FakeIdentities) Create(inObj *userapi.Identity) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(identitiesResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) Update(inObj *userapi.Identity) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(identitiesResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(identitiesResource, name), nil)
	return err
}
