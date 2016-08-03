package bootstrappolicy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/serviceaccount"
)

const (
	// SecurityContextConstraintPrivileged is used as the name for the system default privileged scc.
	SecurityContextConstraintPrivileged     = "privileged"
	SecurityContextConstraintPrivilegedDesc = "privileged allows access to all privileged and host features and the ability to run as any user, any group, any fsGroup, and with any SELinux context.  WARNING: this is the most relaxed SCC and should be used only for cluster administration. Grant with caution."

	// SecurityContextConstraintRestricted is used as the name for the system default restricted scc.
	SecurityContextConstraintRestricted     = "restricted"
	SecurityContextConstraintRestrictedDesc = "restricted denies access to all host features and requires pods to be run with a UID, and SELinux context that are allocated to the namespace.  This is the most restrictive SCC."

	// SecurityContextConstraintNonRoot is used as the name for the system default non-root scc.
	SecurityContextConstraintNonRoot     = "nonroot"
	SecurityContextConstraintNonRootDesc = "nonroot provides all features of the restricted SCC but allows users to run with any non-root UID.  The user must specify the UID or it must be specified on the by the manifest of the container runtime."

	// SecurityContextConstraintHostMountAndAnyUID is used as the name for the system default host mount + any UID scc.
	SecurityContextConstraintHostMountAndAnyUID     = "hostmount-anyuid"
	SecurityContextConstraintHostMountAndAnyUIDDesc = "hostmount-anyuid provides all the features of the restricted SCC but allows host mounts and any UID by a pod.  This is primarily used by the persistent volume recycler. WARNING: this SCC allows host file system access as any UID, including UID 0.  Grant with caution."

	// SecurityContextConstraintHostNS is used as the name for the system default scc
	// that grants access to all host ns features.
	SecurityContextConstraintHostNS     = "hostaccess"
	SecurityContextConstraintHostNSDesc = "hostaccess allows access to all host namespaces but still requires pods to be run with a UID and SELinux context that are allocated to the namespace. WARNING: this SCC allows host access to namespaces, file systems, and PIDS.  It should only be used by trusted pods.  Grant with caution."

	// SecurityContextConstraintsAnyUID is used as the name for the system default scc that
	// grants access to run as any uid but is still restricted to specific SELinux contexts.
	SecurityContextConstraintsAnyUID     = "anyuid"
	SecurityContextConstraintsAnyUIDDesc = "anyuid provides all features of the restricted SCC but allows users to run with any UID and any GID.  This is the default SCC for authenticated users."

	// SecurityContextConstraintsHostNetwork is used as the name for the system default scc that
	// grants access to run with host networking and host ports but still allocates uid/gids/selinux from the
	// namespace.
	SecurityContextConstraintsHostNetwork     = "hostnetwork"
	SecurityContextConstraintsHostNetworkDesc = "hostnetwork allows using host networking and host ports but still requires pods to be run with a UID and SELinux context that are allocated to the namespace."

	// DescriptionAnnotation is the annotation used for attaching descriptions.
	DescriptionAnnotation = "kubernetes.io/description"
)

// GetBootstrapSecurityContextConstraints returns the slice of default SecurityContextConstraints
// for system bootstrapping.  This method takes additional users and groups that should be added
// to the strategies.  Use GetBoostrapSCCAccess to produce the default set of mappings.
func GetBootstrapSecurityContextConstraints(sccNameToAdditionalGroups map[string][]string, sccNameToAdditionalUsers map[string][]string) []kapi.SecurityContextConstraints {
	// define priorities here and reference them below so it is easy to see, at a glance
	// what we're setting
	var (
		// this is set to 10 to allow wiggle room for admins to set other priorities without
		// having to adjust anyUID.
		securityContextConstraintsAnyUIDPriority = int32(10)
	)

	constraints := []kapi.SecurityContextConstraints{
		// SecurityContextConstraintPrivileged allows all access for every field
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintPrivileged,
				Annotations: map[string]string{
					DescriptionAnnotation: SecurityContextConstraintPrivilegedDesc,
				},
			},
			AllowPrivilegedContainer: true,
			Volumes:                  []kapi.FSType{kapi.FSTypeAll},
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
			SeccompProfiles: []string{"*"},
		},
		// SecurityContextConstraintNonRoot does not allow host access, allocates SELinux labels
		// and allows the user to request a specific UID or provide the default in the dockerfile.
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintNonRoot,
				Annotations: map[string]string{
					DescriptionAnnotation: SecurityContextConstraintNonRootDesc,
				},
			},
			Volumes: []kapi.FSType{kapi.FSTypeEmptyDir, kapi.FSTypeSecret, kapi.FSTypeDownwardAPI, kapi.FSTypeConfigMap, kapi.FSTypePersistentVolumeClaim},
			SELinuxContext: kapi.SELinuxContextStrategyOptions{
				// This strategy requires that annotations on the namespace which will be populated
				// by the admission controller.  If namespaces are not annotated creating the strategy
				// will fail.
				Type: kapi.SELinuxStrategyMustRunAs,
			},
			RunAsUser: kapi.RunAsUserStrategyOptions{
				// This strategy requires that the user request to run as a specific UID or that
				// the docker file contain a USER directive.
				Type: kapi.RunAsUserStrategyMustRunAsNonRoot,
			},
			FSGroup: kapi.FSGroupStrategyOptions{
				Type: kapi.FSGroupStrategyRunAsAny,
			},
			SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
				Type: kapi.SupplementalGroupsStrategyRunAsAny,
			},
		},
		// SecurityContextConstraintHostMountAndAnyUID is the same as the restricted scc but allows the use of the hostPath and NFS plugins, and running as any UID.
		// Used by the PV recycler.
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintHostMountAndAnyUID,
				Annotations: map[string]string{
					DescriptionAnnotation: SecurityContextConstraintHostMountAndAnyUIDDesc,
				},
			},
			Volumes: []kapi.FSType{kapi.FSTypeHostPath, kapi.FSTypeEmptyDir, kapi.FSTypeSecret, kapi.FSTypeDownwardAPI, kapi.FSTypeConfigMap, kapi.FSTypePersistentVolumeClaim, kapi.FSTypeNFS},
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
				Type: kapi.RunAsUserStrategyRunAsAny,
			},
			FSGroup: kapi.FSGroupStrategyOptions{
				Type: kapi.FSGroupStrategyRunAsAny,
			},
			SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
				Type: kapi.SupplementalGroupsStrategyRunAsAny,
			},
		},
		// SecurityContextConstraintHostNS allows access to everything except privileged on the host
		// but still allocates UIDs and SELinux.
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintHostNS,
				Annotations: map[string]string{
					DescriptionAnnotation: SecurityContextConstraintHostNSDesc,
				},
			},
			Volumes:          []kapi.FSType{kapi.FSTypeHostPath, kapi.FSTypeEmptyDir, kapi.FSTypeSecret, kapi.FSTypeDownwardAPI, kapi.FSTypeConfigMap, kapi.FSTypePersistentVolumeClaim},
			AllowHostNetwork: true,
			AllowHostPorts:   true,
			AllowHostPID:     true,
			AllowHostIPC:     true,
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
				Type: kapi.FSGroupStrategyMustRunAs,
			},
			SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
				Type: kapi.SupplementalGroupsStrategyRunAsAny,
			},
		},
		// SecurityContextConstraintRestricted allows no host access and allocates UIDs and SELinux.
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintRestricted,
				Annotations: map[string]string{
					DescriptionAnnotation: SecurityContextConstraintRestrictedDesc,
				},
			},
			Volumes: []kapi.FSType{kapi.FSTypeEmptyDir, kapi.FSTypeSecret, kapi.FSTypeDownwardAPI, kapi.FSTypeConfigMap, kapi.FSTypePersistentVolumeClaim},
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
				Type: kapi.FSGroupStrategyMustRunAs,
			},
			SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
				Type: kapi.SupplementalGroupsStrategyRunAsAny,
			},
			// drops unsafe caps
			RequiredDropCapabilities: []kapi.Capability{"KILL", "MKNOD", "SYS_CHROOT", "SETUID", "SETGID"},
		},
		// SecurityContextConstraintsAnyUID allows no host access and allocates SELinux.
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintsAnyUID,
				Annotations: map[string]string{
					DescriptionAnnotation: SecurityContextConstraintsAnyUIDDesc,
				},
			},
			Volumes: []kapi.FSType{kapi.FSTypeEmptyDir, kapi.FSTypeSecret, kapi.FSTypeDownwardAPI, kapi.FSTypeConfigMap, kapi.FSTypePersistentVolumeClaim},
			SELinuxContext: kapi.SELinuxContextStrategyOptions{
				// This strategy requires that annotations on the namespace which will be populated
				// by the admission controller.  If namespaces are not annotated creating the strategy
				// will fail.
				Type: kapi.SELinuxStrategyMustRunAs,
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
			// prefer the anyuid SCC over ones that force a uid
			Priority: &securityContextConstraintsAnyUIDPriority,
			// drops unsafe caps
			RequiredDropCapabilities: []kapi.Capability{"MKNOD", "SYS_CHROOT"},
		},
		// SecurityContextConstraintsHostNetwork allows host network and host ports
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SecurityContextConstraintsHostNetwork,
				Annotations: map[string]string{
					DescriptionAnnotation: SecurityContextConstraintsHostNetworkDesc,
				},
			},
			AllowHostNetwork: true,
			AllowHostPorts:   true,
			Volumes:          []kapi.FSType{kapi.FSTypeEmptyDir, kapi.FSTypeSecret, kapi.FSTypeDownwardAPI, kapi.FSTypeConfigMap, kapi.FSTypePersistentVolumeClaim},
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
				Type: kapi.FSGroupStrategyMustRunAs,
			},
			SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
				Type: kapi.SupplementalGroupsStrategyMustRunAs,
			},
			// drops unsafe caps
			RequiredDropCapabilities: []kapi.Capability{"KILL", "MKNOD", "SYS_CHROOT", "SETUID", "SETGID"},
		},
	}

	// add default access
	for i, constraint := range constraints {
		if usersToAdd, ok := sccNameToAdditionalUsers[constraint.Name]; ok {
			constraints[i].Users = append(constraints[i].Users, usersToAdd...)
		}
		if groupsToAdd, ok := sccNameToAdditionalGroups[constraint.Name]; ok {
			constraints[i].Groups = append(constraints[i].Groups, groupsToAdd...)
		}
	}
	return constraints
}

// GetBoostrapSCCAccess provides the default set of access that should be passed to GetBootstrapSecurityContextConstraints.
func GetBoostrapSCCAccess(infraNamespace string) (map[string][]string, map[string][]string) {
	groups := map[string][]string{
		SecurityContextConstraintPrivileged: {ClusterAdminGroup, NodesGroup},
		SecurityContextConstraintsAnyUID:    {ClusterAdminGroup},
		SecurityContextConstraintRestricted: {AuthenticatedGroup},
	}

	buildControllerUsername := serviceaccount.MakeUsername(infraNamespace, InfraBuildControllerServiceAccountName)
	pvRecyclerControllerUsername := serviceaccount.MakeUsername(infraNamespace, InfraPersistentVolumeRecyclerControllerServiceAccountName)
	users := map[string][]string{
		SecurityContextConstraintPrivileged:         {buildControllerUsername},
		SecurityContextConstraintHostMountAndAnyUID: {pvRecyclerControllerUsername},
	}
	return groups, users
}
