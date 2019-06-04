package defaultrolebindings

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
)

const (
	openShiftDescription = "openshift.io/description"

	ImagePullerRoleName  = "system:image-puller"
	ImageBuilderRoleName = "system:image-builder"
	DeployerRoleName     = "system:deployer"

	DeployerRoleBindingName     = DeployerRoleName + "s"
	ImagePullerRoleBindingName  = ImagePullerRoleName + "s"
	ImageBuilderRoleBindingName = ImageBuilderRoleName + "s"

	BuilderServiceAccountName  = "builder"
	DeployerServiceAccountName = "deployer"
)

func GetBootstrapServiceAccountProjectRoleBindings(namespace string) []rbacv1.RoleBinding {
	imagePuller := newOriginRoleBindingForClusterRoleWithGroup(ImagePullerRoleBindingName, ImagePullerRoleName, namespace, serviceaccount.MakeNamespaceGroupName(namespace))
	imagePuller.Annotations[openShiftDescription] = "Allows all pods in this namespace to pull images from this namespace.  It is auto-managed by a controller; remove subjects to disable."

	imageBuilder := newOriginRoleBindingForClusterRoleWithSA(ImageBuilderRoleBindingName, ImageBuilderRoleName, namespace, BuilderServiceAccountName)
	imageBuilder.Annotations[openShiftDescription] = "Allows builds in this namespace to push images to this namespace.  It is auto-managed by a controller; remove subjects to disable."

	deployer := newOriginRoleBindingForClusterRoleWithSA(DeployerRoleBindingName, DeployerRoleName, namespace, DeployerServiceAccountName)
	deployer.Annotations[openShiftDescription] = "Allows deploymentconfigs in this namespace to rollout pods in this namespace.  It is auto-managed by a controller; remove subjects to disable."

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

func newOriginRoleBindingForClusterRoleWithGroup(bindingName, roleName, namespace, group string) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        bindingName,
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.GroupKind, APIGroup: "rbac.authorization.k8s.io", Name: group},
		},
	}
}

func newOriginRoleBindingForClusterRoleWithSA(bindingName, roleName, namespace, saName string) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        bindingName,
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.ServiceAccountKind, Namespace: namespace, Name: saName},
		},
	}
}
