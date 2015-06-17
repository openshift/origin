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
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newInvalidExtensionPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newInvalidExtensionBindings()

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
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newInvalidExtensionPolicies()
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newInvalidExtensionBindings()

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
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Victor" cannot get pods in project "adze"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Victor" cannot get policies in project "mallet"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Victor" cannot get policies in project "adze"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Victor" cannot create pods in project "mallet"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Victor" cannot create pods in project "adze"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Edgar" cannot update pods in project "adze"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Edgar" cannot update roleBindings in project "mallet"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Edgar" cannot update roleBindings in project "adze"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Edgar" cannot get pods in project "adze"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Matthew" cannot update roleBindings in project "adze"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Matthew" cannot update pods/status in project "mallet"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Matthew" cannot update policies in project "mallet"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Matthew" cannot update roles in project "adze"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
		expectedReason:  `User "Matthew" cannot get policies in project "adze"`,
	}
	test.clusterPolicies = newDefaultClusterPolicies()
	test.policies = newAdzePolicies()
	test.policies = append(test.policies, newMalletPolicies()...)
	test.clusterBindings = newDefaultClusterPolicyBindings()
	test.bindings = newAdzeBindings()
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
			Roles: map[string]*authorizationapi.Role{},
		}}
}
func newMalletBindings() []authorizationapi.PolicyBinding {
	return []authorizationapi.PolicyBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      authorizationapi.ClusterPolicyBindingName,
				Namespace: "mallet",
			},
			RoleBindings: map[string]*authorizationapi.RoleBinding{
				"projectAdmins": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "projectAdmins",
						Namespace: "mallet",
					},
					RoleRef: kapi.ObjectReference{
						Name: bootstrappolicy.AdminRoleName,
					},
					Users: util.NewStringSet("Matthew"),
				},
				"viewers": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "viewers",
						Namespace: "mallet",
					},
					RoleRef: kapi.ObjectReference{
						Name: bootstrappolicy.ViewRoleName,
					},
					Users: util.NewStringSet("Victor"),
				},
				"editors": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "editors",
						Namespace: "mallet",
					},
					RoleRef: kapi.ObjectReference{
						Name: bootstrappolicy.EditRoleName,
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
			Roles: map[string]*authorizationapi.Role{
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
			RoleBindings: map[string]*authorizationapi.RoleBinding{
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

func GetBootstrapPolicy() *authorizationapi.ClusterPolicy {
	policy := &authorizationapi.ClusterPolicy{
		ObjectMeta: kapi.ObjectMeta{
			Name:              authorizationapi.PolicyName,
			CreationTimestamp: util.Now(),
			UID:               util.NewUUID(),
		},
		LastModified: util.Now(),
		Roles:        make(map[string]*authorizationapi.ClusterRole),
	}

	roles := bootstrappolicy.GetBootstrapClusterRoles()
	for i := range roles {
		policy.Roles[roles[i].Name] = &roles[i]
	}

	return policy
}

func GetBootstrapPolicyBinding() *authorizationapi.ClusterPolicyBinding {
	policyBinding := &authorizationapi.ClusterPolicyBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name:              ":Default",
			CreationTimestamp: util.Now(),
			UID:               util.NewUUID(),
		},
		LastModified: util.Now(),
		RoleBindings: make(map[string]*authorizationapi.ClusterRoleBinding),
	}

	bindings := bootstrappolicy.GetBootstrapClusterRoleBindings()
	for i := range bindings {
		policyBinding.RoleBindings[bindings[i].Name] = &bindings[i]
	}

	return policyBinding
}
