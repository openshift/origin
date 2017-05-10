package bootstrappolicy

import (
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
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
			ObjectMeta: metav1.ObjectMeta{Name: name},
		},
	)
}

// GetDeadClusterRoles returns cluster roles which should no longer have any permissions.
// These are enumerated so that a reconcile that tightens permissions will properly.
func GetDeadClusterRoles() []authorizationapi.ClusterRole {
	return deadClusterRoles
}

func init() {
	addDeadClusterRole("system:replication-controller")
}
