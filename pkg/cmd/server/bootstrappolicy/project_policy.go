package bootstrappolicy

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"

	oapi "github.com/openshift/origin/pkg/api"
)

func GetBootstrapServiceAccountProjectRoleBindings(namespace string) []rbacv1.RoleBinding {
	imagePuller := newOriginRoleBindingForClusterRole(ImagePullerRoleBindingName, ImagePullerRoleName, namespace).
		Groups(serviceaccount.MakeNamespaceGroupName(namespace)).
		BindingOrDie()
	if imagePuller.Annotations == nil {
		imagePuller.Annotations = map[string]string{}
	}
	imagePuller.Annotations[oapi.OpenShiftDescription] = "Allows all pods in this namespace to pull images from this namespace.  It is auto-managed by a controller; remove subjects to disable."

	imageBuilder := newOriginRoleBindingForClusterRole(ImageBuilderRoleBindingName, ImageBuilderRoleName, namespace).
		SAs(namespace, BuilderServiceAccountName).
		BindingOrDie()
	if imageBuilder.Annotations == nil {
		imageBuilder.Annotations = map[string]string{}
	}
	imageBuilder.Annotations[oapi.OpenShiftDescription] = "Allows builds in this namespace to push images to this namespace.  It is auto-managed by a controller; remove subjects to disable."

	deployer := newOriginRoleBindingForClusterRole(DeployerRoleBindingName, DeployerRoleName, namespace).
		SAs(namespace, DeployerServiceAccountName).
		BindingOrDie()
	if deployer.Annotations == nil {
		deployer.Annotations = map[string]string{}
	}
	deployer.Annotations[oapi.OpenShiftDescription] = "Allows deploymentconfigs in this namespace to rollout pods in this namespace.  It is auto-managed by a controller; remove subjects to disable."

	return []rbacv1.RoleBinding{
		imagePuller,
		imageBuilder,
		deployer,
	}
}

func GetBootstrapServiceAccountProjectRoleBindingNames() sets.String {
	names := sets.NewString()

	for _, roleBinding := range GetBootstrapServiceAccountProjectRoleBindings("default") {
		names.Insert(roleBinding.Name)
	}

	return names
}
