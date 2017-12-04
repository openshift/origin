package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

func TestValidatePolicy(t *testing.T) {
	errs := ValidatePolicy(
		&authorizationapi.Policy{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: authorizationapi.PolicyName},
		},
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.Policy
		T field.ErrorType
		F string
	}{
		"zero-length namespace": {
			A: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.PolicyName},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.namespace",
		},
		"zero-length name": {
			A: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid name": {
			A: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "name"},
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.name",
		},
		"mismatched name": {
			A: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: authorizationapi.PolicyName},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "any"},
					},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "roles[any1].metadata.name",
		},
		"has role with attributeRestrictions": {
			A: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{AttributeRestrictions: &authorizationapi.RoleBinding{}},
						},
					},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "roles[any1].rules[0].attributeRestrictions",
		},
	}
	for k, v := range errorCases {
		errs := ValidatePolicy(&v.A, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidatePolicyUpdate(t *testing.T) {
	successCases := map[string]struct {
		newO authorizationapi.Policy
		old  authorizationapi.Policy
	}{
		"no new attribute restrictions": {
			newO: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{AttributeRestrictions: &authorizationapi.RoleBinding{}},
							{},
						},
					},
					"any2": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any2"},
						Rules: []authorizationapi.PolicyRule{
							{},
						},
					},
				},
			},
			old: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{AttributeRestrictions: &authorizationapi.RoleBinding{}},
						},
					},
				},
			},
		},
		"new attribute restrictions of the same type": {
			newO: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{AttributeRestrictions: &authorizationapi.RoleBinding{}},
							{AttributeRestrictions: &authorizationapi.RoleBinding{}},
						},
					},
				},
			},
			old: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{AttributeRestrictions: &authorizationapi.RoleBinding{}},
						},
					},
				},
			},
		},
		"less attribute restrictions": {
			newO: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{AttributeRestrictions: &authorizationapi.RoleBinding{}},
						},
					},
				},
			},
			old: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{AttributeRestrictions: &authorizationapi.RoleBinding{}},
							{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
						},
					},
				},
			},
		},
	}
	for k, v := range successCases {
		errs := ValidatePolicyUpdate(&v.newO, &v.old, true)
		if len(errs) != 0 {
			t.Errorf("%s: expected success: %v", k, errs)
		}
	}

	errorCases := map[string]struct {
		newO authorizationapi.Policy
		old  authorizationapi.Policy
		T    field.ErrorType
		F    string
	}{
		"new attribute restrictions": {
			newO: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{AttributeRestrictions: &authorizationapi.RoleBinding{}},
						},
					},
				},
			},
			old: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{},
						},
					},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "roles[any1].rules[0].attributeRestrictions",
		},
		"attribute restrictions of a different type": {
			newO: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{},
							{},
							{},
							{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
						},
					},
				},
			},
			old: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{AttributeRestrictions: &authorizationapi.RoleBinding{}},
						},
					},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "roles[any1].rules[3].attributeRestrictions",
		},
		"similiar attribute restrictions": {
			newO: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{},
							{},
							{APIGroups: []string{"v2"}, AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
						},
					},
				},
			},
			old: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{},
							{},
							{APIGroups: []string{"v1"}, AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
						},
					},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "roles[any1].rules[2].attributeRestrictions",
		},
		"new role with attribute restriction rules like another role's": {
			newO: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{},
							{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
						},
					},
					"any2": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any2"},
						Rules: []authorizationapi.PolicyRule{
							{},
							{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
						},
					},
				},
			},
			old: authorizationapi.Policy{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any1"},
						Rules: []authorizationapi.PolicyRule{
							{},
							{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
						},
					},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "roles[any2].rules[1].attributeRestrictions",
		},
	}
	for k, v := range errorCases {
		errs := ValidatePolicyUpdate(&v.newO, &v.old, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v %v", k, v.newO, v.old)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidatePolicyBinding(t *testing.T) {
	errs := ValidatePolicyBinding(
		&authorizationapi.PolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: authorizationapi.GetPolicyBindingName("master")},
			PolicyRef:  kapi.ObjectReference{Namespace: "master"},
		},
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.PolicyBinding
		T field.ErrorType
		F string
	}{
		"zero-length namespace": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.GetPolicyBindingName(authorizationapi.PolicyName)},
				PolicyRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.namespace",
		},
		"zero-length name": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault},
				PolicyRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid name": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "name"},
				PolicyRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.name",
		},
		"bad role": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: authorizationapi.GetPolicyBindingName(authorizationapi.PolicyName)},
				PolicyRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"any": {
						ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "any"},
						RoleRef:    kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
					},
				},
			},
			T: field.ErrorTypeRequired,
			F: "roleBindings[any].roleRef.name",
		},
		"mismatched name": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: authorizationapi.GetPolicyBindingName(authorizationapi.PolicyName)},
				PolicyRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"any1": {
						ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "any"},
						RoleRef:    kapi.ObjectReference{Namespace: authorizationapi.PolicyName, Name: "valid"},
					},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "roleBindings[any1].metadata.name",
		},
	}
	for k, v := range errorCases {
		errs := ValidatePolicyBinding(&v.A, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateRoleBinding(t *testing.T) {
	errs := ValidateRoleBinding(
		&authorizationapi.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
			RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
			Subjects: []kapi.ObjectReference{
				{Name: "validsaname", Kind: authorizationapi.ServiceAccountKind},
				{Name: "valid@username", Kind: authorizationapi.UserKind},
				{Name: "system:admin", Kind: authorizationapi.SystemUserKind},
				{Name: "valid@groupname", Kind: authorizationapi.GroupKind},
				{Name: "system:authenticated", Kind: authorizationapi.SystemGroupKind},
			},
		},
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.RoleBinding
		T field.ErrorType
		F string
	}{
		"zero-length namespace": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.PolicyName},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.namespace",
		},
		"zero-length name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid ref": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "name"},
				RoleRef:    kapi.ObjectReference{Namespace: "-192083", Name: "valid"},
			},
			T: field.ErrorTypeInvalid,
			F: "roleRef.namespace",
		},
		"bad role": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: authorizationapi.PolicyName},
				RoleRef:    kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			T: field.ErrorTypeRequired,
			F: "roleRef.name",
		},
		"bad subject kind": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "subject"}},
			},
			T: field.ErrorTypeNotSupported,
			F: "subjects[0].kind",
		},
		"bad subject name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "subject:bad", Kind: authorizationapi.ServiceAccountKind}},
			},
			T: field.ErrorTypeInvalid,
			F: "subjects[0].name",
		},
		"bad system user name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "user", Kind: authorizationapi.SystemUserKind}},
			},
			T: field.ErrorTypeInvalid,
			F: "subjects[0].name",
		},
		"bad system group name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "valid", Kind: authorizationapi.SystemGroupKind}},
			},
			T: field.ErrorTypeInvalid,
			F: "subjects[0].name",
		},
		"forbidden fields": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "subject", Kind: authorizationapi.ServiceAccountKind, APIVersion: "foo"}},
			},
			T: field.ErrorTypeForbidden,
			F: "subjects[0].apiVersion",
		},
		"missing subject name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Kind: authorizationapi.ServiceAccountKind}},
			},
			T: field.ErrorTypeRequired,
			F: "subjects[0].name",
		},
	}
	for k, v := range errorCases {
		errs := ValidateRoleBinding(&v.A, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateRoleBindingUpdate(t *testing.T) {
	old := &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master", ResourceVersion: "1"},
		RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
	}

	errs := ValidateRoleBindingUpdate(
		&authorizationapi.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master", ResourceVersion: "1"},
			RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
		},
		old,
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.RoleBinding
		T field.ErrorType
		F string
	}{
		"changedRef": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master", ResourceVersion: "1"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "changed"},
			},
			T: field.ErrorTypeInvalid,
			F: "roleRef",
		},
	}
	for k, v := range errorCases {
		errs := ValidateRoleBindingUpdate(&v.A, old, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateRole(t *testing.T) {
	errs := ValidateRole(
		&authorizationapi.Role{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "master"},
		},
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.Role
		T field.ErrorType
		F string
	}{
		"zero-length namespace": {
			A: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.PolicyName},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.namespace",
		},
		"zero-length name": {
			A: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid rule": {
			A: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: kapi.NamespaceDefault},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "rules[0].attributeRestrictions",
		},
	}
	for k, v := range errorCases {
		errs := ValidateRole(&v.A, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateRoleUpdate(t *testing.T) {
	successCases := map[string]struct {
		newO authorizationapi.Role
		old  authorizationapi.Role
	}{
		"no attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
			},
		},
		"same attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
		},
	}
	for k, v := range successCases {
		errs := ValidateRoleUpdate(&v.newO, &v.old, true, nil)
		if len(errs) != 0 {
			t.Errorf("%s: expected success: %v", k, errs)
		}
	}

	errorCases := map[string]struct {
		newO authorizationapi.Role
		old  authorizationapi.Role
		T    field.ErrorType
		F    string
	}{
		"more attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.Role{}},
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
					{AttributeRestrictions: &authorizationapi.ClusterPolicyBinding{}},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.Role{}},
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "rules[2].attributeRestrictions",
		},
		"added attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.Role{}},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Rules: []authorizationapi.PolicyRule{
					{},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "rules[0].attributeRestrictions",
		},
		"same number of different attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.Role{}},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "rules[0].attributeRestrictions",
		},
		"less but different attribute restrictions": {
			newO: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName, ResourceVersion: "0"},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.ClusterPolicyBinding{}},
				},
			},
			old: authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Rules: []authorizationapi.PolicyRule{
					{AttributeRestrictions: &authorizationapi.Role{}},
					{AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "rules[0].attributeRestrictions",
		},
	}
	for k, v := range errorCases {
		errs := ValidateRoleUpdate(&v.newO, &v.old, true, nil)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v %v", k, v.newO, v.old)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateClusterPolicyBinding(t *testing.T) {
	errorCases := map[string]struct {
		A authorizationapi.PolicyBinding
		T field.ErrorType
		F string
	}{
		"external namespace ref": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.GetPolicyBindingName("master")},
				PolicyRef:  kapi.ObjectReference{Namespace: "master"},
			},
			T: field.ErrorTypeInvalid,
			F: "policyRef.namespace",
		},
	}
	for k, v := range errorCases {
		errs := ValidatePolicyBinding(&v.A, false)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}
