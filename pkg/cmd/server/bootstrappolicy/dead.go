package bootstrappolicy

import (
	"github.com/golang/glog"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	deadClusterRoles = []rbacv1.ClusterRole{}

	deadClusterRoleBindings = []rbacv1.ClusterRoleBinding{}
)

func addDeadClusterRole(name string) {
	for _, existingRole := range deadClusterRoles {
		if name == existingRole.Name {
			glog.Fatalf("role %q was already registered", name)
		}
	}

	deadClusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	addDefaultMetadata(&deadClusterRole)
	deadClusterRoles = append(deadClusterRoles, deadClusterRole)
}

func addDeadClusterRoleBinding(name, roleName string) {
	for _, existing := range deadClusterRoleBindings {
		if name == existing.Name {
			glog.Fatalf("%q was already registered", name)
		}
	}

	deadClusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: roleName},
	}
	addDefaultMetadata(&deadClusterRoleBinding)
	deadClusterRoleBindings = append(deadClusterRoleBindings, deadClusterRoleBinding)
}

// GetDeadClusterRoles returns cluster roles which should no longer have any permissions.
// These are enumerated so that a reconcile that tightens permissions will properly.
func GetDeadClusterRoles() []rbacv1.ClusterRole {
	return deadClusterRoles
}

// GetDeadClusterRoleBindings returns cluster role bindings which should no longer have any subjects.
// These are enumerated so that a reconcile that tightens permissions will properly remove them.
func GetDeadClusterRoleBindings() []rbacv1.ClusterRoleBinding {
	return deadClusterRoleBindings
}

func init() {
	// these were replaced by kube controller roles
	addDeadClusterRole("system:replication-controller")
	addDeadClusterRole("system:endpoint-controller")
	addDeadClusterRole("system:replicaset-controller")
	addDeadClusterRole("system:garbage-collector-controller")
	addDeadClusterRole("system:job-controller")
	addDeadClusterRole("system:hpa-controller")
	addDeadClusterRole("system:daemonset-controller")
	addDeadClusterRole("system:disruption-controller")
	addDeadClusterRole("system:namespace-controller")
	addDeadClusterRole("system:gc-controller")
	addDeadClusterRole("system:certificate-signing-controller")
	addDeadClusterRole("system:statefulset-controller")

	// these were moved under system:openshift:controller:*
	addDeadClusterRole("system:build-controller")
	addDeadClusterRole("system:deploymentconfig-controller")
	addDeadClusterRole("system:deployment-controller")

	// this was replaced by the node authorizer
	addDeadClusterRoleBinding("system:nodes", "system:node")

	// this was replaced by an openshift specific role and binding
	addDeadClusterRoleBinding("system:discovery-binding", "system:discovery")
}
