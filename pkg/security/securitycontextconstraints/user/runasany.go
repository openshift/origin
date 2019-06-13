package user

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	securityv1 "github.com/openshift/api/security/v1"
)

// runAsAny implements the interface RunAsUserSecurityContextConstraintsStrategy.
type runAsAny struct{}

var _ RunAsUserSecurityContextConstraintsStrategy = &runAsAny{}

// NewRunAsAny provides a strategy that will return nil.
func NewRunAsAny(options *securityv1.RunAsUserStrategyOptions) (RunAsUserSecurityContextConstraintsStrategy, error) {
	return &runAsAny{}, nil
}

// Generate creates the uid based on policy rules.
func (s *runAsAny) Generate(pod *corev1.Pod, container *corev1.Container) (*int64, error) {
	return nil, nil
}

// Validate ensures that the specified values fall within the range of the strategy.
func (s *runAsAny) Validate(fldPath *field.Path, _ *corev1.Pod, _ *corev1.Container, runAsNonRoot *bool, runAsUser *int64) field.ErrorList {
	return field.ErrorList{}
}
