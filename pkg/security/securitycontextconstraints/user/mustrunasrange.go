package user

import (
	"fmt"

	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// mustRunAsRange implements the RunAsUserSecurityContextConstraintsStrategy interface
type mustRunAsRange struct {
	opts *securityv1.RunAsUserStrategyOptions
}

var _ RunAsUserSecurityContextConstraintsStrategy = &mustRunAsRange{}

// NewMustRunAsRange provides a strategy that requires the container to run as a specific UID in a range.
func NewMustRunAsRange(options *securityv1.RunAsUserStrategyOptions) (RunAsUserSecurityContextConstraintsStrategy, error) {
	if options == nil {
		return nil, fmt.Errorf("MustRunAsRange requires run as user options")
	}
	if options.UIDRangeMin == nil {
		return nil, fmt.Errorf("MustRunAsRange requires a UIDRangeMin")
	}
	if options.UIDRangeMax == nil {
		return nil, fmt.Errorf("MustRunAsRange requires a UIDRangeMax")
	}
	return &mustRunAsRange{
		opts: options,
	}, nil
}

// Generate creates the uid based on policy rules.  MustRunAs returns the UIDRangeMin it is initialized with.
func (s *mustRunAsRange) Generate(pod *corev1.Pod, container *corev1.Container) (*int64, error) {
	return s.opts.UIDRangeMin, nil
}

// Validate ensures that the specified values fall within the range of the strategy.
func (s *mustRunAsRange) Validate(fldPath *field.Path, _ *corev1.Pod, _ *corev1.Container, runAsNonRoot *bool, runAsUser *int64) field.ErrorList {
	allErrs := field.ErrorList{}

	if runAsUser == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("runAsUser"), ""))
		return allErrs
	}

	if *runAsUser < *s.opts.UIDRangeMin || *runAsUser > *s.opts.UIDRangeMax {
		detail := fmt.Sprintf("must be in the ranges: [%v, %v]", *s.opts.UIDRangeMin, *s.opts.UIDRangeMax)
		allErrs = append(allErrs, field.Invalid(fldPath.Child("runAsUser"), *runAsUser, detail))
		return allErrs
	}

	return allErrs
}
