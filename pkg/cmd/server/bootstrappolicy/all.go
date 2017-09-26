package bootstrappolicy

import (
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacrest "k8s.io/kubernetes/pkg/registry/rbac/rest"
)

func Policy() *rbacrest.PolicyData {
	return &rbacrest.PolicyData{
		ClusterRoles:        GetBootstrapClusterRoles(),
		ClusterRoleBindings: GetBootstrapClusterRoleBindings(),
		Roles: map[string][]rbac.Role{
			DefaultOpenShiftSharedResourcesNamespace: GetBootstrapOpenshiftRoles(DefaultOpenShiftSharedResourcesNamespace),
		},
		RoleBindings: map[string][]rbac.RoleBinding{
			DefaultOpenShiftSharedResourcesNamespace: GetBootstrapOpenshiftRoleBindings(DefaultOpenShiftSharedResourcesNamespace),
		},
	}
}
