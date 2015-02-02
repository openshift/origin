package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func TestRoleValidationSuccess(t *testing.T) {
	role := &authorizationapi.Role{}
	role.Name = "my-name"
	role.Namespace = kapi.NamespaceDefault

	if result := ValidateRole(role); len(result) > 0 {
		t.Errorf("Unexpected validation error returned %v", result)
	}
}

func TestRoleValidationFailure(t *testing.T) {
	role := &authorizationapi.Role{}
	role.Namespace = kapi.NamespaceDefault
	if result := ValidateRole(role); len(result) != 1 {
		t.Errorf("Unexpected validation result: %v", result)
	}
}

func TestRoleBindingValidationSuccess(t *testing.T) {
	roleBinding := &authorizationapi.RoleBinding{}
	roleBinding.Name = "my-name"
	roleBinding.Namespace = kapi.NamespaceDefault
	roleBinding.RoleRef.Namespace = kapi.NamespaceDefault

	if result := ValidateRoleBinding(roleBinding); len(result) > 0 {
		t.Errorf("Unexpected validation error returned %v", result)
	}
}

func TestRoleBindingValidationFailure(t *testing.T) {
	roleBinding := &authorizationapi.RoleBinding{}
	roleBinding.Namespace = kapi.NamespaceDefault
	if result := ValidateRoleBinding(roleBinding); len(result) != 2 {
		t.Errorf("Unexpected validation result: %v", result)
	}
}

func TestPolicyBindingValidationSuccess(t *testing.T) {
	policyBinding := &authorizationapi.PolicyBinding{}
	policyBinding.Name = "my-name"
	policyBinding.Namespace = kapi.NamespaceDefault
	policyBinding.PolicyRef.Namespace = "my-name"

	if result := ValidatePolicyBinding(policyBinding); len(result) > 0 {
		t.Errorf("Unexpected validation error returned %v", result)
	}
}

func TestPolicyBindingValidationFailure(t *testing.T) {
	policyBinding := &authorizationapi.PolicyBinding{}
	policyBinding.Namespace = kapi.NamespaceDefault
	if result := ValidatePolicyBinding(policyBinding); len(result) != 2 {
		t.Errorf("Unexpected validation result: %v", result)
	}
}
