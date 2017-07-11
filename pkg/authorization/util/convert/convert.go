package convert

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/kubernetes/pkg/apis/rbac"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// ClusterRoleToRBAC turns an OpenShift ClusterRole into a Kubernetes RBAC
// ClusterRole, the returned object is safe to mutate
func ClusterRoleToRBAC(origin *authorizationapi.ClusterRole) (*rbac.ClusterRole, error) {
	converted := &rbac.ClusterRole{}
	if err := authorizationapi.Convert_authorization_ClusterRole_To_rbac_ClusterRole(origin, converted, nil); err != nil {
		return nil, err
	}
	// do a deep copy here since conversion does not guarantee a new object.
	deepcopied := &rbac.ClusterRole{}
	if err := rbac.DeepCopy_rbac_ClusterRole(converted, deepcopied, cloner); err != nil {
		return nil, err
	}

	return deepcopied, nil
}

// ClusterRoleBindingToRBAC turns an OpenShift ClusterRoleBinding into a Kubernetes
// RBAC ClusterRoleBinding, the returned object is safe to mutate
func ClusterRoleBindingToRBAC(origin *authorizationapi.ClusterRoleBinding) (*rbac.ClusterRoleBinding, error) {
	converted := &rbac.ClusterRoleBinding{}
	if err := authorizationapi.Convert_authorization_ClusterRoleBinding_To_rbac_ClusterRoleBinding(origin, converted, nil); err != nil {
		return nil, err
	}
	// do a deep copy here since conversion does not guarantee a new object.
	deepcopied := &rbac.ClusterRoleBinding{}
	if err := rbac.DeepCopy_rbac_ClusterRoleBinding(converted, deepcopied, cloner); err != nil {
		return nil, err
	}

	return deepcopied, nil
}

// RoleToRBAC turns an OpenShift Role into a Kubernetes RBAC Role,
// the returned object is safe to mutate
func RoleToRBAC(origin *authorizationapi.Role) (*rbac.Role, error) {
	converted := &rbac.Role{}
	if err := authorizationapi.Convert_authorization_Role_To_rbac_Role(origin, converted, nil); err != nil {
		return nil, err
	}
	// do a deep copy here since conversion does not guarantee a new object.
	deepcopied := &rbac.Role{}
	if err := rbac.DeepCopy_rbac_Role(converted, deepcopied, cloner); err != nil {
		return nil, err
	}

	return deepcopied, nil
}

// RoleBindingToRBAC turns an OpenShift RoleBinding into a Kubernetes RBAC
// Rolebinding, the returned object is safe to mutate
func RoleBindingToRBAC(origin *authorizationapi.RoleBinding) (*rbac.RoleBinding, error) {
	converted := &rbac.RoleBinding{}
	if err := authorizationapi.Convert_authorization_RoleBinding_To_rbac_RoleBinding(origin, converted, nil); err != nil {
		return nil, err
	}
	// do a deep copy here since conversion does not guarantee a new object.
	deepcopied := &rbac.RoleBinding{}
	if err := rbac.DeepCopy_rbac_RoleBinding(converted, deepcopied, cloner); err != nil {
		return nil, err
	}

	return deepcopied, nil
}

var cloner = conversion.NewCloner()
