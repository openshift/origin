package bootstrappolicy

import (
	rbacrest "k8s.io/kubernetes/pkg/registry/rbac/rest"
)

func Policy() *rbacrest.PolicyData {
	return &rbacrest.PolicyData{
		ClusterRoles:            GetBootstrapClusterRoles(),
		ClusterRoleBindings:     GetBootstrapClusterRoleBindings(),
		Roles:                   GetBootstrapNamespaceRoles(),
		RoleBindings:            GetBootstrapNamespaceRoleBindings(),
		ClusterRolesToAggregate: GetBootstrapClusterRolesToAggregate(),
	}
}
