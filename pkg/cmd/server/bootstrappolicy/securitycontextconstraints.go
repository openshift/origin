package bootstrappolicy

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

const (
	// SecurityContextConstraintPrivileged is used as the name for the system default privileged scc.
	SecurityContextConstraintPrivileged = "privileged"
	// SecurityContextConstraintRestricted is used as the name for the system default restricted scc.
	SecurityContextConstraintRestricted = "restricted"
)

// GetBootstrapSecurityContextConstraints returns the slice of default SecurityContextConstraints
// for system bootstrapping.
func GetBootstrapSecurityContextConstraints(buildControllerUsername string) []kapi.SecurityContextConstraints {
	constraints := []kapi.SecurityContextConstraints{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintPrivileged,
			},
			AllowPrivilegedContainer: true,
			AllowHostDirVolumePlugin: true,
			AllowHostNetwork:         true,
			AllowHostPorts:           true,
			AllowHostPID:             true,
			AllowHostIPC:             true,
			SELinuxContext: kapi.SELinuxContextStrategyOptions{
				Type: kapi.SELinuxStrategyRunAsAny,
			},
			RunAsUser: kapi.RunAsUserStrategyOptions{
				Type: kapi.RunAsUserStrategyRunAsAny,
			},
			FSGroup: kapi.FSGroupStrategyOptions{
				Type: kapi.FSGroupStrategyRunAsAny,
			},
			SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
				Type: kapi.SupplementalGroupsStrategyRunAsAny,
			},
			Users:  []string{buildControllerUsername},
			Groups: []string{ClusterAdminGroup, NodesGroup},
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
			FSGroup: kapi.FSGroupStrategyOptions{
				// This strategy requires that annotations on the namespace which will be populated
				// by the admission controller.  Admission will first look for the SupplementalGroupsAnnotation
				// on the namespace and if it is unable to find that annotation it will attempt
				// to use the UIDRangeAnnotation.  If neither annotation exists then creation
				// of the SCC will fail.
				Type: kapi.FSGroupStrategyMustRunAs,
			},
			SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
				// This strategy requires that annotations on the namespace which will be populated
				// by the admission controller.  Admission will first look for the SupplementalGroupsAnnotation
				// on the namespace and if it is unable to find that annotation it will attempt
				// to use the UIDRangeAnnotation.  If neither annotation exists then creation
				// of the SCC will fail.
				Type: kapi.SupplementalGroupsStrategyMustRunAs,
			},
			Groups: []string{AuthenticatedGroup},
		},
	}
	return constraints
}
