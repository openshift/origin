package bootstrappolicy

import (
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/kubernetes/pkg/apis/rbac"
)

func GetBootstrapServiceAccountProjectRoleBindings(namespace string) []rbac.RoleBinding {
	return []rbac.RoleBinding{
		newOriginRoleBindingForClusterRole(ImagePullerRoleBindingName, ImagePullerRoleName, namespace).
			Groups(serviceaccount.MakeNamespaceGroupName(namespace)).
			BindingOrDie(),
		newOriginRoleBindingForClusterRole(ImageBuilderRoleBindingName, ImageBuilderRoleName, namespace).
			SAs(namespace, BuilderServiceAccountName).
			BindingOrDie(),
		newOriginRoleBindingForClusterRole(DeployerRoleBindingName, DeployerRoleName, namespace).
			SAs(namespace, DeployerServiceAccountName).
			BindingOrDie(),
	}
}
