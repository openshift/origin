package fake

import (
	internalversion "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeAuthorization struct {
	*testing.Fake
}

func (c *FakeAuthorization) ClusterRoles() internalversion.ClusterRoleInterface {
	return &FakeClusterRoles{c}
}

func (c *FakeAuthorization) ClusterRoleBindings() internalversion.ClusterRoleBindingInterface {
	return &FakeClusterRoleBindings{c}
}

func (c *FakeAuthorization) Policies(namespace string) internalversion.PolicyInterface {
	return &FakePolicies{c, namespace}
}

func (c *FakeAuthorization) Roles(namespace string) internalversion.RoleInterface {
	return &FakeRoles{c, namespace}
}

func (c *FakeAuthorization) RoleBindings(namespace string) internalversion.RoleBindingInterface {
	return &FakeRoleBindings{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeAuthorization) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
