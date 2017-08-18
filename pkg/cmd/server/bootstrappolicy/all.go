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

func ConvertToOriginClusterRolesOrDie(in []rbac.ClusterRole) []authorizationapi.ClusterRole {
	out := []authorizationapi.ClusterRole{}
	errs := []error{}

	for i := range in {
		newRole := &authorizationapi.ClusterRole{}
		if err := kapi.Scheme.Convert(&in[i], newRole, nil); err != nil {
			errs = append(errs, fmt.Errorf("error converting %q: %v", in[i].Name, err))
			continue
		}
		out = append(out, *newRole)
	}

	if len(errs) > 0 {
		panic(errs)
	}

	return out
}

func ConvertToOriginClusterRoleBindingsOrDie(in []rbac.ClusterRoleBinding) []authorizationapi.ClusterRoleBinding {
	out := []authorizationapi.ClusterRoleBinding{}
	errs := []error{}

	for i := range in {
		newRoleBinding := &authorizationapi.ClusterRoleBinding{}
		if err := kapi.Scheme.Convert(&in[i], newRoleBinding, nil); err != nil {
			errs = append(errs, fmt.Errorf("error converting %q: %v", in[i].Name, err))
			continue
		}
		out = append(out, *newRoleBinding)
	}

	if len(errs) > 0 {
		panic(errs)
	}

	return out
}
