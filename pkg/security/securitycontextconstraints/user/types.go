package user

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// RunAsUserSecurityContextConstraintsStrategy defines the interface for all uid constraint strategies.
type RunAsUserSecurityContextConstraintsStrategy interface {
	// Generate creates the uid based on policy rules.
	Generate(pod *corev1.Pod, container *corev1.Container) (*int64, error)
	// Validate ensures that the specified values fall within the range of the strategy.
	Validate(fldPath *field.Path, pod *corev1.Pod, container *corev1.Container, runAsNonRoot *bool, runAsUser *int64) field.ErrorList
}
