package authorizer

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

type authorizeTest struct {
	clusterPolicies      []authorizationapi.ClusterPolicy
	policies             []authorizationapi.Policy
	policyRetrievalError error

	clusterBindings       []authorizationapi.ClusterPolicyBinding
	bindings              []authorizationapi.PolicyBinding
	bindingRetrievalError error

	context    kapi.Context
	attributes *DefaultAuthorizationAttributes

	expectedAllowed bool
	expectedReason  string
	expectedError   string
}

func TestAPIGroupDeny(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Anna"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "list",
			APIGroup: "group",
			Resource: "pods",
		},
		expectedAllowed: false,
		expectedReason:  `User "Anna" cannot list group/pods in project "adze"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.test(t)
}

func TestAPIGroupDefaultAllow(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Anna"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "list",
			Resource: "pods",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.test(t)
}

func TestAPIGroupAllAllow(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Anna"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "list",
			APIGroup: "group",
			Resource: "pods",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.policies[0].Roles["by-group"] = &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "by-group",
			Namespace: "adze",
		},
		Rules: []authorizationapi.PolicyRule{
			{
				Verbs: sets.NewString("list"), APIGroups: []string{"group"}, Resources: sets.NewString("pods"),
			},
		},
	}
	test.bindings[0].RoleBindings["by-group"] = &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name: "by-group",
		},
		RoleRef: kapi.ObjectReference{
			Namespace: "adze",
			Name:      "by-group",
		},
		Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "Anna"}},
	}
	test.test(t)
}

func TestAPIAllAllow(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Anna"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "list",
			APIGroup: "group",
			Resource: "pods",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.policies[0].Roles["by-group"] = &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "by-group",
			Namespace: "adze",
		},
		Rules: []authorizationapi.PolicyRule{
			{
				Verbs: sets.NewString("list"), APIGroups: []string{authorizationapi.APIGroupAll}, Resources: sets.NewString("pods"),
			},
		},
	}
	test.bindings[0].RoleBindings["by-group"] = &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name: "by-group",
		},
		RoleRef: kapi.ObjectReference{
			Namespace: "adze",
			Name:      "by-group",
		},
		Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "Anna"}},
	}
	test.test(t)
}

func TestResourceNameDeny(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceNone), &user.DefaultInfo{Name: "just-a-user"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:         "get",
			Resource:     "users",
			ResourceName: "just-a-user",
		},
		expectedAllowed: false,
		expectedReason:  `User "just-a-user" cannot get users`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.test(t)
}

func TestResourceNameAllow(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceNone), &user.DefaultInfo{Name: "just-a-user"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:         "get",
			Resource:     "users",
			ResourceName: "~",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by cluster rule",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.test(t)
}

func TestClusterBindingServiceAccountSubject(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceNone), &user.DefaultInfo{Name: serviceaccount.MakeUsername("foo", "default")}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:         "get",
			Resource:     "users",
			ResourceName: "any",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by cluster rule",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.test(t)
}

func TestLocalBindingServiceAccountSubject(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: serviceaccount.MakeUsername("adze", "second")}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)

	test.test(t)
}

func TestLocalBindingOtherServiceAccountSubject(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: serviceaccount.MakeUsername("other", "first")}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)

	test.test(t)
}

func TestDeniedWithError(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Anna"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "roles",
		},
		expectedAllowed: false,
		expectedError:   "my special error",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings[0].RoleBindings["missing"] = &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name: "missing",
		},
		RoleRef: kapi.ObjectReference{
			Name: "not-a-real-binding",
		},
		Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "Anna"}},
	}
	test.policyRetrievalError = errors.New("my special error")

	test.test(t)
}

func TestAllowedWithMissingBinding(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Anna"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "roles",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings[0].RoleBindings["missing"] = &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name: "missing",
		},
		RoleRef: kapi.ObjectReference{
			Name: "not-a-real-binding",
		},
		Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "Anna"}},
	}

	test.test(t)
}

func TestHealthAllow(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.NewContext(), &user.DefaultInfo{Name: "no-one", Groups: []string{"system:unauthenticated"}}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:           "get",
			NonResourceURL: true,
			URL:            "/healthz",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by cluster rule",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()

	test.test(t)
}

func TestNonResourceAllow(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.NewContext(), &user.DefaultInfo{Name: "ClusterAdmin"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:           "get",
			NonResourceURL: true,
			URL:            "not-specified",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by cluster rule",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()

	test.test(t)
}

func TestNonResourceDeny(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.NewContext(), &user.DefaultInfo{Name: "no-one"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:           "get",
			NonResourceURL: true,
			URL:            "not-allowed",
		},
		expectedAllowed: false,
		expectedReason:  `User "no-one" cannot "get" on "not-allowed"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()

	test.test(t)
}

func TestHealthDeny(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.NewContext(), &user.DefaultInfo{Name: "no-one"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:           "get",
			NonResourceURL: true,
			URL:            "/healthz",
		},
		expectedAllowed: false,
		expectedReason:  `User "no-one" cannot "get" on "/healthz"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()

	test.test(t)
}

func TestAdminEditingGlobalDeploymentConfig(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceNone), &user.DefaultInfo{Name: "ClusterAdmin"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "deploymentConfigs",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by cluster rule",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()

	test.test(t)
}

func TestDisallowedViewingGlobalPods(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceNone), &user.DefaultInfo{Name: "SomeYahoo"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedAllowed: false,
		expectedReason:  `User "SomeYahoo" cannot get pods`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()

	test.test(t)
}

func TestProjectAdminEditPolicy(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Anna"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "roles",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)

	test.test(t)
}

func TestGlobalPolicyOutranksLocalPolicy(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "ClusterAdmin"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "roles",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by cluster rule",
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)

	test.test(t)
}

func TestResourceRestrictionsWork(t *testing.T) {
	test1 := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Rachel"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "buildConfigs",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test1.clusterPolicies = newDefaultClusterPolicies()
	test1.policies = newAdzePolicies()
	test1.clusterBindings = newDefaultClusterPolicyBindings()
	test1.bindings = newAdzeBindings()
	test1.test(t)

	test2 := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Rachel"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedAllowed: false,
		expectedReason:  `User "Rachel" cannot get pods in project "adze"`,
	}
	test2.clusterPolicies = newDefaultClusterPolicies()
	test2.policies = newAdzePolicies()
	test2.clusterBindings = newDefaultClusterPolicyBindings()
	test2.bindings = newAdzeBindings()
	test2.test(t)
}

func TestResourceRestrictionsWithWeirdWork(t *testing.T) {
	test1 := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Rachel"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "BUILDCONFIGS",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test1.clusterPolicies = newDefaultClusterPolicies()
	test1.policies = newAdzePolicies()
	test1.clusterBindings = newDefaultClusterPolicyBindings()
	test1.bindings = newAdzeBindings()
	test1.test(t)

	test2 := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Rachel"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "buildconfigs",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test2.clusterPolicies = newDefaultClusterPolicies()
	test2.policies = newAdzePolicies()
	test2.clusterBindings = newDefaultClusterPolicyBindings()
	test2.bindings = newAdzeBindings()
	test2.test(t)
}

func TestLocalRightsDoNotGrantGlobalRights(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "backsaw"), &user.DefaultInfo{Name: "Rachel"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "buildConfigs",
		},
		expectedAllowed: false,
		expectedReason:  `User "Rachel" cannot get buildConfigs in project "backsaw"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = append(test.bindings, newAdzeBindings()...)

	test.test(t)
}

func TestVerbRestrictionsWork(t *testing.T) {
	test1 := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Valerie"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "buildConfigs",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test1.clusterPolicies = newDefaultClusterPolicies()
	test1.policies = newAdzePolicies()
	test1.clusterBindings = newDefaultClusterPolicyBindings()
	test1.bindings = newAdzeBindings()
	test1.test(t)

	test2 := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Valerie"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "create",
			Resource: "buildConfigs",
		},
		expectedAllowed: false,
		expectedReason:  `User "Valerie" cannot create buildConfigs in project "adze"`,
	}
	test2.clusterPolicies = newDefaultClusterPolicies()
	test2.policies = newAdzePolicies()
	test2.clusterBindings = newDefaultClusterPolicyBindings()
	test2.bindings = newAdzeBindings()
	test2.test(t)
}

func (test *authorizeTest) test(t *testing.T) {
	policyRegistry := testpolicyregistry.NewPolicyRegistry(test.policies, test.policyRetrievalError)
	policyBindingRegistry := testpolicyregistry.NewPolicyBindingRegistry(test.bindings, test.bindingRetrievalError)
	clusterPolicyRegistry := testpolicyregistry.NewClusterPolicyRegistry(test.clusterPolicies, test.policyRetrievalError)
	clusterPolicyBindingRegistry := testpolicyregistry.NewClusterPolicyBindingRegistry(test.clusterBindings, test.bindingRetrievalError)
	authorizer := NewAuthorizer(rulevalidation.NewDefaultRuleResolver(policyRegistry, policyBindingRegistry, clusterPolicyRegistry, clusterPolicyBindingRegistry), NewForbiddenMessageResolver(""))

	actualAllowed, actualReason, actualError := authorizer.Authorize(test.context, *test.attributes)

	matchBool(test.expectedAllowed, actualAllowed, "allowed", t)
	if actualAllowed {
		containsString(test.expectedReason, actualReason, "allowReason", t)
	} else {
		containsString(test.expectedReason, actualReason, "denyReason", t)
		matchError(test.expectedError, actualError, "error", t)
	}
}

func matchString(expected, actual string, field string, t *testing.T) {
	if expected != actual {
		t.Errorf("%v: Expected %v, got %v", field, expected, actual)
	}
}
func matchStringSlice(expected, actual []string, field string, t *testing.T) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("%v: Expected %v, got %v", field, expected, actual)
	}
}
func containsString(expected, actual string, field string, t *testing.T) {
	if len(expected) == 0 && len(actual) != 0 {
		t.Errorf("%v: Expected %v, got %v", field, expected, actual)
		return
	}
	if !strings.Contains(actual, expected) {
		t.Errorf("%v: Expected %v, got %v", field, expected, actual)
	}
}
func matchBool(expected, actual bool, field string, t *testing.T) {
	if expected != actual {
		t.Errorf("%v: Expected %v, got %v", field, expected, actual)
	}
}
func matchError(expected string, actual error, field string, t *testing.T) {
	if actual == nil {
		if len(expected) != 0 {
			t.Errorf("%v: Expected %v, got %v", field, expected, actual)
			return
		} else {
			return
		}
	}
	if actual != nil && len(expected) == 0 {
		t.Errorf("%v: Expected %v, got %v", field, expected, actual)
		return
	}
	if !strings.Contains(actual.Error(), expected) {
		t.Errorf("%v: Expected %v, got %v", field, expected, actual)
	}
}

func newDefaultClusterPolicies() []authorizationapi.ClusterPolicy {
	return []authorizationapi.ClusterPolicy{*GetBootstrapPolicy()}
}
func newDefaultClusterPolicyBindings() []authorizationapi.ClusterPolicyBinding {
	policyBinding := authorizationapi.ClusterPolicyBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name: authorizationapi.ClusterPolicyBindingName,
		},
		RoleBindings: map[string]*authorizationapi.ClusterRoleBinding{
			"extra-cluster-admins": {
				ObjectMeta: kapi.ObjectMeta{
					Name: "cluster-admins",
				},
				RoleRef: kapi.ObjectReference{
					Name: "cluster-admin",
				},
				Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "ClusterAdmin"}, {Kind: authorizationapi.GroupKind, Name: "RootUsers"}, {Name: "default", Namespace: "foo", Kind: authorizationapi.ServiceAccountKind}},
			},
			"user-only": {
				ObjectMeta: kapi.ObjectMeta{
					Name: "user-only",
				},
				RoleRef: kapi.ObjectReference{
					Name: "basic-user",
				},
				Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "just-a-user"}},
			},
		},
	}
	for key, value := range GetBootstrapPolicyBinding().RoleBindings {
		policyBinding.RoleBindings[key] = value
	}
	return []authorizationapi.ClusterPolicyBinding{policyBinding}
}

func newAdzePolicies() []authorizationapi.Policy {
	return []authorizationapi.Policy{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      authorizationapi.PolicyName,
				Namespace: "adze",
			},
			Roles: map[string]*authorizationapi.Role{
				"restrictedViewer": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "admin",
						Namespace: "adze",
					},
					Rules: append(make([]authorizationapi.PolicyRule, 0),
						authorizationapi.PolicyRule{
							Verbs:     sets.NewString("watch", "list", "get"),
							Resources: sets.NewString("buildConfigs"),
						}),
				},
			},
		}}
}
func newAdzeBindings() []authorizationapi.PolicyBinding {
	return []authorizationapi.PolicyBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      authorizationapi.ClusterPolicyBindingName,
				Namespace: "adze",
			},
			RoleBindings: map[string]*authorizationapi.RoleBinding{
				"projectAdmins": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "projectAdmins",
						Namespace: "adze",
					},
					RoleRef: kapi.ObjectReference{
						Name: bootstrappolicy.AdminRoleName,
					},
					Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "Anna"}},
				},
				"viewers": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "viewers",
						Namespace: "adze",
					},
					RoleRef: kapi.ObjectReference{
						Name: bootstrappolicy.ViewRoleName,
					},
					Subjects: []kapi.ObjectReference{
						{Kind: authorizationapi.UserKind, Name: "Valerie"},
						{Name: "first", Namespace: "other", Kind: authorizationapi.ServiceAccountKind},
						{Name: "second", Kind: authorizationapi.ServiceAccountKind},
					},
				},
				"editors": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "editors",
						Namespace: "adze",
					},
					RoleRef: kapi.ObjectReference{
						Name: bootstrappolicy.EditRoleName,
					},
					Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "Ellen"}},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      authorizationapi.GetPolicyBindingName("adze"),
				Namespace: "adze",
			},
			RoleBindings: map[string]*authorizationapi.RoleBinding{
				"restrictedViewers": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "restrictedViewers",
						Namespace: "adze",
					},
					RoleRef: kapi.ObjectReference{
						Name:      "restrictedViewer",
						Namespace: "adze",
					},
					Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "Rachel"}},
				},
			},
		},
	}
}
