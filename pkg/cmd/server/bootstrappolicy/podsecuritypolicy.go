package bootstrappolicy

import (
	kapi "k8s.io/kubernetes/pkg/api"

	pspapi "github.com/openshift/origin/pkg/security/policy/api"
)

const (
	// PodSecurityPolicyPrivileged is used as the name for the system default privileged policy.
	PodSecurityPolicyPrivileged = "privileged"
	// PodSecurityPolicyRestricted is used as the name for the system default restricted policy.
	PodSecurityPolicyRestricted = "restricted"
)

// GetBootstrapSecurityContextConstraints returns the slice of default SecurityContextConstraints
// for system bootstrapping.
func GetBootstrapPodSecurityPolicy(buildControllerUsername string) []pspapi.PodSecurityPolicy {
	policies := []pspapi.PodSecurityPolicy{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: PodSecurityPolicyPrivileged,
			},
			Spec: pspapi.PodSecurityPolicySpec{
				Privileged: true,
				Volumes: pspapi.VolumeSecurityPolicy{
					HostPath:              true,
					EmptyDir:              true,
					GCEPersistentDisk:     true,
					AWSElasticBlockStore:  true,
					GitRepo:               true,
					Secret:                true,
					NFS:                   true,
					ISCSI:                 true,
					Glusterfs:             true,
					PersistentVolumeClaim: true,
					RBD:         true,
					Cinder:      true,
					CephFS:      true,
					DownwardAPI: true,
					FC:          true,
				},
				HostNetwork: true,
				HostPorts: []pspapi.HostPortRange{
					{
						Start: 1,
						End:   65535,
					},
				},
				SELinuxContext: pspapi.SELinuxContextStrategyOptions{
					Type: pspapi.SELinuxStrategyRunAsAny,
				},
				RunAsUser: pspapi.RunAsUserStrategyOptions{
					Type: pspapi.RunAsUserStrategyRunAsAny,
				},
				Users:  []string{buildControllerUsername},
				Groups: []string{ClusterAdminGroup, NodesGroup},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: PodSecurityPolicyRestricted,
			},
			Spec: pspapi.PodSecurityPolicySpec{
				SELinuxContext: pspapi.SELinuxContextStrategyOptions{
					// This strategy requires that annotations on the namespace which will be populated
					// by the admission controller.  If namespaces are not annotated creating the strategy
					// will fail.
					Type: pspapi.SELinuxStrategyMustRunAs,
				},
				RunAsUser: pspapi.RunAsUserStrategyOptions{
					// This strategy requires that annotations on the namespace which will be populated
					// by the admission controller.  If namespaces are not annotated creating the strategy
					// will fail.
					Type: pspapi.RunAsUserStrategyMustRunAsRange,
				},
				Groups: []string{AuthenticatedGroup},
			},
		},
	}
	return policies
}
