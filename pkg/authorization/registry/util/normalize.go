package util

import (
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/rbac"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// ConvertToRBACClusterRole performs the conversion and guarantees the returned object is safe to mutate.
func ConvertToRBACClusterRole(originClusterRole *authorizationapi.ClusterRole) (*rbac.ClusterRole, error) {
	// convert the origin role to an rbac role
	equivalentClusterRole, err := ClusterRoleToRBAC(originClusterRole)
	if err != nil {
		return nil, err
	}

	// normalize rules before persisting so RBAC's case sensitive authorizer will work
	normalizePolicyRules(equivalentClusterRole.Rules)

	// resource version cannot be set during creation
	equivalentClusterRole.ResourceVersion = ""

	return equivalentClusterRole, nil
}

// PrepareForUpdateClusterRole compares newClusterRole with existingClusterRole to determine if an update is required.
// newClusterRole must be safe to modify as it is mutated during the comparison which must ignore fields that will never match.
// Returns true if an update is required, in which case newClusterRole should be passed to Update.
func PrepareForUpdateClusterRole(newClusterRole, existingClusterRole *rbac.ClusterRole) bool {
	// if we might need to update, we need to stomp fields that are never going to match like uid and creation time
	newClusterRole.ObjectMeta = prepareForUpdateObjectMeta(newClusterRole.ObjectMeta, existingClusterRole.ObjectMeta)

	// determine if they are not equal
	return !apiequality.Semantic.DeepEqual(newClusterRole, existingClusterRole)
}

// ConvertToRBACClusterRoleBinding performs the conversion and guarantees the returned object is safe to mutate.
func ConvertToRBACClusterRoleBinding(originClusterRoleBinding *authorizationapi.ClusterRoleBinding) (*rbac.ClusterRoleBinding, error) {
	// convert the origin roleBinding to an rbac roleBinding
	equivalentClusterRoleBinding, err := ClusterRoleBindingToRBAC(originClusterRoleBinding)
	if err != nil {
		return nil, err
	}

	// resource version cannot be set during creation
	equivalentClusterRoleBinding.ResourceVersion = ""

	return equivalentClusterRoleBinding, nil
}

// PrepareForUpdateClusterRoleBinding compares newClusterRoleBinding with existingClusterRoleBinding to determine if an update is required.
// newClusterRoleBinding must be safe to modify as it is mutated during the comparison which must ignore fields that will never match.
// Returns true if an update is required, in which case newClusterRoleBinding should be passed to Update.
func PrepareForUpdateClusterRoleBinding(newClusterRoleBinding, existingClusterRoleBinding *rbac.ClusterRoleBinding) bool {
	// if we might need to update, we need to stomp fields that are never going to match like uid and creation time
	newClusterRoleBinding.ObjectMeta = prepareForUpdateObjectMeta(newClusterRoleBinding.ObjectMeta, existingClusterRoleBinding.ObjectMeta)

	// determine if they are not equal
	return !apiequality.Semantic.DeepEqual(newClusterRoleBinding, existingClusterRoleBinding)
}

// ConvertToRBACRole performs the conversion and guarantees the returned object is safe to mutate.
func ConvertToRBACRole(originRole *authorizationapi.Role) (*rbac.Role, error) {
	// convert the origin role to an rbac role
	equivalentRole, err := RoleToRBAC(originRole)
	if err != nil {
		return nil, err
	}

	// normalize rules before persisting so RBAC's case sensitive authorizer will work
	normalizePolicyRules(equivalentRole.Rules)

	// resource version cannot be set during creation
	equivalentRole.ResourceVersion = ""

	return equivalentRole, nil
}

// PrepareForUpdateRole compares newRole with existingRole to determine if an update is required.
// newRole must be safe to modify as it is mutated during the comparison which must ignore fields that will never match.
// Returns true if an update is required, in which case newRole should be passed to Update.
func PrepareForUpdateRole(newRole, existingRole *rbac.Role) bool {
	// if we might need to update, we need to stomp fields that are never going to match like uid and creation time
	newRole.ObjectMeta = prepareForUpdateObjectMeta(newRole.ObjectMeta, existingRole.ObjectMeta)

	// determine if they are not equal
	return !apiequality.Semantic.DeepEqual(newRole, existingRole)
}

// ConvertToRBACRoleBinding performs the conversion and guarantees the returned object is safe to mutate.
func ConvertToRBACRoleBinding(originRoleBinding *authorizationapi.RoleBinding) (*rbac.RoleBinding, error) {
	// convert the origin roleBinding to an rbac roleBinding
	equivalentRoleBinding, err := RoleBindingToRBAC(originRoleBinding)
	if err != nil {
		return nil, err
	}

	// resource version cannot be set during creation
	equivalentRoleBinding.ResourceVersion = ""

	return equivalentRoleBinding, nil
}

// PrepareForUpdateRoleBinding compares newRoleBinding with existingRoleBinding to determine if an update is required.
// newRoleBinding must be safe to modify as it is mutated during the comparison which must ignore fields that will never match.
// Returns true if an update is required, in which case newRoleBinding should be passed to Update.
func PrepareForUpdateRoleBinding(newRoleBinding, existingRoleBinding *rbac.RoleBinding) bool {
	// if we might need to update, we need to stomp fields that are never going to match like uid and creation time
	newRoleBinding.ObjectMeta = prepareForUpdateObjectMeta(newRoleBinding.ObjectMeta, existingRoleBinding.ObjectMeta)

	// determine if they are not equal
	return !apiequality.Semantic.DeepEqual(newRoleBinding, existingRoleBinding)
}

// We check if we need to update by comparing the new and existing object meta.
// Thus we need to stomp fields that are never going to match.
func prepareForUpdateObjectMeta(newObjectMeta, existingObjectMeta v1.ObjectMeta) v1.ObjectMeta {
	newObjectMeta.SelfLink = existingObjectMeta.SelfLink
	newObjectMeta.UID = existingObjectMeta.UID
	newObjectMeta.ResourceVersion = existingObjectMeta.ResourceVersion
	newObjectMeta.CreationTimestamp = existingObjectMeta.CreationTimestamp
	return newObjectMeta
}

// normalizePolicyRules mutates the given rules and lowercases verbs, resources and API groups.
// Origin's authorizer is case-insensitive to these fields but Kubernetes RBAC is not.  Thus normalizing
// the Origin rules before persisting them as RBAC will allow authorization to continue to work.
func normalizePolicyRules(rules []rbac.PolicyRule) {
	for i := range rules {
		rule := &rules[i]
		toLowerSlice(rule.Verbs)
		toLowerSlice(rule.APIGroups)
		toLowerSlice(rule.Resources)
	}
}

func toLowerSlice(slice []string) {
	for i, item := range slice {
		slice[i] = strings.ToLower(item)
	}
}
