package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// ProjectRequestLimitConfig is the configuration for the project request limit plug-in
// It contains an ordered list of limits based on user label selectors. Selectors will
// be checked in order and the first one that applies will be used as the limit.
type ProjectRequestLimitConfig struct {
	unversioned.TypeMeta
	Limits []ProjectLimitBySelector
}

// ProjectLimitBySelector specifies the maximum number of projects allowed for a given user label selector
type ProjectLimitBySelector struct {
	// Selector is a user label selector. An empty selector selects everything.
	Selector map[string]string
	// MaxProjects is the number of projects allowed for this class of users. If MaxProjects is nil,
	// there is no limit to the number of projects users can request. An unlimited number of projects
	// is useful in the case a limit is specified as the default for all users and only users with a
	// specific set of labels should be allowed unlimited project creation.
	MaxProjects *int
}
