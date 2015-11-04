package admission

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// ByRestrictions is a helper to sort SCCs based on priority.  If priorities are equal
// a string compare of the name is used.
type ByPriority []*kapi.SecurityContextConstraints

func (s ByPriority) Len() int {
	return len(s)
}
func (s ByPriority) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByPriority) Less(i, j int) bool {
	iSCC := s[i]
	jSCC := s[j]

	iSCCPriority := getPriority(iSCC)
	jSCCPriority := getPriority(jSCC)

	// a higher priority is considered "less" so that it moves to the front of the line
	if iSCCPriority > jSCCPriority {
		return true
	}

	if iSCCPriority < jSCCPriority {
		return false
	}

	// priorities are equal, let's try point values
	iRestrictionScore := pointValue(iSCC)
	jRestrictionScore := pointValue(jSCC)

	// a lower restriction score is considered "less" so that it moves to the front of the line
	// (the greater the score, the more lax the SCC is)
	if iRestrictionScore < jRestrictionScore {
		return true
	}

	if iRestrictionScore > jRestrictionScore {
		return false
	}

	// they are still equal, sort by name
	return iSCC.Name < jSCC.Name
}

func getPriority(scc *kapi.SecurityContextConstraints) int {
	if scc.Priority == nil {
		return 0
	}
	return *scc.Priority
}
