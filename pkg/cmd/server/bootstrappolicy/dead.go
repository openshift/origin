package bootstrappolicy

import (
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

var (
	deadClusterRoles = []authorizationapi.ClusterRole{}
)

func addDeadClusterRole(name string) {
	for _, existingRole := range deadClusterRoles {
		if name == existingRole.Name {
			glog.Fatalf("role %q was already registered", name)
		}
	}

	deadClusterRoles = append(deadClusterRoles,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
		},
	)
}

// GetDeadClusterRoles returns cluster roles which should no longer have any permissions.
// These are enumerated so that a reconcile that tightens permissions will properly.
func GetDeadClusterRoles() []authorizationapi.ClusterRole {
	return deadClusterRoles
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
}
