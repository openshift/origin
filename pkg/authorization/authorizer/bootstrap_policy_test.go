package authorizer

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	authenticationapi "github.com/openshift/origin/pkg/auth/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func TestViewerGetAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Victor",
			},
			Verb:         "get",
			ResourceKind: "pods",
			Namespace:    "mallet",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}
func TestViewerGetAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Victor",
			},
			Verb:         "get",
			ResourceKind: "pods",
			Namespace:    "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}

func TestViewerGetDisallowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Victor",
			},
			Verb:         "get",
			ResourceKind: "policies",
			Namespace:    "mallet",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}
func TestViewerGetDisallowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Victor",
			},
			Verb:         "get",
			ResourceKind: "policies",
			Namespace:    "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}

func TestViewerCreateAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Victor",
			},
			Verb:         "create",
			ResourceKind: "pods",
			Namespace:    "mallet",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}
func TestViewerCreateAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Victor",
			},
			Verb:         "create",
			ResourceKind: "pods",
			Namespace:    "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}

func TestEditorUpdateAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Edgar",
			},
			Verb:         "update",
			ResourceKind: "pods",
			Namespace:    "mallet",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}
func TestEditorUpdateAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Edgar",
			},
			Verb:         "update",
			ResourceKind: "pods",
			Namespace:    "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}

func TestEditorUpdateDisallowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Edgar",
			},
			Verb:         "update",
			ResourceKind: "roleBindings",
			Namespace:    "mallet",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}
func TestEditorUpdateDisallowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Edgar",
			},
			Verb:         "update",
			ResourceKind: "roleBindings",
			Namespace:    "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}

func TestEditorGetAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Edgar",
			},
			Verb:         "get",
			ResourceKind: "pods",
			Namespace:    "mallet",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}
func TestEditorGetAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Edgar",
			},
			Verb:         "get",
			ResourceKind: "pods",
			Namespace:    "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}

func TestAdminUpdateAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Matthew",
			},
			Verb:         "update",
			ResourceKind: "roleBindings",
			Namespace:    "mallet",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}
func TestAdminUpdateAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Matthew",
			},
			Verb:         "update",
			ResourceKind: "roleBindings",
			Namespace:    "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}

func TestAdminUpdateDisallowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Matthew",
			},
			Verb:         "update",
			ResourceKind: "policies",
			Namespace:    "mallet",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}
func TestAdminUpdateDisallowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Matthew",
			},
			Verb:         "update",
			ResourceKind: "roles",
			Namespace:    "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}

func TestAdminGetAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Matthew",
			},
			Verb:         "get",
			ResourceKind: "policies",
			Namespace:    "mallet",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}
func TestAdminGetAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		attributes: &DefaultAuthorizationAttributes{
			User: &authenticationapi.DefaultUserInfo{
				Name: "Matthew",
			},
			Verb:         "get",
			ResourceKind: "policies",
			Namespace:    "adze",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.globalPolicy, test.globalPolicyBinding = newDefaultGlobalPolicy()
	test.namespacedPolicy, test.namespacedPolicyBinding = allNamespacedPolicies()
	test.test(t)
}

func allNamespacedPolicies() ([]authorizationapi.Policy, []authorizationapi.PolicyBinding) {
	adzePolicy, adzeBinding := newMalletPolicy()
	malletPolicy, malletBinding := newMalletPolicy()

	policies := make([]authorizationapi.Policy, 0)
	policies = append(policies, adzePolicy...)
	policies = append(policies, malletPolicy...)

	bindings := make([]authorizationapi.PolicyBinding, 0)
	bindings = append(bindings, adzeBinding...)
	bindings = append(bindings, malletBinding...)

	return policies, bindings

}

func newMalletPolicy() ([]authorizationapi.Policy, []authorizationapi.PolicyBinding) {
	return append(make([]authorizationapi.Policy, 0, 0),
			authorizationapi.Policy{
				ObjectMeta: kapi.ObjectMeta{
					Name:      authorizationapi.PolicyName,
					Namespace: "mallet",
				},
				Roles: map[string]authorizationapi.Role{},
			}),
		append(make([]authorizationapi.PolicyBinding, 0, 0),
			authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{
					Name:      testMasterNamespace,
					Namespace: "mallet",
				},
				RoleBindings: map[string]authorizationapi.RoleBinding{
					"projectAdmins": {
						ObjectMeta: kapi.ObjectMeta{
							Name:      "projectAdmins",
							Namespace: "mallet",
						},
						RoleRef: kapi.ObjectReference{
							Name:      "admin",
							Namespace: testMasterNamespace,
						},
						UserNames: append(make([]string, 0), "Matthew"),
					},
					"viewers": {
						ObjectMeta: kapi.ObjectMeta{
							Name:      "viewers",
							Namespace: "mallet",
						},
						RoleRef: kapi.ObjectReference{
							Name:      "view",
							Namespace: testMasterNamespace,
						},
						UserNames: append(make([]string, 0), "Victor"),
					},
					"editors": {
						ObjectMeta: kapi.ObjectMeta{
							Name:      "editors",
							Namespace: "mallet",
						},
						RoleRef: kapi.ObjectReference{
							Name:      "edit",
							Namespace: testMasterNamespace,
						},
						UserNames: append(make([]string, 0), "Edgar"),
					},
				},
			},
		)
}
