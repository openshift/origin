package authorizer

import (
	"testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/test"
)

type subjectsTest struct {
	policies                    []authorizationapi.Policy
	bindings                    []authorizationapi.PolicyBinding
	policyRetrievalError        error
	policyBindingRetrievalError error

	attributes *DefaultAuthorizationAttributes

	expectedUsers  []string
	expectedGroups []string
	expectedError  string
}

func TestSubjects(t *testing.T) {
	test := &subjectsTest{
		attributes: &DefaultAuthorizationAttributes{
			Verb:      "get",
			Resource:  "pods",
			Namespace: "adze",
		},
		expectedUsers:  []string{"Anna", "ClusterAdmin", "Ellen", "Valerie"},
		expectedGroups: []string{"RootUsers"},
	}
	globalPolicy, globalPolicyBinding := newDefaultGlobalPolicy()
	namespacedPolicy, namespacedPolicyBinding := newAdzePolicy()
	test.policies = make([]authorizationapi.Policy, 0, 0)
	test.policies = append(test.policies, namespacedPolicy...)
	test.policies = append(test.policies, globalPolicy...)
	test.bindings = make([]authorizationapi.PolicyBinding, 0, 0)
	test.bindings = append(test.bindings, namespacedPolicyBinding...)
	test.bindings = append(test.bindings, globalPolicyBinding...)

	test.test(t)
}

func (test *subjectsTest) test(t *testing.T) {
	policyRegistry := &testpolicyregistry.PolicyRegistry{
		Err:             test.policyRetrievalError,
		MasterNamespace: testMasterNamespace,
		Policies:        test.policies,
	}

	policyBindingRegistry := &testpolicyregistry.PolicyBindingRegistry{
		Err:             test.policyBindingRetrievalError,
		MasterNamespace: testMasterNamespace,
		PolicyBindings:  test.bindings,
	}
	authorizer := NewAuthorizer(testMasterNamespace, policyRegistry, policyBindingRegistry)

	actualUsers, actualGroups, actualError := authorizer.GetAllowedSubjects(*test.attributes)

	matchStringSlice(test.expectedUsers, actualUsers, "users", t)
	matchStringSlice(test.expectedGroups, actualGroups, "groups", t)
	matchError(test.expectedError, actualError, "error", t)
}
