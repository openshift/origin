package scc

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	policyv1beta "k8s.io/api/policy/v1beta1"
	"k8s.io/kubernetes/pkg/security/podsecuritypolicy/seccomp"

	securityv1 "github.com/openshift/api/security/v1"
)

// SCC.AllowPrivilegedContainer is the same as PSP.Privileged.
func extractPrivileged(scc *securityv1.SecurityContextConstraints) bool {
	return scc.AllowPrivilegedContainer
}

// SCC.DefaultAddCapabilities is the same as PSP.DefaultAddCapabilities.
func extractDefaultAddCapabilities(scc *securityv1.SecurityContextConstraints) []corev1.Capability {
	return convertCapabilities(scc.DefaultAddCapabilities)
}

// SCC.RequiredDropCapabilities is the same as PSP.RequiredDropCapabilities.
func extractRequiredDropCapabilities(scc *securityv1.SecurityContextConstraints) []corev1.Capability {
	return convertCapabilities(scc.RequiredDropCapabilities)
}

// SCC.AllowedCapabilities is the same as PSP.AllowedCapabilities.
func extractAllowedCapabilities(scc *securityv1.SecurityContextConstraints) []corev1.Capability {
	return convertCapabilities(scc.AllowedCapabilities)
}

// SCC.Volumes are the same as PSP.Volumes except that OpenShift has additional "none" type.
func extractVolumes(scc *securityv1.SecurityContextConstraints) []policyv1beta.FSType {
	volumes := make([]policyv1beta.FSType, 0, len(scc.Volumes))
	for _, volume := range scc.Volumes {
		if volume == securityv1.FSTypeNone {
			return []policyv1beta.FSType{}
		}

		fsType := policyv1beta.FSType(string(volume))
		volumes = append(volumes, fsType)
	}
	return volumes
}

// SCC.AllowHostNetwork is the same as PSP.HostNetwork.
func extractHostNetwork(scc *securityv1.SecurityContextConstraints) bool {
	return scc.AllowHostNetwork
}

// SCC.AllowHostPorts doesn't exist in PSP but there is HostPorts with a list of ranges of ports.
func extractHostPorts(scc *securityv1.SecurityContextConstraints) []policyv1beta.HostPortRange {
	if scc.AllowHostPorts {
		return []policyv1beta.HostPortRange{{Min: 0, Max: 65535}}
	}
	return []policyv1beta.HostPortRange{}
}

// SCC.AllowHostPID is the same as PSP.HostPID.
func extractHostPID(scc *securityv1.SecurityContextConstraints) bool {
	return scc.AllowHostPID
}

// SCC.AllowHostIPC is the same as PSP.HostIPC.
func extractHostIPC(scc *securityv1.SecurityContextConstraints) bool {
	return scc.AllowHostIPC
}

// SCC.SELinuxContext is the same as PSP.SELinux.
func extractSELinux(scc *securityv1.SecurityContextConstraints) (policyv1beta.SELinuxStrategyOptions, error) {
	var strategy policyv1beta.SELinuxStrategy
	var selOpts *corev1.SELinuxOptions

	switch scc.SELinuxContext.Type {
	case securityv1.SELinuxStrategyRunAsAny:
		strategy = policyv1beta.SELinuxStrategyRunAsAny
	case securityv1.SELinuxStrategyMustRunAs:
		strategy = policyv1beta.SELinuxStrategyMustRunAs
	default:
		return policyv1beta.SELinuxStrategyOptions{},
			fmt.Errorf("unknown SELinuxContext.Type: %q", scc.SELinuxContext.Type)
	}

	sccSelOpts := scc.SELinuxContext.SELinuxOptions
	if sccSelOpts != nil {
		selOpts = &corev1.SELinuxOptions{
			User:  sccSelOpts.User,
			Role:  sccSelOpts.Role,
			Type:  sccSelOpts.Type,
			Level: sccSelOpts.Level,
		}
	}

	opts := policyv1beta.SELinuxStrategyOptions{
		Rule:           strategy,
		SELinuxOptions: selOpts,
	}

	return opts, nil
}

// SCC.RunAsUser is similar to PSP.RunAsUser, with the following exceptions:
// - it has an additional "MustRunAsRange" strategy
// - "MustRunAs" may have a single value or a single range (while in PSP this strategy has a list of ranges only)
// See for details: https://docs.openshift.org/3.6/architecture/additional_concepts/authorization.html#authorization-RunAsUser
func extractRunAsUser(scc *securityv1.SecurityContextConstraints) (policyv1beta.RunAsUserStrategyOptions, error) {
	var strategy policyv1beta.RunAsUserStrategy
	var ranges []policyv1beta.IDRange

	switch scc.RunAsUser.Type {
	case securityv1.RunAsUserStrategyRunAsAny:
		strategy = policyv1beta.RunAsUserStrategyRunAsAny
	case securityv1.RunAsUserStrategyMustRunAs, securityv1.RunAsUserStrategyMustRunAsRange:
		strategy = policyv1beta.RunAsUserStrategyMustRunAs
	case securityv1.RunAsUserStrategyMustRunAsNonRoot:
		strategy = policyv1beta.RunAsUserStrategyMustRunAsNonRoot
	default:
		return policyv1beta.RunAsUserStrategyOptions{},
			fmt.Errorf("unknown RunAsUser.Type: %q", scc.RunAsUser.Type)
	}

	hasMinUID := scc.RunAsUser.UIDRangeMin != nil
	hasMaxUID := scc.RunAsUser.UIDRangeMax != nil

	if (hasMaxUID && !hasMinUID) || (hasMinUID && !hasMaxUID) {
		return policyv1beta.RunAsUserStrategyOptions{},
			fmt.Errorf("found RunAsUser with half-filled range: Min: %v, Max: %v, expected to have both",
				int64val(scc.RunAsUser.UIDRangeMin), int64val(scc.RunAsUser.UIDRangeMax))
	}

	hasUIDRange := hasMinUID || hasMaxUID
	hasUID := scc.RunAsUser.UID != nil

	if hasUID && hasUIDRange {
		return policyv1beta.RunAsUserStrategyOptions{},
			fmt.Errorf("found RunAsUser with both uid (%v) and range(%v-%v) specified, expected to have only one of them",
				int64val(scc.RunAsUser.UID), int64val(scc.RunAsUser.UIDRangeMin), int64val(scc.RunAsUser.UIDRangeMax))
	}

	if hasUID {
		ranges = createSingleIDRange(*scc.RunAsUser.UID, *scc.RunAsUser.UID)
	}

	if hasUIDRange {
		ranges = createSingleIDRange(*scc.RunAsUser.UIDRangeMin, *scc.RunAsUser.UIDRangeMax)
	}

	opts := policyv1beta.RunAsUserStrategyOptions{
		Rule:   strategy,
		Ranges: ranges,
	}

	return opts, nil
}

// SCC.SupplementalGroups is the same as PSP.SupplementalGroups.
func extractSupplementalGroups(scc *securityv1.SecurityContextConstraints) (policyv1beta.SupplementalGroupsStrategyOptions, error) {
	var strategy policyv1beta.SupplementalGroupsStrategyType
	var ranges []policyv1beta.IDRange

	switch scc.SupplementalGroups.Type {
	case securityv1.SupplementalGroupsStrategyRunAsAny:
		strategy = policyv1beta.SupplementalGroupsStrategyRunAsAny
	case securityv1.SupplementalGroupsStrategyMustRunAs:
		strategy = policyv1beta.SupplementalGroupsStrategyMustRunAs
	default:
		return policyv1beta.SupplementalGroupsStrategyOptions{},
			fmt.Errorf("unknown SupplementalGroups.Type: %q", scc.SupplementalGroups.Type)
	}

	if len(scc.SupplementalGroups.Ranges) > 0 {
		ranges = make([]policyv1beta.IDRange, len(scc.SupplementalGroups.Ranges))
		for idx, rng := range scc.SupplementalGroups.Ranges {
			ranges[idx] = policyv1beta.IDRange{Min: rng.Min, Max: rng.Max}
		}
	}

	opts := policyv1beta.SupplementalGroupsStrategyOptions{
		Rule:   strategy,
		Ranges: ranges,
	}

	return opts, nil
}

// SCC.FSGroup is the same as PSP.FSGroup.
func extractFSGroup(scc *securityv1.SecurityContextConstraints) (policyv1beta.FSGroupStrategyOptions, error) {
	var strategy policyv1beta.FSGroupStrategyType
	var ranges []policyv1beta.IDRange

	switch scc.FSGroup.Type {
	case securityv1.FSGroupStrategyRunAsAny:
		strategy = policyv1beta.FSGroupStrategyRunAsAny
	case securityv1.FSGroupStrategyMustRunAs:
		strategy = policyv1beta.FSGroupStrategyMustRunAs
	default:
		return policyv1beta.FSGroupStrategyOptions{},
			fmt.Errorf("unknown SCC.FSGroup.Type: %q", scc.FSGroup.Type)
	}

	if len(scc.FSGroup.Ranges) > 0 {
		ranges = make([]policyv1beta.IDRange, len(scc.FSGroup.Ranges))
		for idx, rng := range scc.FSGroup.Ranges {
			ranges[idx] = policyv1beta.IDRange{Min: rng.Min, Max: rng.Max}
		}
	}

	opts := policyv1beta.FSGroupStrategyOptions{
		Rule:   strategy,
		Ranges: ranges,
	}

	return opts, nil
}

// SCC.ReadOnlyRootFilesystem is the same as PSP.ReadOnlyRootFilesystem.
func extractReadOnlyRootFilesystem(scc *securityv1.SecurityContextConstraints) bool {
	return scc.ReadOnlyRootFilesystem
}

// SCC.AllowedFlexVolumes are the same as PSP.AllowedFlexVolumes.
func extractAllowedFlexVolumes(scc *securityv1.SecurityContextConstraints) []policyv1beta.AllowedFlexVolume {
	volumes := make([]policyv1beta.AllowedFlexVolume, 0, len(scc.AllowedFlexVolumes))
	for _, volume := range scc.AllowedFlexVolumes {
		allowedVolume := policyv1beta.AllowedFlexVolume{
			Driver: volume.Driver,
		}
		volumes = append(volumes, allowedVolume)
	}
	return volumes
}

// SCC.SeccompProfiles doesn't exist as a field in the PSP. Instead it's represented as 2 annotations:
// one for setting a default profile and another with a list of allowed profiles.
// See for details:
// - https://kubernetes.io/docs/concepts/policy/pod-security-policy/#seccomp
// - https://docs.openshift.org/3.6/admin_guide/seccomp.html
func extractSeccompProfiles(scc *securityv1.SecurityContextConstraints, annotations map[string]string) {
	if len(scc.SeccompProfiles) == 0 {
		return
	}

	nonWildcardProfiles := make([]string, 0, len(scc.SeccompProfiles))
	hasWildcardProfile := false
	for _, profile := range scc.SeccompProfiles {
		if profile == seccomp.AllowAny {
			hasWildcardProfile = true
			continue
		}
		nonWildcardProfiles = append(nonWildcardProfiles, profile)
	}

	hasNonWildcardProfiles := len(nonWildcardProfiles) > 0

	if hasWildcardProfile {
		annotations[seccomp.AllowedProfilesAnnotationKey] = seccomp.AllowAny
		if hasNonWildcardProfiles {
			annotations[seccomp.DefaultProfileAnnotationKey] = nonWildcardProfiles[0] // first non-wildcard is used to be default
		}

	} else {
		annotations[seccomp.AllowedProfilesAnnotationKey] = strings.Join(nonWildcardProfiles, ",")
		annotations[seccomp.DefaultProfileAnnotationKey] = nonWildcardProfiles[0]
	}
}

func extractAllowedUnsafeSysctls(scc *securityv1.SecurityContextConstraints) []string {
	return scc.AllowedUnsafeSysctls
}

func extractForbiddenSysctls(scc *securityv1.SecurityContextConstraints) []string {
	return scc.ForbiddenSysctls
}

// Sysctl support is implemented as an annotation in older versions of OpenShift and Kubernetes.
func extractSysctlsAnnotation(scc *securityv1.SecurityContextConstraints, annotations map[string]string) {
	sysctls, hasSysctls := scc.Annotations[sysctlsPodSecurityPolicyAnnotationKey]
	if hasSysctls {
		annotations[sysctlsPodSecurityPolicyAnnotationKey] = sysctls
	}
}

// PSP doesn't have priority like SCC does. We hold the value of SCC.Priroty field in a custom annotation.
func extractPriority(scc *securityv1.SecurityContextConstraints, annotations map[string]string) {
	if scc.Priority != nil {
		annotations[pspPriorityAnnotationKey] = fmt.Sprintf("%d", *scc.Priority)
	}
}

func extractDefaultAllowPrivilegeEscalation(scc *securityv1.SecurityContextConstraints) *bool {
	return scc.DefaultAllowPrivilegeEscalation
}

func extractAllowPrivilegeEscalation(scc *securityv1.SecurityContextConstraints) *bool {
	return scc.AllowPrivilegeEscalation
}
