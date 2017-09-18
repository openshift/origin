package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// FakeUserIdentityMappings implements UserIdentityMappingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeUserIdentityMappings struct {
	Fake *Fake
}

var userIdentityMappingsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "useridentitymappings"}

func (c *FakeUserIdentityMappings) Get(name string, options metav1.GetOptions) (*userapi.UserIdentityMapping, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(userIdentityMappingsResource, name), &userapi.UserIdentityMapping{})
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.UserIdentityMapping), err
}

func (c *FakeUserIdentityMappings) Create(inObj *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(userIdentityMappingsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.UserIdentityMapping), err
}

func (c *FakeUserIdentityMappings) Update(inObj *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(userIdentityMappingsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*userapi.UserIdentityMapping), err
}

func (c *FakeUserIdentityMappings) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(userIdentityMappingsResource, name), &userapi.UserIdentityMapping{})
	return err
}
