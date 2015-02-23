package authorizer

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type subjectsTest struct {
	policies              []authorizationapi.Policy
	bindings              []authorizationapi.PolicyBinding
	policyRetrievalError  error
	bindingRetrievalError error

	context    kapi.Context
	attributes *DefaultAuthorizationAttributes

	expectedUsers  []string
	expectedGroups []string
	expectedError  string
}

func TestSubjects(t *testing.T) {
	test := &subjectsTest{
		context: kapi.WithNamespace(kapi.NewContext(), "adze"),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedUsers:  []string{"Anna", "ClusterAdmin", "Ellen", "Valerie", "system:admin", "system:kube-client", "system:openshift-client", "system:openshift-deployer"},
		expectedGroups: []string{"RootUsers", "system:authenticated", "system:unauthenticated"},
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)

	test.test(t)
}

func (test *subjectsTest) test(t *testing.T) {
	policyRegistry := testpolicyregistry.NewPolicyRegistry(test.policies, test.policyRetrievalError)
	policyBindingRegistry := testpolicyregistry.NewPolicyBindingRegistry(test.bindings, test.bindingRetrievalError)
	authorizer := NewAuthorizer(testMasterNamespace, rulevalidation.NewDefaultRuleResolver(policyRegistry, policyBindingRegistry))

	actualUsers, actualGroups, actualError := authorizer.GetAllowedSubjects(test.context, *test.attributes)

	matchStringSlice(test.expectedUsers, actualUsers, "users", t)
	matchStringSlice(test.expectedGroups, actualGroups, "groups", t)
	matchError(test.expectedError, actualError, "error", t)
}
