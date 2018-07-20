package resourceapply

import (
	"fmt"

	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacclientv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
)

// ApplyClusterRole merges objectmeta, requires rules, aggregation rules are not allowed for now.
func ApplyClusterRole(client rbacclientv1.ClusterRolesGetter, required *rbacv1.ClusterRole) (*rbacv1.ClusterRole, bool, error) {
	if required.AggregationRule != nil && len(required.AggregationRule.ClusterRoleSelectors) != 0 {
		return nil, false, fmt.Errorf("cannot create an aggregated cluster role")
	}

	existing, err := client.ClusterRoles().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.ClusterRoles().Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existing.Rules, required.Rules)
	if contentSame && !*modified {
		return existing, false, nil
	}
	existing.Rules = required.Rules
	existing.AggregationRule = nil

	actual, err := client.ClusterRoles().Update(existing)
	return actual, true, err
}

// ApplyClusterRoleBinding merges objectmeta, requires subjects and role refs
// TODO on non-matching roleref, delete and recreate
func ApplyClusterRoleBinding(client rbacclientv1.ClusterRoleBindingsGetter, required *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, bool, error) {
	existing, err := client.ClusterRoleBindings().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.ClusterRoleBindings().Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existing.Subjects, required.Subjects) &&
		equality.Semantic.DeepEqual(existing.RoleRef, required.RoleRef)
	if contentSame && !*modified {
		return existing, false, nil
	}
	existing.Subjects = required.Subjects
	existing.RoleRef = required.RoleRef

	actual, err := client.ClusterRoleBindings().Update(existing)
	return actual, true, err
}

// ApplyRole merges objectmeta, requires rules
func ApplyRole(client rbacclientv1.RolesGetter, required *rbacv1.Role) (*rbacv1.Role, bool, error) {
	existing, err := client.Roles(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Roles(required.Namespace).Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existing.Rules, required.Rules)
	if contentSame && !*modified {
		return existing, false, nil
	}
	existing.Rules = required.Rules

	actual, err := client.Roles(required.Namespace).Update(existing)
	return actual, true, err
}

// ApplyRoleBinding merges objectmeta, requires subjects and role refs
// TODO on non-matching roleref, delete and recreate
func ApplyRoleBinding(client rbacclientv1.RoleBindingsGetter, required *rbacv1.RoleBinding) (*rbacv1.RoleBinding, bool, error) {
	existing, err := client.RoleBindings(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.RoleBindings(required.Namespace).Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existing.Subjects, required.Subjects) &&
		equality.Semantic.DeepEqual(existing.RoleRef, required.RoleRef)
	if contentSame && !*modified {
		return existing, false, nil
	}
	existing.Subjects = required.Subjects
	existing.RoleRef = required.RoleRef

	actual, err := client.RoleBindings(required.Namespace).Update(existing)
	return actual, true, err
}
