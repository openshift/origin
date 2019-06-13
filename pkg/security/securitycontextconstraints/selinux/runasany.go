package selinux

import (
	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// runAsAny implements the SELinuxSecurityContextConstraintsStrategy interface.
type runAsAny struct{}

var _ SELinuxSecurityContextConstraintsStrategy = &runAsAny{}

// NewRunAsAny provides a strategy that will return the configured se linux context or nil.
func NewRunAsAny(options *securityv1.SELinuxContextStrategyOptions) (SELinuxSecurityContextConstraintsStrategy, error) {
	return &runAsAny{}, nil
}

// Generate creates the SELinuxOptions based on constraint rules.
func (s *runAsAny) Generate(pod *corev1.Pod, container *corev1.Container) (*corev1.SELinuxOptions, error) {
	return nil, nil
}

// Validate ensures that the specified values fall within the range of the strategy.
func (s *runAsAny) Validate(fldPath *field.Path, _ *corev1.Pod, _ *corev1.Container, options *corev1.SELinuxOptions) field.ErrorList {
	return field.ErrorList{}
}
