package capabilities

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// CapabilitiesSecurityContextConstraintsStrategy defines the interface for all cap constraint strategies.
type CapabilitiesSecurityContextConstraintsStrategy interface {
	// Generate creates the capabilities based on policy rules.
	Generate(pod *corev1.Pod, container *corev1.Container) (*corev1.Capabilities, error)
	// Validate ensures that the specified values fall within the range of the strategy.
	Validate(pod *corev1.Pod, container *corev1.Container, capabilities *corev1.Capabilities) field.ErrorList
}
