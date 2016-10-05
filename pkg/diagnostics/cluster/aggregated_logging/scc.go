package aggregated_logging

import (
	"fmt"

	"k8s.io/kubernetes/pkg/util/sets"
)

const sccPrivilegedName = "privileged"

var sccPrivilegedNames = sets.NewString(fluentdServiceAccountName)

const sccPrivilegedUnboundServiceAccount = `
The ServiceAccount '%[1]s' does not have a privileged SecurityContextConstraint for project '%[2]s'.  As a
user with a cluster-admin role, you can grant the permissions by running
the following:

  oadm policy add-scc-to-user privileged system:serviceaccount:%[2]s:%[1]s
`

func checkSccs(r diagnosticReporter, adapter sccAdapter, project string) {
	r.Debug("AGL0700", "Checking SecurityContextConstraints...")
	scc, err := adapter.getScc(sccPrivilegedName)
	if err != nil {
		r.Error("AGL0705", err, fmt.Sprintf("There was an error while trying to retrieve the SecurityContextConstraints for the logging stack: %s", err))
		return
	}
	privilegedUsers := sets.NewString()
	for _, user := range scc.Users {
		privilegedUsers.Insert(user)
	}
	for _, name := range sccPrivilegedNames.List() {
		if !privilegedUsers.Has(fmt.Sprintf("system:serviceaccount:%s:%s", project, name)) {
			r.Error("AGL0710", nil, fmt.Sprintf(sccPrivilegedUnboundServiceAccount, name, project))
		}
	}
}
