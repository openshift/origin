package authorizer

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

type authorizeTest struct {
	policies             []authorizationapi.Policy
	policyRetrievalError error

	bindings              []authorizationapi.PolicyBinding
	bindingRetrievalError error

	context    kapi.Context
	attributes *DefaultAuthorizationAttributes

	expectedAllowed bool
	expectedReason  string
	expectedError   string
}

func TestResourceNameDeny(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), bootstrappolicy.DefaultMasterAuthorizationNamespace), &user.DefaultInfo{Name: "just-a-user"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:         "get",
			Resource:     "users",
			ResourceName: "just-a-user",
		},
		expectedAllowed: false,
		expectedReason:  `just-a-user cannot get on users with name "just-a-user"`,
	}
	test.policies = newDefaultGlobalPolicies()
	test.bindings = newDefaultGlobalBinding()
	test.test(t)
}

func TestResourceNameAllow(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), bootstrappolicy.DefaultMasterAuthorizationNamespace), &user.DefaultInfo{Name: "just-a-user"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:         "get",
			Resource:     "users",
			ResourceName: "~",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in master",
	}
	test.policies = newDefaultGlobalPolicies()
	test.bindings = newDefaultGlobalBinding()
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
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings[0].RoleBindings["missing"] = authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "missing",
			Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
		},
		RoleRef: kapi.ObjectReference{
			Name:      "not-a-real-binding",
			Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
		},
		Users: util.NewStringSet("Anna"),
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
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings[0].RoleBindings["missing"] = authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "missing",
			Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
		},
		RoleRef: kapi.ObjectReference{
			Name:      "not-a-real-binding",
			Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
		},
		Users: util.NewStringSet("Anna"),
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
		expectedReason:  "allowed by rule in master",
	}
	test.policies = newDefaultGlobalPolicies()
	test.bindings = newDefaultGlobalBinding()

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
		expectedReason:  "allowed by rule in master",
	}
	test.policies = newDefaultGlobalPolicies()
	test.bindings = newDefaultGlobalBinding()

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
		expectedReason:  `no-one cannot get on not-allowed`,
	}
	test.policies = newDefaultGlobalPolicies()
	test.bindings = newDefaultGlobalBinding()

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
		expectedReason:  `no-one cannot get on /healthz`,
	}
	test.policies = newDefaultGlobalPolicies()
	test.bindings = newDefaultGlobalBinding()

	test.test(t)
}

func TestAdminEditingGlobalDeploymentConfig(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), bootstrappolicy.DefaultMasterAuthorizationNamespace), &user.DefaultInfo{Name: "ClusterAdmin"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "deploymentConfigs",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in master",
	}
	test.policies = newDefaultGlobalPolicies()
	test.bindings = newDefaultGlobalBinding()

	test.test(t)
}

func TestDisallowedViewingGlobalPods(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), bootstrappolicy.DefaultMasterAuthorizationNamespace), &user.DefaultInfo{Name: "SomeYahoo"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedAllowed: false,
		expectedReason:  `SomeYahoo cannot get on pods`,
	}
	test.policies = newDefaultGlobalPolicies()
	test.bindings = newDefaultGlobalBinding()

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
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.bindings = newDefaultGlobalBinding()
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
		expectedReason:  "allowed by rule in master",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.bindings = newDefaultGlobalBinding()
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
	test1.policies = newDefaultGlobalPolicies()
	test1.policies = append(test1.policies, newAdzePolicies()...)
	test1.bindings = newDefaultGlobalBinding()
	test1.bindings = append(test1.bindings, newAdzeBindings()...)
	test1.test(t)

	test2 := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Rachel"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedAllowed: false,
		expectedReason:  `Rachel cannot get on pods in adze`,
	}
	test2.policies = newDefaultGlobalPolicies()
	test2.policies = append(test2.policies, newAdzePolicies()...)
	test2.bindings = newDefaultGlobalBinding()
	test2.bindings = append(test2.bindings, newAdzeBindings()...)
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
	test1.policies = newDefaultGlobalPolicies()
	test1.policies = append(test1.policies, newAdzePolicies()...)
	test1.bindings = newDefaultGlobalBinding()
	test1.bindings = append(test1.bindings, newAdzeBindings()...)
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
	test2.policies = newDefaultGlobalPolicies()
	test2.policies = append(test2.policies, newAdzePolicies()...)
	test2.bindings = newDefaultGlobalBinding()
	test2.bindings = append(test2.bindings, newAdzeBindings()...)
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
		expectedReason:  `Rachel cannot get on buildConfigs in backsaw`,
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.bindings = newDefaultGlobalBinding()
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
	test1.policies = newDefaultGlobalPolicies()
	test1.policies = append(test1.policies, newAdzePolicies()...)
	test1.bindings = newDefaultGlobalBinding()
	test1.bindings = append(test1.bindings, newAdzeBindings()...)
	test1.test(t)

	test2 := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Valerie"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "create",
			Resource: "buildConfigs",
		},
		expectedAllowed: false,
		expectedReason:  `Valerie cannot create on buildConfigs in adze`,
	}
	test2.policies = newDefaultGlobalPolicies()
	test2.policies = append(test2.policies, newAdzePolicies()...)
	test2.bindings = newDefaultGlobalBinding()
	test2.bindings = append(test2.bindings, newAdzeBindings()...)
	test2.test(t)
}

func (test *authorizeTest) test(t *testing.T) {
	policyRegistry := testpolicyregistry.NewPolicyRegistry(test.policies, test.policyRetrievalError)
	policyBindingRegistry := testpolicyregistry.NewPolicyBindingRegistry(test.bindings, test.bindingRetrievalError)
	authorizer := NewAuthorizer(bootstrappolicy.DefaultMasterAuthorizationNamespace, rulevalidation.NewDefaultRuleResolver(policyRegistry, policyBindingRegistry))

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

func newDefaultGlobalPolicies() []authorizationapi.Policy {
	return []authorizationapi.Policy{*GetBootstrapPolicy(bootstrappolicy.DefaultMasterAuthorizationNamespace)}
}
func newDefaultGlobalBinding() []authorizationapi.PolicyBinding {
	policyBinding := authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name:      bootstrappolicy.DefaultMasterAuthorizationNamespace,
			Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
		},
		RoleBindings: map[string]authorizationapi.RoleBinding{
			"cluster-admins": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "cluster-admins",
					Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "cluster-admin",
					Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
				},
				Users:  util.NewStringSet("ClusterAdmin"),
				Groups: util.NewStringSet("RootUsers"),
			},
			"user-only": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "user-only",
					Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "basic-user",
					Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
				},
				Users: util.NewStringSet("just-a-user"),
			},
		},
	}
	for key, value := range GetBootstrapPolicyBinding(bootstrappolicy.DefaultMasterAuthorizationNamespace).RoleBindings {
		policyBinding.RoleBindings[key] = value
	}
	return []authorizationapi.PolicyBinding{policyBinding}
}

func newAdzePolicies() []authorizationapi.Policy {
	return []authorizationapi.Policy{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      authorizationapi.PolicyName,
				Namespace: "adze",
			},
			Roles: map[string]authorizationapi.Role{
				"restrictedViewer": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "admin",
						Namespace: "adze",
					},
					Rules: append(make([]authorizationapi.PolicyRule, 0),
						authorizationapi.PolicyRule{
							Verbs:     util.NewStringSet("watch", "list", "get"),
							Resources: util.NewStringSet("buildConfigs"),
						}),
				},
			},
		}}
}
func newAdzeBindings() []authorizationapi.PolicyBinding {
	return []authorizationapi.PolicyBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      bootstrappolicy.DefaultMasterAuthorizationNamespace,
				Namespace: "adze",
			},
			RoleBindings: map[string]authorizationapi.RoleBinding{
				"projectAdmins": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "projectAdmins",
						Namespace: "adze",
					},
					RoleRef: kapi.ObjectReference{
						Name:      bootstrappolicy.AdminRoleName,
						Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
					},
					Users: util.NewStringSet("Anna"),
				},
				"viewers": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "viewers",
						Namespace: "adze",
					},
					RoleRef: kapi.ObjectReference{
						Name:      bootstrappolicy.ViewRoleName,
						Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
					},
					Users: util.NewStringSet("Valerie"),
				},
				"editors": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "editors",
						Namespace: "adze",
					},
					RoleRef: kapi.ObjectReference{
						Name:      bootstrappolicy.EditRoleName,
						Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
					},
					Users: util.NewStringSet("Ellen"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "adze",
				Namespace: "adze",
			},
			RoleBindings: map[string]authorizationapi.RoleBinding{
				"restrictedViewers": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "restrictedViewers",
						Namespace: "adze",
					},
					RoleRef: kapi.ObjectReference{
						Name:      "restrictedViewer",
						Namespace: "adze",
					},
					Users: util.NewStringSet("Rachel"),
				},
			},
		},
	}
}
