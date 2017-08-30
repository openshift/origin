package fake

import (
	internalversion "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeUser struct {
	*testing.Fake
}

func (c *FakeUser) Groups() internalversion.GroupInterface {
	return &FakeGroups{c}
}

func (c *FakeUser) Identities() internalversion.IdentityInterface {
	return &FakeIdentities{c}
}

func (c *FakeUser) Users() internalversion.UserResourceInterface {
	return &FakeUsers{c}
}

func (c *FakeUser) UserIdentityMappings() internalversion.UserIdentityMappingInterface {
	return &FakeUserIdentityMappings{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeUser) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
