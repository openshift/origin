package scc

import (
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

// ByRestrictions is a helper to sort SCCs in order of most restrictive to least restrictive.
type ByRestrictions []*securityapi.SecurityContextConstraints

func (s ByRestrictions) Len() int {
	return len(s)
}
func (s ByRestrictions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByRestrictions) Less(i, j int) bool {
	return pointValue(s[i]) < pointValue(s[j])
}

// pointValue places a value on the SCC based on the settings of the SCC that can be used
// to determine how restrictive it is.  The lower the number, the more restrictive it is.
func pointValue(constraint *securityapi.SecurityContextConstraints) int {
	points := 0

	// make sure these are always valued higher than the combination of the highest strategies
	if constraint.AllowPrivilegedContainer {
		points += 20
	}

	// add points based on volume requests
	points += volumePointValue(constraint)

	// strategies in order of least restrictive to most restrictive
	switch constraint.SELinuxContext.Type {
	case securityapi.SELinuxStrategyRunAsAny:
		points += 4
	case securityapi.SELinuxStrategyMustRunAs:
		points += 1
	}

	switch constraint.RunAsUser.Type {
	case securityapi.RunAsUserStrategyRunAsAny:
		points += 4
	case securityapi.RunAsUserStrategyMustRunAsNonRoot:
		points += 3
	case securityapi.RunAsUserStrategyMustRunAsRange:
		points += 2
	case securityapi.RunAsUserStrategyMustRunAs:
		points += 1
	}
	return points
}

// volumePointValue returns a score based on the volumes allowed by the SCC.
// Allowing a host volume will return a score of 10.  Allowance of anything other
// than Secret, ConfigMap, EmptyDir, DownwardAPI, Projected, and None will result in
// a score of 5.  If the SCC only allows these trivial types, it will have a
// score of 0.
func volumePointValue(scc *securityapi.SecurityContextConstraints) int {
	hasHostVolume := false
	hasNonTrivialVolume := false
	for _, v := range scc.Volumes {
		switch v {
		case securityapi.FSTypeHostPath, securityapi.FSTypeAll:
			hasHostVolume = true
			// nothing more to do, this is the max point value
			break
		// it is easier to specifically list the trivial volumes and allow the
		// default case to be non-trivial so we don't have to worry about adding
		// volumes in the future unless they're trivial.
		case securityapi.FSTypeSecret, securityapi.FSTypeConfigMap, securityapi.FSTypeEmptyDir,
			securityapi.FSTypeDownwardAPI, securityapi.FSProjected, securityapi.FSTypeNone:
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
