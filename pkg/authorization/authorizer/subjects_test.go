package authorizer

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type subjectsTest struct {
	policies              []authorizationapi.Policy
	bindings              []authorizationapi.PolicyBinding
	clusterPolicies       []authorizationapi.ClusterPolicy
	clusterBindings       []authorizationapi.ClusterPolicyBinding
	policyRetrievalError  error
	bindingRetrievalError error

	context    kapi.Context
	attributes *DefaultAuthorizationAttributes

	expectedUsers  sets.String
	expectedGroups sets.String
	expectedError  string
}

func TestSubjects(t *testing.T) {
	test := &subjectsTest{
		context: kapi.WithNamespace(kapi.NewContext(), "adze"),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedUsers:  sets.NewString("Anna", "ClusterAdmin", "Ellen", "Valerie", "system:serviceaccount:adze:second", "system:serviceaccount:foo:default", "system:serviceaccount:other:first", "system:admin"),
		expectedGroups: sets.NewString("RootUsers", "system:cluster-admins", "system:cluster-readers", "system:masters", "system:nodes"),
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()

	test.test(t)
}

func (test *subjectsTest) test(t *testing.T) {
	policyRegistry := testpolicyregistry.NewPolicyRegistry(test.policies, test.policyRetrievalError)
	policyBindingRegistry := testpolicyregistry.NewPolicyBindingRegistry(test.bindings, test.bindingRetrievalError)
	clusterPolicyRegistry := testpolicyregistry.NewClusterPolicyRegistry(test.clusterPolicies, test.policyRetrievalError)
	clusterPolicyBindingRegistry := testpolicyregistry.NewClusterPolicyBindingRegistry(test.clusterBindings, test.bindingRetrievalError)

	authorizer := NewAuthorizer(rulevalidation.NewDefaultRuleResolver(policyRegistry, policyBindingRegistry, clusterPolicyRegistry, clusterPolicyBindingRegistry), NewForbiddenMessageResolver(""))

	actualUsers, actualGroups, actualError := authorizer.GetAllowedSubjects(test.context, *test.attributes)

	matchStringSlice(test.expectedUsers.List(), actualUsers.List(), "users", t)
	matchStringSlice(test.expectedGroups.List(), actualGroups.List(), "groups", t)
	matchError(test.expectedError, actualError, "error", t)
}
