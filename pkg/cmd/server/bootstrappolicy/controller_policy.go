package bootstrappolicy

import (
	"strings"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbac "k8s.io/kubernetes/pkg/apis/rbac"
)

const saRolePrefix = "system:openshift:controller:"

var (
	// controllerRoles is a slice of roles used for controllers
	controllerRoles = []rbac.ClusterRole{}
	// controllerRoleBindings is a slice of roles used for controllers
	controllerRoleBindings = []rbac.ClusterRoleBinding{}
)

func addControllerRole(role rbac.ClusterRole) {
	if !strings.HasPrefix(role.Name, saRolePrefix) {
		glog.Fatalf(`role %q must start with %q`, role.Name, saRolePrefix)
	}

	for _, existingRole := range controllerRoles {
		if role.Name == existingRole.Name {
			glog.Fatalf("role %q was already registered", role.Name)
		}
	}

	if role.Annotations == nil {
		role.Annotations = map[string]string{}
	}
	role.Annotations[roleSystemOnly] = roleIsSystemOnly

	controllerRoles = append(controllerRoles, role)

	controllerRoleBindings = append(controllerRoleBindings,
		rbac.NewClusterBinding(role.Name).SAs(DefaultOpenShiftInfraNamespace, role.Name[len(saRolePrefix):]).BindingOrDie())
}

func eventsRule() rbac.PolicyRule {
	return rbac.NewRule("create", "update", "patch").Groups(kapiGroup).Resources("events").RuleOrDie()
}

func init() {
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraBuildControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "list", "watch", "update", "delete").Groups(buildGroup, legacyBuildGroup).Resources("builds").RuleOrDie(),
			rbac.NewRule("get").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs").RuleOrDie(),
			rbac.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("builds/docker", "builds/source", "builds/custom", "builds/jenkinspipeline").RuleOrDie(),
			rbac.NewRule("get").Groups(imageGroup, legacyImageGroup).Resources("imagestreams").RuleOrDie(),
			rbac.NewRule("get", "list", "create", "delete").Groups(kapiGroup).Resources("pods").RuleOrDie(),
			rbac.NewRule("get").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
			eventsRule(),
		},
	})

	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraDeployerControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("create", "get", "list", "watch", "update", "patch", "delete").Groups(kapiGroup).Resources("pods").RuleOrDie(),
			rbac.NewRule("create", "get", "list", "watch", "update", "delete").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			eventsRule(),
		},
	})

	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraDeploymentConfigControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("pods").RuleOrDie(),
			rbac.NewRule("create", "get", "list", "watch", "update", "delete").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			rbac.NewRule("update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/status").RuleOrDie(),
			rbac.NewRule("get", "list", "watch").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs").RuleOrDie(),
			eventsRule(),
		},
	})
}

// ControllerRoles returns the cluster roles used by controllers
func ControllerRoles() []rbac.ClusterRole {
	return controllerRoles
}

// ControllerRoleBindings returns the role bindings used by controllers
func ControllerRoleBindings() []rbac.ClusterRoleBinding {
	return controllerRoleBindings
}
