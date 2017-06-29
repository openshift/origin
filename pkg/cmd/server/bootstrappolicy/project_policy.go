package bootstrappolicy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

func GetBootstrapServiceAccountProjectRoleBindings(namespace string) []authorizationapi.RoleBinding {
	return []authorizationapi.RoleBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ImagePullerRoleBindingName,
				Namespace: namespace,
			},
			RoleRef: kapi.ObjectReference{
				Name: ImagePullerRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: serviceaccount.MakeNamespaceGroupName(namespace)}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ImageBuilderRoleBindingName,
				Namespace: namespace,
			},
			RoleRef: kapi.ObjectReference{
				Name: ImageBuilderRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.ServiceAccountKind, Name: BuilderServiceAccountName}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
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
