package securitycontextconstraints

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// SecurityContextConstraintsProvider provides the implementation to generate a new security
// context based on constraints or validate an existing security context against constraints.
type SecurityContextConstraintsProvider interface {
	// Create a PodSecurityContext based on the given constraints.
	CreatePodSecurityContext(pod *corev1.Pod) (*corev1.PodSecurityContext, map[string]string, error)
	// Create a container SecurityContext based on the given constraints
	CreateContainerSecurityContext(pod *corev1.Pod, container *corev1.Container) (*corev1.SecurityContext, error)
	// Ensure a pod's SecurityContext is in compliance with the given constraints.
	ValidatePodSecurityContext(pod *corev1.Pod, fldPath *field.Path) field.ErrorList
	// Ensure a container's SecurityContext is in compliance with the given constraints
	ValidateContainerSecurityContext(pod *corev1.Pod, container *corev1.Container, fldPath *field.Path) field.ErrorList
	// Get the name of the SCC that this provider was initialized with.
	GetSCCName() string
	// Get the users associated to the SCC this provider was initialized with
	GetSCCUsers() []string
	// Get the groups associated to the SCC this provider was initialized with
	GetSCCGroups() []string
}
