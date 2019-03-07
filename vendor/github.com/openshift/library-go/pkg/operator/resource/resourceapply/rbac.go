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
	requiredCopy := required.DeepCopy()

	// Enforce apiGroup fields in roleRefs
	existingCopy.RoleRef.APIGroup = rbacv1.GroupName
	for i := range existingCopy.Subjects {
		if existingCopy.Subjects[i].Kind == "User" {
			existingCopy.Subjects[i].APIGroup = rbacv1.GroupName
		}
	}

	requiredCopy.RoleRef.APIGroup = rbacv1.GroupName
	for i := range requiredCopy.Subjects {
		if existingCopy.Subjects[i].Kind == "User" {
			requiredCopy.Subjects[i].APIGroup = rbacv1.GroupName
		}
	}

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, requiredCopy.ObjectMeta)

	subjectsAreSame := equality.Semantic.DeepEqual(existingCopy.Subjects, requiredCopy.Subjects)
	roleRefIsSame := equality.Semantic.DeepEqual(existingCopy.RoleRef, requiredCopy.RoleRef)

	if subjectsAreSame && roleRefIsSame && !*modified {
		return existingCopy, false, nil
	}

	existingCopy.Subjects = requiredCopy.Subjects
	existingCopy.RoleRef = requiredCopy.RoleRef

	if glog.V(4) {
		glog.Infof("ClusterRoleBinding %q changes: %v", requiredCopy.Name, JSONPatch(existing, existingCopy))
	}

	actual, err := client.ClusterRoleBindings().Update(existingCopy)
	reportUpdateEvent(recorder, requiredCopy, err)
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
	requiredCopy := required.DeepCopy()

	// Enforce apiGroup fields in roleRefs and subjects
	existingCopy.RoleRef.APIGroup = rbacv1.GroupName
	for i := range existingCopy.Subjects {
		if existingCopy.Subjects[i].Kind == "User" {
			existingCopy.Subjects[i].APIGroup = rbacv1.GroupName
		}
	}

	requiredCopy.RoleRef.APIGroup = rbacv1.GroupName
	for i := range requiredCopy.Subjects {
		if existingCopy.Subjects[i].Kind == "User" {
			requiredCopy.Subjects[i].APIGroup = rbacv1.GroupName
		}
	}

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, requiredCopy.ObjectMeta)

	subjectsAreSame := equality.Semantic.DeepEqual(existingCopy.Subjects, requiredCopy.Subjects)
	roleRefIsSame := equality.Semantic.DeepEqual(existingCopy.RoleRef, requiredCopy.RoleRef)

	if subjectsAreSame && roleRefIsSame && !*modified {
		return existingCopy, false, nil
	}

	existingCopy.Subjects = requiredCopy.Subjects
	existingCopy.RoleRef = requiredCopy.RoleRef

	if glog.V(4) {
		glog.Infof("RoleBinding %q changes: %v", requiredCopy.Namespace+"/"+requiredCopy.Name, JSONPatch(existing, existingCopy))
	}

	actual, err := client.RoleBindings(requiredCopy.Namespace).Update(existingCopy)
	reportUpdateEvent(recorder, requiredCopy, err)
	return actual, true, err
}
