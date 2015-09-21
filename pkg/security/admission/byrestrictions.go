package admission

import (
	"github.com/openshift/origin/pkg/security/policy/api"
)

// ByRestrictions is a helper to sort SCCs in order of most restrictive to least restrictive.
type ByRestrictions []*api.PodSecurityPolicy

func (s ByRestrictions) Len() int {
	return len(s)
}
func (s ByRestrictions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByRestrictions) Less(i, j int) bool {
	return s.pointValue(s[i]) < s.pointValue(s[j])
}

// pointValue places a value on the SCC based on the settings of the SCC that can be used
// to determine how restrictive it is.  The lower the number, the more restrictive it is.
func (s ByRestrictions) pointValue(constraint *api.PodSecurityPolicy) int {
	points := 0

	// make sure these are always valued higher than the combination of the highest strategies
	if constraint.Spec.Privileged {
		points += 20
	}
	// 9 gives us a value slightly higher than an SCC that allows run as any in both strategies since
	// we're allowing access to the host system
	//TODO extra considerations for other volume plugin types?
	if constraint.Spec.Volumes.HostPath {
		points += 10
	}

	// TODO host pid/ipc/ports

	// strategies in order of least restrictive to most restrictive
	switch constraint.Spec.SELinuxContext.Type {
	case api.SELinuxStrategyRunAsAny:
		points += 4
	case api.SELinuxStrategyMustRunAs:
		points += 1
	}

	switch constraint.Spec.RunAsUser.Type {
	case api.RunAsUserStrategyRunAsAny:
		points += 4
	case api.RunAsUserStrategyMustRunAsNonRoot:
		points += 3
	case api.RunAsUserStrategyMustRunAsRange:
		points += 2
	case api.RunAsUserStrategyMustRunAs:
		points += 1
	}
	return points
}
