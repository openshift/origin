package bootstrappolicy

import (
	kapi "k8s.io/kubernetes/pkg/api"

	sccapi "github.com/openshift/origin/pkg/security/scc/api"
)

const (
	// SecurityContextConstraintPrivileged is used as the name for the system default privileged scc.
	SecurityContextConstraintPrivileged = "privileged"
	// SecurityContextConstraintRestricted is used as the name for the system default restricted scc.
	SecurityContextConstraintRestricted = "restricted"
)

// GetBootstrapSecurityContextConstraints returns the slice of default SecurityContextConstraints
// for system bootstrapping.
func GetBootstrapSecurityContextConstraints(buildControllerUsername string) []sccapi.SecurityContextConstraints {
	constraints := []sccapi.SecurityContextConstraints{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintPrivileged,
			},
			AllowPrivilegedContainer: true,
			AllowHostDirVolumePlugin: true,
			AllowHostNetwork:         true,
			AllowHostPorts:           true,
			SELinuxContext: sccapi.SELinuxContextStrategyOptions{
				Type: sccapi.SELinuxStrategyRunAsAny,
			},
			RunAsUser: sccapi.RunAsUserStrategyOptions{
				Type: sccapi.RunAsUserStrategyRunAsAny,
			},
			Users:  []string{buildControllerUsername},
			Groups: []string{ClusterAdminGroup, NodesGroup},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintRestricted,
			},
			SELinuxContext: sccapi.SELinuxContextStrategyOptions{
				// This strategy requires that annotations on the namespace which will be populated
				// by the admission controller.  If namespaces are not annotated creating the strategy
				// will fail.
				Type: sccapi.SELinuxStrategyMustRunAs,
			},
			RunAsUser: sccapi.RunAsUserStrategyOptions{
				// This strategy requires that annotations on the namespace which will be populated
				// by the admission controller.  If namespaces are not annotated creating the strategy
				// will fail.
				Type: sccapi.RunAsUserStrategyMustRunAsRange,
			},
			Groups: []string{AuthenticatedGroup},
		},
	}
	return constraints
}
