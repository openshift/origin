package authorizer

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

type subjectsTest struct {
	policies              []authorizationapi.Policy
	bindings              []authorizationapi.PolicyBinding
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
		expectedUsers:  util.NewStringSet("Anna", "ClusterAdmin", "Ellen", "Valerie", "system:kube-client", "system:openshift-client", "system:openshift-deployer"),
		expectedGroups: util.NewStringSet("RootUsers", "system:cluster-admins", "system:nodes"),
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
	authorizer := NewAuthorizer(bootstrappolicy.DefaultMasterAuthorizationNamespace, rulevalidation.NewDefaultRuleResolver(policyRegistry, policyBindingRegistry))

	actualUsers, actualGroups, actualError := authorizer.GetAllowedSubjects(test.context, *test.attributes)

	matchStringSlice(test.expectedUsers.List(), actualUsers.List(), "users", t)
	matchStringSlice(test.expectedGroups.List(), actualGroups.List(), "groups", t)
	matchError(test.expectedError, actualError, "error", t)
}
