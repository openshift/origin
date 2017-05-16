package fake

import (
	v1 "github.com/openshift/origin/pkg/authorization/generated/clientset/typed/authorization/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeAuthorizationV1 struct {
	*testing.Fake
}

func (c *FakeAuthorizationV1) ClusterPolicies() v1.ClusterPolicyInterface {
	return &FakeClusterPolicies{c}
}

func (c *FakeAuthorizationV1) ClusterPolicyBindings() v1.ClusterPolicyBindingInterface {
	return &FakeClusterPolicyBindings{c}
}

func (c *FakeAuthorizationV1) ClusterRoles() v1.ClusterRoleInterface {
	return &FakeClusterRoles{c}
}

func (c *FakeAuthorizationV1) ClusterRoleBindings() v1.ClusterRoleBindingInterface {
	return &FakeClusterRoleBindings{c}
}

func (c *FakeAuthorizationV1) Policies(namespace string) v1.PolicyInterface {
	return &FakePolicies{c, namespace}
}

func (c *FakeAuthorizationV1) PolicyBindings(namespace string) v1.PolicyBindingInterface {
	return &FakePolicyBindings{c, namespace}
}

func (c *FakeAuthorizationV1) Roles(namespace string) v1.RoleInterface {
	return &FakeRoles{c, namespace}
}

func (c *FakeAuthorizationV1) RoleBindings(namespace string) v1.RoleBindingInterface {
	return &FakeRoleBindings{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeAuthorizationV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
