package bootstrappolicy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

const (
	// SecurityContextConstraintPrivileged is used as the name for the system default privileged scc.
	SecurityContextConstraintPrivileged = "privileged"
	// SecurityContextConstraintRestricted is used as the name for the system default restricted scc.
	SecurityContextConstraintRestricted = "restricted"
)

// GetBootstrapSecurityContextConstraints returns the slice of default SecurityContextConstraints
// for system bootstrapping.
func GetBootstrapSecurityContextConstraints() []kapi.SecurityContextConstraints {
	constraints := []kapi.SecurityContextConstraints{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintPrivileged,
			},
			AllowPrivilegedContainer: true,
			AllowHostDirVolumePlugin: true,
			SELinuxContext: kapi.SELinuxContextStrategyOptions{
				Type: kapi.SELinuxStrategyRunAsAny,
			},
			RunAsUser: kapi.RunAsUserStrategyOptions{
				Type: kapi.RunAsUserStrategyRunAsAny,
			},
			// TODO: AuthenticatedGroup should be removed from this list ASAP
			Groups: []string{AuthenticatedGroup, ClusterAdminGroup},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintRestricted,
			},
			SELinuxContext: kapi.SELinuxContextStrategyOptions{
				// This strategy requires that annotations on the namespace which will be populated
				// by the admission controller.  If namespaces are not annotated creating the strategy
				// will fail.
				Type: kapi.SELinuxStrategyMustRunAs,
			},
			RunAsUser: kapi.RunAsUserStrategyOptions{
				// This strategy requires that annotations on the namespace which will be populated
				// by the admission controller.  If namespaces are not annotated creating the strategy
				// will fail.
				Type: kapi.RunAsUserStrategyMustRunAsRange,
			},
		},
	}
	return constraints
}
