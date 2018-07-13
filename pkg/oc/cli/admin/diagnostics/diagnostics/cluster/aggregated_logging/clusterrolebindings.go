package aggregated_logging

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/apis/rbac"
)

const clusterReaderRoleBindingRoleName = "cluster-reader"

var clusterReaderRoleBindingNames = sets.NewString(fluentdServiceAccountName)

const clusterReaderUnboundServiceAccount = `
The ServiceAccount '%[1]s' is not a cluster-reader in the '%[2]s' project.  This
is required to enable Fluentd to look up pod metadata for the logs it gathers.
As a user with a cluster-admin role, you can grant the permissions by running
the following:

  $ oc adm policy add-cluster-role-to-user cluster-reader system:serviceaccount:%[2]s:%[1]s
`

func checkClusterRoleBindings(r diagnosticReporter, adapter clusterRoleBindingsAdapter, project string) {
	r.Debug("AGL0600", "Checking ClusterRoleBindings...")
	crbs, err := adapter.listClusterRoleBindings()
	if err != nil {
		r.Error("AGL0605", err, fmt.Sprintf("There was an error while trying to retrieve the ClusterRoleBindings for the logging stack: %s", err))
		return
	}
	boundServiceAccounts := sets.NewString()
	for _, crb := range crbs.Items {
		if crb.RoleRef.Name != clusterReaderRoleBindingRoleName {
			continue
		}
		for _, subject := range crb.Subjects {
			if subject.Kind == rbac.ServiceAccountKind && subject.Namespace == project {
				boundServiceAccounts.Insert(subject.Name)
			}
		}
	}
	for _, name := range clusterReaderRoleBindingNames.List() {
		if !boundServiceAccounts.Has(name) {
			r.Error("AGL0610", nil, fmt.Sprintf(clusterReaderUnboundServiceAccount, name, project))
		}
	}
}
