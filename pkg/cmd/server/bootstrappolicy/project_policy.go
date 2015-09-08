package bootstrappolicy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/controller/serviceaccount"

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
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: serviceaccount.MakeNamespaceGroupName(namespace)}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      ImageBuilderRoleBindingName,
				Namespace: namespace,
			},
			RoleRef: kapi.ObjectReference{
				Name: ImageBuilderRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.ServiceAccountKind, Name: BuilderServiceAccountName}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      DeployerRoleBindingName,
				Namespace: namespace,
			},
			RoleRef: kapi.ObjectReference{
				Name: DeployerRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.ServiceAccountKind, Name: DeployerServiceAccountName}},
		},
	}
}
