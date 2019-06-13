package seccomp

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// SeccompStrategy defines the interface for all seccomp constraint strategies.
type SeccompStrategy interface {
	// Generate creates the profile based on policy rules.
	Generate(pod *corev1.Pod) (string, error)
	// ValidatePod ensures that the specified values on the pod fall within the range
	// of the strategy.
	ValidatePod(pod *corev1.Pod) field.ErrorList
	// ValidateContainer ensures that the specified values on the container fall within
	// the range of the strategy.
	ValidateContainer(pod *corev1.Pod, container *corev1.Container) field.ErrorList
}
