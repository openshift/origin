package authorizer

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func TestInvalidRole(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Brad"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "buildConfigs",
		},
		expectedAllowed: false,
		expectedError:   "unable to interpret:",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newInvalidExtensionPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newInvalidExtensionBindings()...)

	test.test(t)
}
func TestInvalidRoleButRuleNotUsed(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Brad"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "buildConfigs",
		},
		expectedAllowed: true,
		expectedReason:  "allowed by rule in mallet",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newInvalidExtensionPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newInvalidExtensionBindings()...)

	test.test(t)
}
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
		expectedReason:  "Victor cannot get on pods in adze",
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
		expectedReason:  "Victor cannot get on policies in mallet",
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
		expectedReason:  "Victor cannot get on policies in adze",
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
		expectedReason:  "Victor cannot create on pods in mallet",
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
		expectedReason:  "Victor cannot create on pods in adze",
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
		expectedReason:  "Edgar cannot update on pods in adze",
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
		expectedReason:  "Edgar cannot update on roleBindings in mallet",
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
		expectedReason:  "Edgar cannot update on roleBindings in adze",
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
		expectedReason:  "Edgar cannot get on pods in adze",
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
		expectedReason:  "Matthew cannot update on roleBindings in adze",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}

func TestAdminUpdateStatusInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Matthew"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "pods/status",
		},
		expectedAllowed: false,
		expectedReason:  "Matthew cannot update on pods/status in mallet",
	}
	test.policies = newDefaultGlobalPolicies()
	test.policies = append(test.policies, newAdzePolicies()...)
	test.policies = append(test.policies, newMalletPolicies()...)
	test.bindings = newDefaultGlobalBinding()
	test.bindings = append(test.bindings, newAdzeBindings()...)
	test.bindings = append(test.bindings, newMalletBindings()...)

	test.test(t)
}
func TestAdminGetStatusInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Matthew"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "get",
			Resource: "pods/status",
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

func TestAdminUpdateDisallowedKindInMallet(t *testing.T) {
	test := &authorizeTest{
		context: kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "mallet"), &user.DefaultInfo{Name: "Matthew"}),
		attributes: &DefaultAuthorizationAttributes{
			Verb:     "update",
			Resource: "policies",
		},
		expectedAllowed: false,
		expectedReason:  "Matthew cannot update on policies in mallet",
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
		expectedReason:  "Matthew cannot update on roles in adze",
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
		expectedReason:  "Matthew cannot get on policies in adze",
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
				Name:      bootstrappolicy.DefaultMasterAuthorizationNamespace,
				Namespace: "mallet",
			},
			RoleBindings: map[string]authorizationapi.RoleBinding{
				"projectAdmins": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "projectAdmins",
						Namespace: "mallet",
					},
					RoleRef: kapi.ObjectReference{
						Name:      bootstrappolicy.AdminRoleName,
						Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
					},
					Users: util.NewStringSet("Matthew"),
				},
				"viewers": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "viewers",
						Namespace: "mallet",
					},
					RoleRef: kapi.ObjectReference{
						Name:      bootstrappolicy.ViewRoleName,
						Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
					},
					Users: util.NewStringSet("Victor"),
				},
				"editors": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "editors",
						Namespace: "mallet",
					},
					RoleRef: kapi.ObjectReference{
						Name:      bootstrappolicy.EditRoleName,
						Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
					},
					Users: util.NewStringSet("Edgar"),
				},
			},
		},
	}
}
func newInvalidExtensionPolicies() []authorizationapi.Policy {
	return []authorizationapi.Policy{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      authorizationapi.PolicyName,
				Namespace: "mallet",
			},
			Roles: map[string]authorizationapi.Role{
				"badExtension": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "failure",
						Namespace: "mallet",
					},
					Rules: []authorizationapi.PolicyRule{
						{
							Verbs:                 util.NewStringSet("watch", "list", "get"),
							Resources:             util.NewStringSet("buildConfigs"),
							AttributeRestrictions: runtime.EmbeddedObject{&authorizationapi.Role{}},
						},
						{
							Verbs:     util.NewStringSet("update"),
							Resources: util.NewStringSet("buildConfigs"),
						},
					},
				},
			},
		}}
}
func newInvalidExtensionBindings() []authorizationapi.PolicyBinding {
	return []authorizationapi.PolicyBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "mallet",
				Namespace: "mallet",
			},
			RoleBindings: map[string]authorizationapi.RoleBinding{
				"borked": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "borked",
						Namespace: "mallet",
					},
					RoleRef: kapi.ObjectReference{
						Name:      "badExtension",
						Namespace: "mallet",
					},
					Users: util.NewStringSet("Brad"),
				},
			},
		},
	}
}

func GetBootstrapPolicy(masterNamespace string) *authorizationapi.Policy {
	policy := &authorizationapi.Policy{
		ObjectMeta: kapi.ObjectMeta{
			Name:              authorizationapi.PolicyName,
			Namespace:         masterNamespace,
			CreationTimestamp: util.Now(),
			UID:               util.NewUUID(),
		},
		LastModified: util.Now(),
		Roles:        make(map[string]authorizationapi.Role),
	}

	for _, role := range bootstrappolicy.GetBootstrapMasterRoles(masterNamespace) {
		policy.Roles[role.Name] = role
	}

	return policy
}

func GetBootstrapPolicyBinding(masterNamespace string) *authorizationapi.PolicyBinding {
	policyBinding := &authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name:              masterNamespace,
			Namespace:         masterNamespace,
			CreationTimestamp: util.Now(),
			UID:               util.NewUUID(),
		},
		LastModified: util.Now(),
		PolicyRef:    kapi.ObjectReference{Namespace: masterNamespace},
		RoleBindings: make(map[string]authorizationapi.RoleBinding),
	}

	for _, roleBinding := range bootstrappolicy.GetBootstrapMasterRoleBindings(masterNamespace) {
		policyBinding.RoleBindings[roleBinding.Name] = roleBinding
	}

	return policyBinding
}
