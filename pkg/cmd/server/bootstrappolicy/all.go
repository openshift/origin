package bootstrappolicy

import rbacv1 "k8s.io/api/rbac/v1"

type PolicyData struct {
	ClusterRoles        []rbacv1.ClusterRole
	ClusterRoleBindings []rbacv1.ClusterRoleBinding
	Roles               map[string][]rbacv1.Role
	RoleBindings        map[string][]rbacv1.RoleBinding
	// ClusterRolesToAggregate maps from previous clusterrole name to the new clusterrole name
	ClusterRolesToAggregate map[string]string
}

func Policy() *PolicyData {
	return &PolicyData{
		ClusterRoles:            GetBootstrapClusterRoles(),
		ClusterRoleBindings:     GetBootstrapClusterRoleBindings(),
		Roles:                   GetBootstrapNamespaceRoles(),
		RoleBindings:            GetBootstrapNamespaceRoleBindings(),
		ClusterRolesToAggregate: GetBootstrapClusterRolesToAggregate(),
	}
}
