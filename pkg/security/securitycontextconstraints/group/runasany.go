package group

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// mustRunAs implements the GroupSecurityContextConstraintsStrategy interface
type runAsAny struct {
}

var _ GroupSecurityContextConstraintsStrategy = &runAsAny{}

// NewRunAsAny provides a new RunAsAny strategy.
func NewRunAsAny() (GroupSecurityContextConstraintsStrategy, error) {
	return &runAsAny{}, nil
}

// Generate creates the group based on policy rules.  This strategy returns an empty slice.
func (s *runAsAny) Generate(_ *corev1.Pod) ([]int64, error) {
	return nil, nil
}

// Generate a single value to be applied.  This is used for FSGroup.  This strategy returns nil.
func (s *runAsAny) GenerateSingle(_ *corev1.Pod) (*int64, error) {
	return nil, nil
}

// Validate ensures that the specified values fall within the range of the strategy.
func (s *runAsAny) Validate(_ *corev1.Pod, groups []int64) field.ErrorList {
	return field.ErrorList{}

}
