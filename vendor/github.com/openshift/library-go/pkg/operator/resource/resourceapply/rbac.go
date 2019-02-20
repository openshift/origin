package resourceapply

import (
	"fmt"

	"github.com/golang/glog"

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
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existingCopy.Rules, required.Rules)
	if contentSame && !*modified {
		return existingCopy, false, nil
	}

	existingCopy.Rules = required.Rules
	existingCopy.AggregationRule = nil

	if glog.V(4) {
		glog.Infof("ClusterRole %q changes: %v", required.Name, JSONPatch(existing, existingCopy))
	}

	actual, err := client.ClusterRoles().Update(existingCopy)
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
	existingCopy := existing.DeepCopy()

	// Enforce apiGroup field
	existingCopy.RoleRef.APIGroup = rbacv1.GroupName

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existingCopy.Subjects, required.Subjects) &&
		deepEqualRoleRef(existingCopy.RoleRef, required.RoleRef)
	if contentSame && !*modified {
		return existingCopy, false, nil
	}

	existingCopy.Subjects = required.Subjects
	existingCopy.RoleRef = required.RoleRef

	if glog.V(4) {
		glog.Infof("ClusterRoleBinding %q changes: %v", required.Name, JSONPatch(existing, existingCopy))
	}

	actual, err := client.ClusterRoleBindings().Update(existingCopy)
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
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existingCopy.Rules, required.Rules)
	if contentSame && !*modified {
		return existingCopy, false, nil
	}
	existingCopy.Rules = required.Rules

	if glog.V(4) {
		glog.Infof("Role %q changes: %v", required.Namespace+"/"+required.Name, JSONPatch(existing, existingCopy))
	}
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
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existingCopy.Subjects, required.Subjects) &&
		deepEqualRoleRef(existingCopy.RoleRef, required.RoleRef)
	if contentSame && !*modified {
		return existingCopy, false, nil
	}

	existingCopy.Subjects = required.Subjects
	existingCopy.RoleRef = required.RoleRef

	if glog.V(4) {
		glog.Infof("RoleBinding %q changes: %v", required.Namespace+"/"+required.Name, JSONPatch(existing, existingCopy))
	}

	actual, err := client.RoleBindings(required.Namespace).Update(existingCopy)
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
