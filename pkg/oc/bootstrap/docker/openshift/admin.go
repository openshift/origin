package openshift

import (
	"io"

	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	"github.com/openshift/origin/pkg/oc/admin/policy"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
)

const (
	DefaultNamespace = "default"
)

func AddClusterRole(authorizationClient authorizationtypedclient.ClusterRoleBindingsGetter, role, user string) error {
	clusterRoleBindingAccessor := policy.NewClusterRoleBindingAccessor(authorizationClient)
	addClusterReaderRole := policy.RoleModificationOptions{
		RoleName:            role,
		RoleBindingAccessor: clusterRoleBindingAccessor,
		Users:               []string{user},
	}
	return addClusterReaderRole.AddRole()
}

func AddSCCToServiceAccount(securityClient securitytypedclient.SecurityContextConstraintsGetter, scc, sa, namespace string, out io.Writer) error {
	modifySCC := policy.SCCModificationOptions{
		SCCName:      scc,
		SCCInterface: securityClient.SecurityContextConstraints(),
		Subjects: []kapi.ObjectReference{
			{
				Namespace: namespace,
				Name:      sa,
				Kind:      "ServiceAccount",
			},
		},

		Out: out,
	}
	return modifySCC.AddSCC()
}
