package resourceapply

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacclientv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyClusterRole merges objectmeta, requires rules, aggregation rules are not allowed for now.
func ApplyClusterRole(client rbacclientv1.ClusterRolesGetter, recorder events.Recorder, required *rbacv1.ClusterRole) (*rbacv1.ClusterRole, bool, error) {
	if required.AggregationRule != nil && len(required.AggregationRule.ClusterRoleSelectors) != 0 {
		return nil, false, fmt.Errorf("cannot create an aggregated cluster role")
	}

	existing, err := client.ClusterRoles().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.ClusterRoles().Create(required)
		reportCreateEvent(recorder, required, err)
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
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyClusterRoleBinding merges objectmeta, requires subjects and role refs
// TODO on non-matching roleref, delete and recreate
func ApplyClusterRoleBinding(client rbacclientv1.ClusterRoleBindingsGetter, recorder events.Recorder, required *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, bool, error) {
	existing, err := client.ClusterRoleBindings().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.ClusterRoleBindings().Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existing.Subjects, required.Subjects) &&
		deepEqualRoleRef(existing.RoleRef, required.RoleRef)
	if contentSame && !*modified {
		return existing, false, nil
	}
	existing.Subjects = required.Subjects
	existing.RoleRef = required.RoleRef

	actual, err := client.ClusterRoleBindings().Update(existing)
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyRole merges objectmeta, requires rules
func ApplyRole(client rbacclientv1.RolesGetter, recorder events.Recorder, required *rbacv1.Role) (*rbacv1.Role, bool, error) {
	existing, err := client.Roles(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Roles(required.Namespace).Create(required)
		reportCreateEvent(recorder, required, err)
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
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyRoleBinding merges objectmeta, requires subjects and role refs
// TODO on non-matching roleref, delete and recreate
func ApplyRoleBinding(client rbacclientv1.RoleBindingsGetter, recorder events.Recorder, required *rbacv1.RoleBinding) (*rbacv1.RoleBinding, bool, error) {
	existing, err := client.RoleBindings(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.RoleBindings(required.Namespace).Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existing.Subjects, required.Subjects) &&
		deepEqualRoleRef(existing.RoleRef, required.RoleRef)
	if contentSame && !*modified {
		return existing, false, nil
	}
	existing.Subjects = required.Subjects
	existing.RoleRef = required.RoleRef

	actual, err := client.RoleBindings(required.Namespace).Update(existing)
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// deepEqualRoleRef compares RoleRefs with knowledge of defaulting and comparison rules that "don't matter".
func deepEqualRoleRef(lhs, rhs rbacv1.RoleRef) bool {
	currRHS := &rhs
	// this is a default value that can be set and doesn't change the meaning of rolebinding or clusterrolebinding
	if len(currRHS.APIGroup) == 0 && lhs.APIGroup == "rbac.authorization.k8s.io" {
		// copy the rhs so we can mutate this field
		currRHS = currRHS.DeepCopy()
		currRHS.APIGroup = lhs.APIGroup
	}
	if !equality.Semantic.DeepEqual(&lhs, currRHS) {
		return false
	}

	return true
}
