package authorizer

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/test"
)

const testMasterNamespace = "master"

type authorizeTest struct {
	globalPolicy         []authorizationapi.Policy
	namespacedPolicy     []authorizationapi.Policy
	policyRetrievalError error

	globalPolicyBinding         []authorizationapi.PolicyBinding
	namespacedPolicyBinding     []authorizationapi.PolicyBinding
	policyBindingRetrievalError error

	attributes *DefaultAuthorizationAttributes

	expectedAllowed bool
	expectedReason  string
	expectedError   string
}

func TestResourceNameDeny(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "just-a-user",
			},
			Verb:         "get",
			Resource:     "users",
			ResourceName: "just-a-user",
			Namespace:    testMasterNamespace,
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.test(t)
}

func TestResourceNameAllow(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "just-a-user",
			},
			Verb:         "get",
			Resource:     "users",
			ResourceName: "~",
			Namespace:    testMasterNamespace,
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in master",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.test(t)
}

func TestDeniedWithError(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "Anna",
			},
			Verb:      "update",
			Resource:  "roles",
			Namespace: "adze",
		},
		expectedAllowed: false,
		expectedError:   "my special error",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.globalPolicyBinding[0].RoleBindings["missing"] = authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "missing",
			Namespace: testMasterNamespace,
		},
		RoleRef: kapi.ObjectReference{
			Name:      "not-a-real-binding",
			Namespace: testMasterNamespace,
		},
		UserNames: []string{"Anna"},
	}
	test.namespacedPolicy, test.namespacedPolicyBinding = newAdzePolicy()
	test.policyRetrievalError = errors.New("my special error")

	test.test(t)
}

func TestAllowedWithMissingBinding(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "Anna",
			},
			Verb:      "update",
			Resource:  "roles",
			Namespace: "adze",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.globalPolicyBinding[0].RoleBindings["missing"] = authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "missing",
			Namespace: testMasterNamespace,
		},
		RoleRef: kapi.ObjectReference{
			Name:      "not-a-real-binding",
			Namespace: testMasterNamespace,
		},
		UserNames: []string{"Anna"},
	}
	test.namespacedPolicy, test.namespacedPolicyBinding = newAdzePolicy()

	test.test(t)
}

func TestAdminEditingGlobalDeploymentConfig(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "ClusterAdmin",
			},
			Verb:      "update",
			Resource:  "deploymentConfigs",
			Namespace: testMasterNamespace,
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in master",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.test(t)
}

func TestDisallowedViewingGlobalPods(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "SomeYahoo",
			},
			Verb:      "get",
			Resource:  "pods",
			Namespace: testMasterNamespace,
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.test(t)
}

func TestProjectAdminEditPolicy(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "Anna",
			},
			Verb:      "update",
			Resource:  "roles",
			Namespace: "adze",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = newAdzePolicy()
	test.test(t)
}

func TestGlobalPolicyOutranksLocalPolicy(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "ClusterAdmin",
			},
			Verb:      "update",
			Resource:  "roles",
			Namespace: "adze",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in master",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = newAdzePolicy()
	test.test(t)
}

func TestResourceRestrictionsWork(t *testing.T) {
	test1 := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "Rachel",
			},
			Verb:      "get",
			Resource:  "buildConfigs",
			Namespace: "adze",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test1.globalPolicy, test1.globalPolicyBinding = newDefaultGlobalPolicy()
	test1.namespacedPolicy, test1.namespacedPolicyBinding = newAdzePolicy()
	test1.test(t)

	test2 := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "Rachel",
			},
			Verb:      "get",
			Resource:  "pods",
			Namespace: "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test2.globalPolicy, test2.globalPolicyBinding = newDefaultGlobalPolicy()
	test2.namespacedPolicy, test2.namespacedPolicyBinding = newAdzePolicy()
	test2.test(t)
}

func TestResourceRestrictionsWithWeirdWork(t *testing.T) {
	test1 := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "Rachel",
			},
			Verb:      "get",
			Resource:  "BUILDCONFIGS",
			Namespace: "adze",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test1.globalPolicy, test1.globalPolicyBinding = newDefaultGlobalPolicy()
	test1.namespacedPolicy, test1.namespacedPolicyBinding = newAdzePolicy()
	test1.test(t)

	test2 := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "Rachel",
			},
			Verb:      "get",
			Resource:  "buildconfigs",
			Namespace: "adze",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test2.globalPolicy, test2.globalPolicyBinding = newDefaultGlobalPolicy()
	test2.namespacedPolicy, test2.namespacedPolicyBinding = newAdzePolicy()
	test2.test(t)
}

func TestLocalRightsDoNotGrantGlobalRights(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "Rachel",
			},
			Verb:      "get",
			Resource:  "buildConfigs",
			Namespace: "backsaw",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = newAdzePolicy()
	test.test(t)
}

func TestVerbRestrictionsWork(t *testing.T) {
	test1 := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "Valerie",
			},
			Verb:      "get",
			Resource:  "buildConfigs",
			Namespace: "adze",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in adze",
	}
	test1.globalPolicy, test1.globalPolicyBinding = newDefaultGlobalPolicy()
	test1.namespacedPolicy, test1.namespacedPolicyBinding = newAdzePolicy()
	test1.test(t)

	test2 := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &user.DefaultInfo{
				Name: "Valerie",
			},
			Verb:      "create",
			Resource:  "buildConfigs",
			Namespace: "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test2.globalPolicy, test2.globalPolicyBinding = newDefaultGlobalPolicy()
	test2.namespacedPolicy, test2.namespacedPolicyBinding = newAdzePolicy()
	test2.test(t)
}

func (test *authorizeTest) test(t *testing.T) {
	policies := make([]authorizationapi.Policy, 0, 0)
	policies = append(policies, test.namespacedPolicy...)
	policies = append(policies, test.globalPolicy...)
	policyRegistry := &testpolicyregistry.PolicyRegistry{
		Err:             test.policyRetrievalError,
		MasterNamespace: testMasterNamespace,
		Policies:        policies,
	}

	policyBindings := make([]authorizationapi.PolicyBinding, 0, 0)
	policyBindings = append(policyBindings, test.namespacedPolicyBinding...)
	policyBindings = append(policyBindings, test.globalPolicyBinding...)
	policyBindingRegistry := &testpolicyregistry.PolicyBindingRegistry{
		Err:             test.policyBindingRetrievalError,
		MasterNamespace: testMasterNamespace,
		PolicyBindings:  policyBindings,
	}
	authorizer := NewAuthorizer(testMasterNamespace, policyRegistry, policyBindingRegistry)

	actualAllowed, actualReason, actualError := authorizer.Authorize(*test.attributes)

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

func newDefaultGlobalPolicy() ([]authorizationapi.Policy, []authorizationapi.PolicyBinding) {
	return []authorizationapi.Policy{*GetBootstrapPolicy(testMasterNamespace)},
		[]authorizationapi.PolicyBinding{
			{
				ObjectMeta: kapi.ObjectMeta{
					Name:      testMasterNamespace,
					Namespace: testMasterNamespace,
				},
				RoleBindings: map[string]authorizationapi.RoleBinding{
					"cluster-admins": {
						ObjectMeta: kapi.ObjectMeta{
							Name:      "cluster-admins",
							Namespace: testMasterNamespace,
						},
						RoleRef: kapi.ObjectReference{
							Name:      "cluster-admin",
							Namespace: testMasterNamespace,
						},
						UserNames:  []string{"ClusterAdmin"},
						GroupNames: []string{"RootUsers"},
					},
					"user-only": {
						ObjectMeta: kapi.ObjectMeta{
							Name:      "user-only",
							Namespace: testMasterNamespace,
						},
						RoleRef: kapi.ObjectReference{
							Name:      "basic-user",
							Namespace: testMasterNamespace,
						},
						UserNames: []string{"just-a-user"},
					},
				},
			},
		}
}

func newAdzePolicy() ([]authorizationapi.Policy, []authorizationapi.PolicyBinding) {
	return append(make([]authorizationapi.Policy, 0, 0),
			authorizationapi.Policy{
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
								Verbs:     []string{"watch", "list", "get"},
								Resources: []string{"buildConfigs"},
							}),
					},
				},
			}),
		append(make([]authorizationapi.PolicyBinding, 0, 0),
			authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{
					Name:      testMasterNamespace,
					Namespace: "adze",
				},
				RoleBindings: map[string]authorizationapi.RoleBinding{
					"projectAdmins": {
						ObjectMeta: kapi.ObjectMeta{
							Name:      "projectAdmins",
							Namespace: "adze",
						},
						RoleRef: kapi.ObjectReference{
							Name:      "admin",
							Namespace: testMasterNamespace,
						},
						UserNames: []string{"Anna"},
					},
					"viewers": {
						ObjectMeta: kapi.ObjectMeta{
							Name:      "viewers",
							Namespace: "adze",
						},
						RoleRef: kapi.ObjectReference{
							Name:      "view",
							Namespace: testMasterNamespace,
						},
						UserNames: []string{"Valerie"},
					},
					"editors": {
						ObjectMeta: kapi.ObjectMeta{
							Name:      "editors",
							Namespace: "adze",
						},
						RoleRef: kapi.ObjectReference{
							Name:      "edit",
							Namespace: testMasterNamespace,
						},
						UserNames: []string{"Ellen"},
					},
				},
			},
			authorizationapi.PolicyBinding{
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
						UserNames: []string{"Rachel"},
					},
				},
			},
		)
}
