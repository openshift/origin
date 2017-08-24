package bootstrappolicy

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacrest "k8s.io/kubernetes/pkg/registry/rbac/rest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// TODO: this needs some work since we are double converting

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

func ConvertToOriginClusterRoles(in []rbac.ClusterRole) ([]authorizationapi.ClusterRole, error) {
	out := []authorizationapi.ClusterRole{}

	for i := range in {
		newRole := &authorizationapi.ClusterRole{}
		if err := kapi.Scheme.Convert(&in[i], newRole, nil); err != nil {
			return nil, fmt.Errorf("error converting %q: %v", in[i].Name, err)
		}
		out = append(out, *newRole)
	}
	return out, nil
}

func ConvertToOriginClusterRoleBindings(in []rbac.ClusterRoleBinding) ([]authorizationapi.ClusterRoleBinding, error) {
	out := []authorizationapi.ClusterRoleBinding{}

	for i := range in {
		newRoleBinding := &authorizationapi.ClusterRoleBinding{}
		if err := kapi.Scheme.Convert(&in[i], newRoleBinding, nil); err != nil {
			return nil, fmt.Errorf("error converting %q: %v", in[i].Name, err)
		}
		out = append(out, *newRoleBinding)
	}

	return out, nil
}
