package bootstrappolicy

import (
	"strings"
	"testing"
)

// TestSystemRoleNames makes sure that our system roles all conform to our contract of starting with system:
func TestSystemRoleNames(t *testing.T) {
	for _, role := range GetSystemClusterRoles() {
		if !strings.HasPrefix(role.Name, "system:") {
			t.Errorf("system role MUST start with system:, not %v", role.Name)
		}
	}
}

// TestSystemRoleBindingRefs makes sure that our system role bindings all conform to our contract of starting with system: and
// only refer to roles present in system roles
func TestSystemRoleBindingRefs(t *testing.T) {
	roles := GetSystemClusterRoles()

	for _, binding := range GetSystemClusterRoleBindings() {
		if !strings.HasPrefix(binding.Name, "system:") {
			t.Errorf("system rolebinding MUST start with system:, not %v", binding.Name)
		}

		found := false
		for _, role := range roles {
			if role.Name == binding.RoleRef.Name {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("system rolebinding MUST refer to a system role (GetSystemClusterRoles), not %v", binding.RoleRef.Name)
		}
	}
}

// TestBootstrapRoleNames makes sure that we aren't creating any system: roles in our policy.json file
func TestBootstrapRoleNames(t *testing.T) {
	for _, role := range GetBootstrapClusterRoles() {
		if strings.HasPrefix(role.Name, "system:") {
			t.Errorf("bootstrap role MAY NOT start with system:, not %v", role.Name)
		}
	}
}

// TestBootstrapRoleBindingRefs makes sure that our bootstrap role bindings don't start with system: and don't refer to
// to system roles.  They also make sure that they do refer to real cluster roles
func TestBootstrapRoleBindingRefs(t *testing.T) {
	roles := GetBootstrapClusterRoles()

	for _, binding := range GetBootstrapClusterRoleBindings() {
		if strings.HasPrefix(binding.Name, "system:") {
			t.Errorf("system rolebinding MAY NOT start with system:, not %v", binding.Name)
		}

		found := false
		for _, role := range roles {
			if role.Name == binding.RoleRef.Name {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("system rolebinding MUST refer to a boot role (GetBootstrapClusterRoles), not %v", binding.RoleRef.Name)
		}
	}
}
