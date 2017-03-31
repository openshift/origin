package bootstrappolicy

import (
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	rbac "k8s.io/kubernetes/pkg/apis/rbac"

	"github.com/openshift/origin/pkg/controller"

	"github.com/golang/glog"
)

const saRolePrefix = "system:controller:"

var (
	// controllerClusterRoles is a slice of cluster roles used for controllers
	controllerClusterRoles = []rbac.ClusterRole{}
	// controllerClusterRoleBindings is a slice of cluster role bindings used for controllers
	controllerClusterRoleBindings = []rbac.ClusterRoleBinding{}
)

func addControllerRole(role rbac.ClusterRole) {
	if !strings.HasPrefix(role.Name, saRolePrefix) {
		glog.Fatalf(`role %q must start with %q`, role.Name, saRolePrefix)
	}

	for _, existingRole := range controllerClusterRoles {
		if role.Name == existingRole.Name {
			glog.Fatalf("role %q was already registered", role.Name)
		}
	}

	if role.Annotations == nil {
		role.Annotations = map[string]string{}
	}
	role.Annotations[roleSystemOnly] = roleIsSystemOnly

	controllerClusterRoles = append(controllerClusterRoles, role)

	controllerClusterRoleBindings = append(controllerClusterRoleBindings,
		rbac.NewClusterBinding(role.Name).SAs(DefaultOpenShiftInfraNamespace, role.Name[len(saRolePrefix):]).BindingOrDie(),
	)
}

func init() {
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: kapi.ObjectMeta{
			Name: saRolePrefix + controller.DockercfgTokenDeletedControllerName,
		},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("list", "watch", "delete").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
		},
	})
}

// GetBootstrapControllerClusterRoles returns the cluster roles used by controllers
func GetBootstrapControllerClusterRoles() []rbac.ClusterRole {
	return controllerClusterRoles
}

// GetBootstrapControllerClusterRoleBindings returns the cluster role bindings used by controllers
func GetBootstrapControllerClusterRoleBindings() []rbac.ClusterRoleBinding {
	return controllerClusterRoleBindings
}
