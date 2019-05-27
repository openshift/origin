package util

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/apis/authorization/rbacconversion"
)

// ClusterRoleToRBAC turns an OpenShift ClusterRole into a Kubernetes RBAC
// ClusterRole, the returned object is safe to mutate
func ClusterRoleToRBAC(obj *authorizationapi.ClusterRole) (*rbacv1.ClusterRole, error) {
	// do a deep copy here since conversion does not guarantee a new object.
	objCopy := obj.DeepCopy()

	convertedObjInternal := &rbac.ClusterRole{}
	if err := rbacconversion.Convert_authorization_ClusterRole_To_rbac_ClusterRole(objCopy, convertedObjInternal, nil); err != nil {
		return nil, err
	}

	convertedObj := &rbacv1.ClusterRole{}
	if err := rbacv1helpers.Convert_rbac_ClusterRole_To_v1_ClusterRole(convertedObjInternal, convertedObj, nil); err != nil {
		return nil, err
	}

	return convertedObj, nil
}

// ClusterRoleBindingToRBAC turns an OpenShift ClusterRoleBinding into a Kubernetes
// RBAC ClusterRoleBinding, the returned object is safe to mutate
func ClusterRoleBindingToRBAC(obj *authorizationapi.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
	// do a deep copy here since conversion does not guarantee a new object.
	objCopy := obj.DeepCopy()

	convertedObjInternal := &rbac.ClusterRoleBinding{}
	if err := rbacconversion.Convert_authorization_ClusterRoleBinding_To_rbac_ClusterRoleBinding(objCopy, convertedObjInternal, nil); err != nil {
		return nil, err
	}

	convertedObj := &rbacv1.ClusterRoleBinding{}
	if err := rbacv1helpers.Convert_rbac_ClusterRoleBinding_To_v1_ClusterRoleBinding(convertedObjInternal, convertedObj, nil); err != nil {
		return nil, err
	}

	return convertedObj, nil
}

// RoleToRBAC turns an OpenShift Role into a Kubernetes RBAC Role,
// the returned object is safe to mutate
func RoleToRBAC(obj *authorizationapi.Role) (*rbacv1.Role, error) {
	// do a deep copy here since conversion does not guarantee a new object.
	objCopy := obj.DeepCopy()

	convertedObjInternal := &rbac.Role{}
	if err := rbacconversion.Convert_authorization_Role_To_rbac_Role(objCopy, convertedObjInternal, nil); err != nil {
		return nil, err
	}

	convertedObj := &rbacv1.Role{}
	if err := rbacv1helpers.Convert_rbac_Role_To_v1_Role(convertedObjInternal, convertedObj, nil); err != nil {
		return nil, err
	}

	return convertedObj, nil
}

// RoleBindingToRBAC turns an OpenShift RoleBinding into a Kubernetes RBAC
// Rolebinding, the returned object is safe to mutate
func RoleBindingToRBAC(obj *authorizationapi.RoleBinding) (*rbacv1.RoleBinding, error) {
	// do a deep copy here since conversion does not guarantee a new object.
	objCopy := obj.DeepCopy()

	convertedObjInternal := &rbac.RoleBinding{}
	if err := rbacconversion.Convert_authorization_RoleBinding_To_rbac_RoleBinding(objCopy, convertedObjInternal, nil); err != nil {
		return nil, err
	}

	convertedObj := &rbacv1.RoleBinding{}
	if err := rbacv1helpers.Convert_rbac_RoleBinding_To_v1_RoleBinding(convertedObjInternal, convertedObj, nil); err != nil {
		return nil, err
	}

	return convertedObj, nil
}

// ClusterRoleFromRBAC turns a Kubernetes RBAC ClusterRole into an Openshift
// ClusterRole, the returned object is safe to mutate
func ClusterRoleFromRBAC(obj *rbacv1.ClusterRole) (*authorizationapi.ClusterRole, error) {
	// do a deep copy here since conversion does not guarantee a new object.
	objCopy := obj.DeepCopy()

	convertedObjInternal := &rbac.ClusterRole{}
	if err := rbacv1helpers.Convert_v1_ClusterRole_To_rbac_ClusterRole(objCopy, convertedObjInternal, nil); err != nil {
		return nil, err
	}

	convertedObj := &authorizationapi.ClusterRole{}
	if err := rbacconversion.Convert_rbac_ClusterRole_To_authorization_ClusterRole(convertedObjInternal, convertedObj, nil); err != nil {
		return nil, err
	}

	return convertedObj, nil
}

// ClusterRoleBindingFromRBAC turns a Kuberenets RBAC ClusterRoleBinding into
// an Openshift ClusterRoleBinding, the returned object is safe to mutate
func ClusterRoleBindingFromRBAC(obj *rbacv1.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	// do a deep copy here since conversion does not guarantee a new object.
	objCopy := obj.DeepCopy()

	convertedObjInternal := &rbac.ClusterRoleBinding{}
	if err := rbacv1helpers.Convert_v1_ClusterRoleBinding_To_rbac_ClusterRoleBinding(objCopy, convertedObjInternal, nil); err != nil {
		return nil, err
	}

	convertedObj := &authorizationapi.ClusterRoleBinding{}
	if err := rbacconversion.Convert_rbac_ClusterRoleBinding_To_authorization_ClusterRoleBinding(convertedObjInternal, convertedObj, nil); err != nil {
		return nil, err
	}

	return convertedObj, nil
}

// RoleFromRBAC turns a Kubernetes RBAC Role into an OpenShift Role,
// the returned object is safe to mutate
func RoleFromRBAC(obj *rbacv1.Role) (*authorizationapi.Role, error) {
	// do a deep copy here since conversion does not guarantee a new object.
	objCopy := obj.DeepCopy()

	convertedObjInternal := &rbac.Role{}
	if err := rbacv1helpers.Convert_v1_Role_To_rbac_Role(objCopy, convertedObjInternal, nil); err != nil {
		return nil, err
	}

	convertedObj := &authorizationapi.Role{}
	if err := rbacconversion.Convert_rbac_Role_To_authorization_Role(convertedObjInternal, convertedObj, nil); err != nil {
		return nil, err
	}

	return convertedObj, nil
}

// RoleBindingFromRBAC turns a Kubernetes RBAC RoleBinding into an OpenShift
// Rolebinding, the returned object is safe to mutate
func RoleBindingFromRBAC(obj *rbacv1.RoleBinding) (*authorizationapi.RoleBinding, error) {
	// do a deep copy here since conversion does not guarantee a new object.
	objCopy := obj.DeepCopy()

	convertedObjInternal := &rbac.RoleBinding{}
	if err := rbacv1helpers.Convert_v1_RoleBinding_To_rbac_RoleBinding(objCopy, convertedObjInternal, nil); err != nil {
		return nil, err
	}

	convertedObj := &authorizationapi.RoleBinding{}
	if err := rbacconversion.Convert_rbac_RoleBinding_To_authorization_RoleBinding(convertedObjInternal, convertedObj, nil); err != nil {
		return nil, err
	}

	return convertedObj, nil
}
