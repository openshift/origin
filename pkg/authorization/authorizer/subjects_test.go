package authorizer

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
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

	expectedUsers  util.StringSet
	expectedGroups util.StringSet
	expectedError  string
}

func TestSubjects(t *testing.T) {
	test := &subjectsTest{
		context: kapi.WithNamespace(kapi.NewContext(), "adze"),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedUsers:  util.NewStringSet("Anna", "ClusterAdmin", "Ellen", "Valerie", "system:openshift-client", "system:openshift-deployer"),
		expectedGroups: util.NewStringSet("RootUsers", "system:cluster-admins", "system:cluster-readers", "system:nodes"),
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
	clusterPolicyRegistry := clusterpolicyregistry.NewSimulatedRegistry(testpolicyregistry.NewClusterPolicyRegistry(test.clusterPolicies, test.policyRetrievalError))
	clusterPolicyBindingRegistry := clusterpolicybindingregistry.NewSimulatedRegistry(testpolicyregistry.NewClusterPolicyBindingRegistry(test.clusterBindings, test.bindingRetrievalError))

	authorizer := NewAuthorizer(rulevalidation.NewDefaultRuleResolver(policyRegistry, policyBindingRegistry, clusterPolicyRegistry, clusterPolicyBindingRegistry), NewForbiddenMessageResolver(""))

	actualUsers, actualGroups, actualError := authorizer.GetAllowedSubjects(test.context, *test.attributes)

	matchStringSlice(test.expectedUsers.List(), actualUsers.List(), "users", t)
	matchStringSlice(test.expectedGroups.List(), actualGroups.List(), "groups", t)
	matchError(test.expectedError, actualError, "error", t)
}
