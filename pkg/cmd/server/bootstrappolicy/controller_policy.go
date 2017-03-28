package bootstrappolicy

import (
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/controller"

	"github.com/golang/glog"
)

const saRolePrefix = "system:controller:"

var (
	// controllerRoles is a slice of roles used for controllers
	controllerRoles = []authorizationapi.ClusterRole{}
	// controllerRoleBindings is a slice of roles used for controllers
	controllerRoleBindings = []authorizationapi.ClusterRoleBinding{}
)

func addControllerRole(role authorizationapi.ClusterRole) {
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
		authorizationapi.ClusterRoleBinding{
			ObjectMeta: kapi.ObjectMeta{
				Name: role.Name,
			},
			RoleRef: kapi.ObjectReference{
				Name: role.Name,
			},
			Subjects: []kapi.ObjectReference{
				{
					Kind:      authorizationapi.ServiceAccountKind,
					Namespace: DefaultOpenShiftInfraNamespace,
					Name:      role.Name[len(saRolePrefix):],
				},
			},
		},
	)
}

func init() {
	addControllerRole(authorizationapi.ClusterRole{
		ObjectMeta: kapi.ObjectMeta{
			Name: saRolePrefix + controller.DockercfgTokenDeletedControllerName,
		},
		Rules: []authorizationapi.PolicyRule{
			authorizationapi.NewRule("list", "watch", "delete").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
		},
	})
}

// GetBootstrapControllerRoles returns the cluster roles used by controllers
func GetBootstrapControllerRoles() []authorizationapi.ClusterRole {
	return controllerRoles
}

// GetBootstrapControllerRoleBindings returns the role bindings used by controllers
func GetBootstrapControllerRoleBindings() []authorizationapi.ClusterRoleBinding {
	return controllerRoleBindings
}
