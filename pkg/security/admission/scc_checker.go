package admission

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// constraintChecker checks pod against a constraint.
type constraintChecker interface {
	// allowsPod checks whether a pod will be allowed by a constraint.
	allowsPod(pod *kapi.Pod) bool

	// forbidsPod checks whether a pod will be forbidden by a constraint.
	forbidsPod(pod *kapi.Pod) bool
}

// sccChecker implements constraintChecker interface for checking a pod against SecurityContextConstraints.
type sccChecker struct {
	scc *kapi.SecurityContextConstraints
}

var _ constraintChecker = &sccChecker{}

// NewSccChecker creates a new SCC checker.
func NewSccChecker(scc *kapi.SecurityContextConstraints) constraintChecker {
	// TODO: check for nil
	return &sccChecker{
		scc:scc,
	}
}

// allowsPod checks whether a pod will be allowed by SCC.
func (checker *sccChecker) allowsPod(pod *kapi.Pod) bool {
	return !checker.forbidsPod(pod)
}

// forbidsPod checks whether a pod will be forbidden by SCC.
func (checker *sccChecker) forbidsPod(pod *kapi.Pod) bool {
	// TODO: this method expects that all the fields in SecurityContext filled with non-null values

	if violatesAllowPrivilegedContainer(checker.scc, pod) ||
			violatesDefaultAddCapabilities(checker.scc, pod) ||
			violatesRequiredDropCapabilities(checker.scc, pod) ||
			violatesAllowedCapabilities(checker.scc, pod) ||
			violatesVolumes(checker.scc, pod) ||
			violatesAllowHostNetwork(checker.scc, pod) ||
			violatesAllowHostPorts(checker.scc, pod) ||
			violatesAllowHostPID(checker.scc, pod) ||
			violatesAllowHostIPC(checker.scc, pod) ||
			violatesSELinuxOptions(checker.scc, pod) ||
			violatesRunAsUser(checker.scc, pod) ||
			violatesSupplementalGroups(checker.scc, pod) ||
			violatesFsGroup(checker.scc, pod) ||
			violatesReadOnlyRootFilesystem(checker.scc, pod) ||
			violatesSeccompProfiles(checker.scc, pod) {
		return true
	}

	return false
}


//
// Helpers
//

func violatesAllowPrivilegedContainer(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	if scc.AllowPrivilegedContainer {
		return false
	}

	return hasPrivilegedContainer(pod) || hasPrivilegedInitContainer(pod)
}

func violatesDefaultAddCapabilities(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	return false // TODO
}

func violatesRequiredDropCapabilities(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	return false // TODO
}

func violatesAllowedCapabilities(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	return false // TODO
}

func violatesVolumes(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	return false // TODO
}

func violatesAllowHostNetwork(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	if scc.AllowHostNetwork {
		return false
	}

	return pod.Spec.SecurityContext.HostNetwork
}

func violatesAllowHostPorts(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	if scc.AllowHostPorts {
		return false
	}

	return hasContainerWithHostPort(pod) || hasInitContainerWithHostPort(pod)
}

func violatesAllowHostPID(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	if scc.AllowHostPID {
		return false
	}

	return pod.Spec.SecurityContext.HostPID
}

func violatesAllowHostIPC(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	if scc.AllowHostIPC {
		return false
	}

	return pod.Spec.SecurityContext.HostIPC
}

func violatesSELinuxOptions(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	switch scc.SELinuxContext.Type {
	case kapi.SELinuxStrategyRunAsAny:
		return false
	case kapi.SELinuxStrategyMustRunAs:
		mandatoryOptions := scc.SELinuxContext.SELinuxOptions
		return !containersHaveSelinuxOptions(mandatoryOptions, pod) ||
				!initContainersHaveSelinuxOptions(mandatoryOptions, pod)
	default:
		return true // FIXME: how to handle unknown strategy type?
	}
}

func violatesRunAsUser(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	return false // TODO
}

func violatesSupplementalGroups(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	switch scc.SupplementalGroups.Type {
	case kapi.SupplementalGroupsStrategyRunAsAny:
		return false
	case kapi.SupplementalGroupsStrategyMustRunAs:
		for _, supplementalGroup := range pod.Spec.SecurityContext.SupplementalGroups {
			if fallsIntoRange(&supplementalGroup, scc.SupplementalGroups.Ranges) {
				return true
			}
		}
		return false
	default:
		return true // FIXME: how to handle unknown strategy type?
	}
}

func violatesFsGroup(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	switch scc.FSGroup.Type {

	case kapi.FSGroupStrategyRunAsAny:
		return false

	case kapi.FSGroupStrategyMustRunAs:
		// FIXME: what empty range means: all are allowed or none?
		if len(scc.FSGroup.Ranges) == 0 {
			return false
		}
		fsGroup := pod.Spec.SecurityContext.FSGroup
		return unspecified(fsGroup) || !fallsIntoRange(fsGroup, scc.FSGroup.Ranges)

	default:
		return true // FIXME: how to handle unknown strategy type?
	}
}

func violatesReadOnlyRootFilesystem(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	if scc.ReadOnlyRootFilesystem {
		return hasNonReadOnlyContainer(pod) || hasNonReadOnlyInitContainer(pod)
	}

	return false

}

func violatesSeccompProfiles(scc *kapi.SecurityContextConstraints, pod *kapi.Pod) bool {
	return false // TODO
}


//
// Predicates for Container
//

func isPrivileged(container *kapi.Container) bool {
	sc := container.SecurityContext
	if sc == nil {
		return false
	}
	privileged := sc.Privileged
	return privileged != nil && *privileged
}

func isUsingHostPort(container *kapi.Container) bool {
	for _, port := range container.Ports {
		if port.HostPort > 0 { // FIXME: is it correct?
			return true
		}
	}
	return false
}

func nonReadOnly(container *kapi.Container) bool {
	readOnly := container.SecurityContext.ReadOnlyRootFilesystem
	return !(readOnly != nil && *readOnly)
}


//
// Predicates for Pod
//

func hasPrivilegedContainer(pod *kapi.Pod) bool {
	return existContainerThat(isPrivileged, pod.Spec.Containers)
}

func hasPrivilegedInitContainer(pod *kapi.Pod) bool {
	return existContainerThat(isPrivileged, pod.Spec.InitContainers)
}

func hasContainerWithHostPort(pod *kapi.Pod) bool {
	return existContainerThat(isUsingHostPort, pod.Spec.Containers)
}

func hasInitContainerWithHostPort(pod *kapi.Pod) bool {
	return existContainerThat(isUsingHostPort, pod.Spec.InitContainers)
}

func containersHaveSelinuxOptions(requiredSelinuxOptions *kapi.SELinuxOptions, pod *kapi.Pod) bool {
	podSelinuxOptions := pod.Spec.SecurityContext.SELinuxOptions
	return !existContainerThat(doesNotConformsToItsOwnOrPodSelinuxOptions(requiredSelinuxOptions, podSelinuxOptions), pod.Spec.Containers)
}

func initContainersHaveSelinuxOptions(requiredSelinuxOptions *kapi.SELinuxOptions, pod *kapi.Pod) bool {
	podSelinuxOptions := pod.Spec.SecurityContext.SELinuxOptions
	return !existContainerThat(doesNotConformsToItsOwnOrPodSelinuxOptions(requiredSelinuxOptions, podSelinuxOptions), pod.Spec.InitContainers)
}

func hasNonReadOnlyContainer(pod *kapi.Pod) bool {
	return existContainerThat(nonReadOnly, pod.Spec.Containers)
}

func hasNonReadOnlyInitContainer(pod *kapi.Pod) bool {
	return existContainerThat(nonReadOnly, pod.Spec.InitContainers)
}


//
// Utilities
//

func doesNotConformsToItsOwnOrPodSelinuxOptions(requiredSelinuxOptions *kapi.SELinuxOptions, podSelinuxOptions *kapi.SELinuxOptions) func(*kapi.Container) bool {
	return func(container *kapi.Container) bool {
		if requiredSelinuxOptions == nil {
			return false
		}

		sc := container.SecurityContext
		if sc == nil {
			return true
		}

		// FIXME: is it possible that User specified on the container level and Role on the pod level?
		effectiveSelinuxOptions := sc.SELinuxOptions
		if effectiveSelinuxOptions == nil {
			effectiveSelinuxOptions = podSelinuxOptions
		}

		if effectiveSelinuxOptions == nil {
			return true
		}

		return !selinuxOptionsAreEqual(requiredSelinuxOptions, effectiveSelinuxOptions)
	}
}

func selinuxOptionsAreEqual(left *kapi.SELinuxOptions, right *kapi.SELinuxOptions) bool {
	return left.User == right.User &&
			left.Role == right.Role &&
			left.Type == right.Type &&
			left.Level == right.Level
}

func unspecified(value *int64) bool {
	return value == nil
}

func fallsIntoRange(value *int64, allowedRanges []kapi.IDRange) bool {
	for _, allowedRange := range allowedRanges {
		if *value < allowedRange.Min {
			return false
		}
		if *value > allowedRange.Max {
			return false
		}
	}
	return true
}

func existContainerThat(acceptable func(*kapi.Container) bool, containers []kapi.Container) bool {
	for _, container := range  containers {
		if acceptable(&container) {
			return true
		}
	}
	return false
}
