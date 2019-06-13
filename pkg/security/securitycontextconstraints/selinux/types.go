package selinux

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// SELinuxSecurityContextConstraintsStrategy defines the interface for all SELinux constraint strategies.
type SELinuxSecurityContextConstraintsStrategy interface {
	// Generate creates the SELinuxOptions based on constraint rules.
	Generate(pod *corev1.Pod, container *corev1.Container) (*corev1.SELinuxOptions, error)
	// Validate ensures that the specified values fall within the range of the strategy.
	Validate(fldPath *field.Path, pod *corev1.Pod, container *corev1.Container, options *corev1.SELinuxOptions) field.ErrorList
}
