package user

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/api"
)

// RunAsUserSecurityContextConstraintsStrategy defines the interface for all uid constraint strategies.
type RunAsUserSecurityContextConstraintsStrategy interface {
	// Generate creates the uid based on policy rules.
	Generate(pod *api.Pod, container *api.Container) (*int64, error)
	// Validate ensures that the specified values fall within the range of the strategy.
	Validate(pod *api.Pod, container *api.Container) field.ErrorList
}
