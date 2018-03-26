package resourcemerge

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

// TODO do something real like reconciliation
func EnsureClusterRoleBinding(modified *bool, existing *rbacv1.ClusterRoleBinding, required rbacv1.ClusterRoleBinding) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	if !equality.Semantic.DeepEqual(existing.Subjects, required.Subjects) {
		*modified = true
		existing.Subjects = required.Subjects
	}
	if !equality.Semantic.DeepEqual(existing.RoleRef, required.RoleRef) {
		*modified = true
		existing.RoleRef = required.RoleRef
	}
}
