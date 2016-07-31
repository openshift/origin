package scc

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"

	oscache "github.com/openshift/origin/pkg/client/cache"
)

// SCCMatcher defines interface for SecurityContextConstraint matcher
type SCCMatcher interface {
	FindApplicableSCCs(user user.Info) ([]*kapi.SecurityContextConstraints, error)
}

// DefaultSCCMatcher implements default implementation for SCCMatcher interface
type DefaultSCCMatcher struct {
	cache *oscache.IndexerToSecurityContextConstraintsLister
}

// NewDefaultSCCMatcher builds and initializes a DefaultSCCMatcher
func NewDefaultSCCMatcher(c *oscache.IndexerToSecurityContextConstraintsLister) SCCMatcher {
	return DefaultSCCMatcher{cache: c}
}

// FindApplicableSCCs implements SCCMatcher interface for DefaultSCCMatcher
func (d DefaultSCCMatcher) FindApplicableSCCs(userInfo user.Info) ([]*kapi.SecurityContextConstraints, error) {
	var matchedConstraints []*kapi.SecurityContextConstraints
	constraints, err := d.cache.List()
	if err != nil {
		return nil, err
	}
	for _, constraint := range constraints {
		if ConstraintAppliesTo(constraint, userInfo) {
			matchedConstraints = append(matchedConstraints, constraint)
		}
	}
	return matchedConstraints, nil
}

// ConstraintAppliesTo inspects the constraint's users and groups against the userInfo to determine
// if it is usable by the userInfo.
func ConstraintAppliesTo(constraint *kapi.SecurityContextConstraints, userInfo user.Info) bool {
	for _, user := range constraint.Users {
		if userInfo.GetName() == user {
			return true
		}
	}
	for _, userGroup := range userInfo.GetGroups() {
		if constraintSupportsGroup(userGroup, constraint.Groups) {
			return true
		}
	}
	return false
}

// constraintSupportsGroup checks that group is in constraintGroups.
func constraintSupportsGroup(group string, constraintGroups []string) bool {
	for _, g := range constraintGroups {
		if g == group {
			return true
		}
	}
	return false
}
