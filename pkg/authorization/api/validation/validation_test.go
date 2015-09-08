package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func TestValidatePolicy(t *testing.T) {
	errs := ValidatePolicy(
		&authorizationapi.Policy{
			ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
		},
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.Policy
		T fielderrors.ValidationErrorType
		F string
	}{
		"zero-length namespace": {
			A: authorizationapi.Policy{
				ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName},
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.namespace",
		},
		"zero-length name": {
			A: authorizationapi.Policy{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault},
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid name": {
			A: authorizationapi.Policy{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "name"},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "metadata.name",
		},
		"mismatched name": {
			A: authorizationapi.Policy{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				Roles: map[string]*authorizationapi.Role{
					"any1": {
						ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any"},
					},
				},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "roles.any1.metadata.name",
		},
	}
	for k, v := range errorCases {
		errs := ValidatePolicy(&v.A, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidatePolicyBinding(t *testing.T) {
	errs := ValidatePolicyBinding(
		&authorizationapi.PolicyBinding{
			ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.GetPolicyBindingName("master")},
			PolicyRef:  kapi.ObjectReference{Namespace: "master"},
		},
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.PolicyBinding
		T fielderrors.ValidationErrorType
		F string
	}{
		"zero-length namespace": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.GetPolicyBindingName(authorizationapi.PolicyName)},
				PolicyRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.namespace",
		},
		"zero-length name": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault},
				PolicyRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid name": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "name"},
				PolicyRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "metadata.name",
		},
		"bad role": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.GetPolicyBindingName(authorizationapi.PolicyName)},
				PolicyRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"any": {
						ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any"},
						RoleRef:    kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
					},
				},
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "roleBindings.any.roleRef.name",
		},
		"mismatched name": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.GetPolicyBindingName(authorizationapi.PolicyName)},
				PolicyRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"any1": {
						ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "any"},
						RoleRef:    kapi.ObjectReference{Namespace: authorizationapi.PolicyName, Name: "valid"},
					},
				},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "roleBindings.any1.metadata.name",
		},
	}
	for k, v := range errorCases {
		errs := ValidatePolicyBinding(&v.A, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateRoleBinding(t *testing.T) {
	errs := ValidateRoleBinding(
		&authorizationapi.RoleBinding{
			ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master"},
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
		T fielderrors.ValidationErrorType
		F string
	}{
		"zero-length namespace": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.namespace",
		},
		"zero-length name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid ref": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "name"},
				RoleRef:    kapi.ObjectReference{Namespace: "-192083", Name: "valid"},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "roleRef.namespace",
		},
		"bad role": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: authorizationapi.PolicyName},
				RoleRef:    kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "roleRef.name",
		},
		"bad subject kind": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "subject"}},
			},
			T: fielderrors.ValidationErrorTypeNotSupported,
			F: "subjects[0].kind",
		},
		"bad subject name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "subject:bad", Kind: authorizationapi.ServiceAccountKind}},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "subjects[0].name",
		},
		"bad system user name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "user", Kind: authorizationapi.SystemUserKind}},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "subjects[0].name",
		},
		"bad system group name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "valid", Kind: authorizationapi.SystemGroupKind}},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "subjects[0].name",
		},
		"forbidden fields": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Name: "subject", Kind: authorizationapi.ServiceAccountKind, APIVersion: "foo"}},
			},
			T: fielderrors.ValidationErrorTypeForbidden,
			F: "subjects[0].apiVersion",
		},
		"missing subject name": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
				Subjects:   []kapi.ObjectReference{{Kind: authorizationapi.ServiceAccountKind}},
			},
			T: fielderrors.ValidationErrorTypeRequired,
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
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateRoleBindingUpdate(t *testing.T) {
	old := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master", ResourceVersion: "1"},
		RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "valid"},
	}

	errs := ValidateRoleBindingUpdate(
		&authorizationapi.RoleBinding{
			ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master", ResourceVersion: "1"},
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
		T fielderrors.ValidationErrorType
		F string
	}{
		"changedRef": {
			A: authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master", ResourceVersion: "1"},
				RoleRef:    kapi.ObjectReference{Namespace: "master", Name: "changed"},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
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
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateRole(t *testing.T) {
	errs := ValidateRole(
		&authorizationapi.Role{
			ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "master"},
		},
		true,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A authorizationapi.Role
		T fielderrors.ValidationErrorType
		F string
	}{
		"zero-length namespace": {
			A: authorizationapi.Role{
				ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName},
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.namespace",
		},
		"zero-length name": {
			A: authorizationapi.Role{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault},
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.name",
		},
	}
	for k, v := range errorCases {
		errs := ValidateRole(&v.A, true)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateClusterPolicyBinding(t *testing.T) {
	errorCases := map[string]struct {
		A authorizationapi.PolicyBinding
		T fielderrors.ValidationErrorType
		F string
	}{
		"external namespace ref": {
			A: authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.GetPolicyBindingName("master")},
				PolicyRef:  kapi.ObjectReference{Namespace: "master"},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
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
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}
