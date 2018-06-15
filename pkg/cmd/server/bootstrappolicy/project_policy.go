package bootstrappolicy

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacv1conversions "k8s.io/kubernetes/pkg/apis/rbac/v1"

	oapi "github.com/openshift/origin/pkg/api"
)

func GetBootstrapServiceAccountProjectRoleBindings(namespace string) []rbac.RoleBinding {
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

	return []rbac.RoleBinding{
		imagePuller,
		imageBuilder,
		deployer,
	}
}

func GetBootstrapServiceAccountProjectV1RoleBindings(namespace string) []rbacv1.RoleBinding {
	ret := []rbacv1.RoleBinding{}

	internalRoleBindings := GetBootstrapServiceAccountProjectRoleBindings(namespace)
	for i := range internalRoleBindings {
		out := &rbacv1.RoleBinding{}
		if err := rbacv1conversions.Convert_rbac_RoleBinding_To_v1_RoleBinding(&internalRoleBindings[i], out, nil); err != nil {
			// coding error
			panic(err)
		}
		ret = append(ret, *out)
	}

	return ret
}

func GetBootstrapServiceAccountProjectRoleBindingNames() sets.String {
	names := sets.NewString()

	for _, roleBinding := range GetBootstrapServiceAccountProjectRoleBindings("default") {
		names.Insert(roleBinding.Name)
	}

	return names
}
