package selinux

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	api "k8s.io/kubernetes/pkg/apis/core"
)

// SELinuxSecurityContextConstraintsStrategy defines the interface for all SELinux constraint strategies.
type SELinuxSecurityContextConstraintsStrategy interface {
	// Generate creates the SELinuxOptions based on constraint rules.
	Generate(pod *api.Pod, container *api.Container) (*api.SELinuxOptions, error)
	// Validate ensures that the specified values fall within the range of the strategy.
	Validate(pod *api.Pod, container *api.Container) field.ErrorList
}
