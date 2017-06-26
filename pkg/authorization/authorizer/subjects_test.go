package authorizer

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
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

	attributes kauthorizer.AttributesRecord

	expectedUsers  sets.String
	expectedGroups sets.String
	expectedError  string
}

func TestSubjects(t *testing.T) {
	test := &subjectsTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			Namespace:       "adze",
			Verb:            "get",
			Resource:        "pods",
		},
		expectedUsers: sets.NewString("Anna", "ClusterAdmin", "Ellen", "Valerie",
			"system:serviceaccount:adze:second",
			"system:serviceaccount:foo:default",
			"system:serviceaccount:other:first",
			"system:serviceaccount:kube-system:deployment-controller",
			"system:serviceaccount:kube-system:endpoint-controller",
			"system:serviceaccount:kube-system:generic-garbage-collector",
			"system:serviceaccount:kube-system:namespace-controller",
			"system:serviceaccount:kube-system:persistent-volume-binder",
			"system:serviceaccount:kube-system:statefulset-controller",
			"system:admin",
			"system:kube-scheduler",
			"system:serviceaccount:openshift-infra:build-controller",
			"system:serviceaccount:openshift-infra:deployer-controller",
			"system:serviceaccount:openshift-infra:template-instance-controller",
			"system:serviceaccount:openshift-infra:template-instance-controller",
			"system:serviceaccount:openshift-infra:build-controller",
			"system:serviceaccount:openshift-infra:pv-recycler-controller",
			"system:serviceaccount:openshift-infra:sdn-controller",
		),
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

	_, subjectLocator := NewAuthorizer(rulevalidation.NewDefaultRuleResolver(policyRegistry, policyBindingRegistry, clusterPolicyRegistry, clusterPolicyBindingRegistry), NewForbiddenMessageResolver(""))

	actualUsers, actualGroups, actualError := subjectLocator.GetAllowedSubjects(test.attributes)

	matchStringSlice(test.expectedUsers.List(), actualUsers.List(), "users", t)
	matchStringSlice(test.expectedGroups.List(), actualGroups.List(), "groups", t)
	matchError(test.expectedError, actualError, "error", t)
}
