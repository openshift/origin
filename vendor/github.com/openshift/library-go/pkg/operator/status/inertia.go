package status

import (
	"fmt"
	"regexp"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// Inertia returns the inertial duration for the given condition.
type Inertia func(condition operatorv1.OperatorCondition) time.Duration

// InertiaCondition configures an inertia duration for a given set of
// condition types.
type InertiaCondition struct {
	// ConditionTypeMatcher is a regular expression selecting condition types
	// with which this InertiaCondition is associated.
	ConditionTypeMatcher *regexp.Regexp

	// Duration is the inertial duration for associated conditions.
	Duration time.Duration
}

// InertiaConfig holds configuration for an Inertia implementation.
type InertiaConfig struct {
	defaultDuration time.Duration
	conditions      []InertiaCondition
}

// NewInertia creates a new InertiaConfig object.  Conditions are
// applied in the given order, so a condition type matching multiple
// regular expressions will have the duration associated with the first
// matching entry.
func NewInertia(defaultDuration time.Duration, conditions ...InertiaCondition) (*InertiaConfig, error) {
	for i, condition := range conditions {
		if condition.ConditionTypeMatcher == nil {
			return nil, fmt.Errorf("condition %d has a nil ConditionTypeMatcher", i)
		}
	}

	return &InertiaConfig{
		defaultDuration: defaultDuration,
		conditions:      conditions,
	}, nil
}

// MustNewInertia is like NewInertia but panics on error.
func MustNewInertia(defaultDuration time.Duration, conditions ...InertiaCondition) *InertiaConfig {
	inertia, err := NewInertia(defaultDuration, conditions...)
	if err != nil {
		panic(err)
	}

	return inertia
}

// Inertia returns the configured inertia for the given condition type.
func (c *InertiaConfig) Inertia(condition operatorv1.OperatorCondition) time.Duration {
	for _, matcher := range c.conditions {
		if matcher.ConditionTypeMatcher.MatchString(condition.Type) {
			return matcher.Duration
		}
	}
	return c.defaultDuration
}
