package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	core "k8s.io/client-go/testing"

	userapi "github.com/openshift/origin/pkg/user/api"
)

// FakeIdentities implements IdentitiesInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeIdentities struct {
	Fake *Fake
}

var identitiesResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "identities"}

func (c *FakeIdentities) Get(name string, options metav1.GetOptions) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(identitiesResource, name), &userapi.Identity{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) List(opts metainternal.ListOptions) (*userapi.IdentityList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(core.NewRootListAction(identitiesResource, optsv1), &userapi.IdentityList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.IdentityList), err
}

func (c *FakeIdentities) Create(inObj *userapi.Identity) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(identitiesResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) Update(inObj *userapi.Identity) (*userapi.Identity, error) {
	obj, err := c.Fake.Invokes(core.NewRootUpdateAction(identitiesResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.Identity), err
}

func (c *FakeIdentities) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(identitiesResource, name), nil)
	return err
}
