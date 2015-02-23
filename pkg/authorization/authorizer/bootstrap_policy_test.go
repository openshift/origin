package authorizer

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func TestViewerGetAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Victor"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}
func TestViewerGetAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Victor"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}

func TestViewerGetDisallowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Victor"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "policies",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}
func TestViewerGetDisallowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Victor"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "policies",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}

func TestViewerCreateAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Victor"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "create",
			Resource: "pods",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}
func TestViewerCreateAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Victor"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "create",
			Resource: "pods",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}

func TestEditorUpdateAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Edgar"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "pods",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}
func TestEditorUpdateAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Edgar"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "pods",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}

func TestEditorUpdateDisallowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Edgar"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "roleBindings",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}
func TestEditorUpdateDisallowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Edgar"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "roleBindings",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}

func TestEditorGetAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Edgar"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}
func TestEditorGetAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Edgar"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}

func TestAdminUpdateAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Matthew"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "roleBindings",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}
func TestAdminUpdateAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Matthew"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "roleBindings",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}

func TestAdminUpdateDisallowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Matthew"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "policies",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}
func TestAdminUpdateDisallowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Matthew"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "roles",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}

func TestAdminGetAllowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Matthew"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "policies",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}
func TestAdminGetAllowedKindInAdze(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "adze"), &user.DefaultInfo{Name: "Matthew"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "policies",
		},
		expectedAllowed: false,
		expectedReason:  "denied by default",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}

func newMalletPolicies() []authorizationapi.Policy {
	return []authorizationapi.Policy{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      authorizationapi.PolicyName,
				Namespace: "mallet",
			},
			Roles: map[string]authorizationapi.Role{},
		}}
}
func newMalletBindings() []authorizationapi.PolicyBinding {
	return []authorizationapi.PolicyBinding{
		{
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
					Users: util.NewStringSet("Matthew"),
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
					Users: util.NewStringSet("Victor"),
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
					Users: util.NewStringSet("Edgar"),
				},
			},
		},
	}
}
