package scc

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// ByRestrictions is a helper to sort SCCs in order of most restrictive to least restrictive.
type ByRestrictions []*kapi.SecurityContextConstraints

func (s ByRestrictions) Len() int {
	return len(s)
}
func (s ByRestrictions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByRestrictions) Less(i, j int) bool {
	return pointValue(s[i]) < pointValue(s[j])
}

// pointValue places a value on the SCC based on the settings of the SCC that can be used
// to determine how restrictive it is.  The lower the number, the more restrictive it is.
func pointValue(constraint *kapi.SecurityContextConstraints) int {
	points := 0

	// make sure these are always valued higher than the combination of the highest strategies
	if constraint.AllowPrivilegedContainer {
		points += 20
	}

	// add points based on volume requests
	points += volumePointValue(constraint)

	// strategies in order of least restrictive to most restrictive
	switch constraint.SELinuxContext.Type {
	case kapi.SELinuxStrategyRunAsAny:
		points += 4
	case kapi.SELinuxStrategyMustRunAs:
		points += 1
	}

	switch constraint.RunAsUser.Type {
	case kapi.RunAsUserStrategyRunAsAny:
		points += 4
	case kapi.RunAsUserStrategyMustRunAsNonRoot:
		points += 3
	case kapi.RunAsUserStrategyMustRunAsRange:
		points += 2
	case kapi.RunAsUserStrategyMustRunAs:
		points += 1
	}
	return points
}

// allowsHostPathVolume returns a score based on the volumes allowed by the SCC.
// Allowing a host volume wil return a score of 10.  Allowance of anything other
// than kapi.FSTypeSecret, kapi.FSTypeConfigMap, kapi.FSTypeConfigMap, kapi.FSTypeDownwardAPI
// will result in a score of 5.  If the SCC only allows kapi.FSTypeSecret, kapi.FSTypeConfigMap,
// kapi.FSTypeEmptyDir, kapi.FSTypeDownwardAPI it will have a score of 0.
func volumePointValue(scc *kapi.SecurityContextConstraints) int {
	hasHostVolume := false
	hasNonTrivialVolume := false
	for _, v := range scc.Volumes {
		switch v {
		case kapi.FSTypeHostPath, kapi.FSTypeAll:
			hasHostVolume = true
			// nothing more to do, this is the max point value
			break
		// it is easier to specifically list the trivial volumes and allow the
		// default case to be non-trivial so we don't have to worry about adding
		// volumes in the future unless they're trivial.
		case kapi.FSTypeSecret, kapi.FSTypeConfigMap,
			kapi.FSTypeEmptyDir, kapi.FSTypeDownwardAPI:
			// do nothing
		default:
			hasNonTrivialVolume = true
		}
	}

	if hasHostVolume {
		return 10
	}
	if hasNonTrivialVolume {
		return 5
	}
	return 0
}
