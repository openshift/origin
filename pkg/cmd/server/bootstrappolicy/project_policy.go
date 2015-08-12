package bootstrappolicy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func GetBootstrapServiceAccountProjectRoleBindings(namespace string) []authorizationapi.RoleBinding {
	return []authorizationapi.RoleBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      ImagePullerRoleBindingName,
				Namespace: namespace,
			},
			RoleRef: kapi.ObjectReference{
				Name: ImagePullerRoleName,
			},
			Groups: util.NewStringSet(serviceaccount.MakeNamespaceGroupName(namespace)),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      ImageBuilderRoleBindingName,
				Namespace: namespace,
			},
			RoleRef: kapi.ObjectReference{
				Name: ImageBuilderRoleName,
			},
			Users: util.NewStringSet(serviceaccount.MakeUsername(namespace, BuilderServiceAccountName)),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      DeployerRoleBindingName,
				Namespace: namespace,
			},
			RoleRef: kapi.ObjectReference{
				Name: DeployerRoleName,
			},
			Users: util.NewStringSet(serviceaccount.MakeUsername(namespace, DeployerServiceAccountName)),
		},
	}
}
