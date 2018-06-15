package bootstrappolicy

import (
	"k8s.io/kubernetes/pkg/apis/rbac"
)

type PolicyData struct {
	ClusterRoles        []rbac.ClusterRole
	ClusterRoleBindings []rbac.ClusterRoleBinding
	Roles               map[string][]rbac.Role
	RoleBindings        map[string][]rbac.RoleBinding
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
