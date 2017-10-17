package selinux

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
	api "k8s.io/kubernetes/pkg/apis/core"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

type mustRunAs struct {
	opts *securityapi.SELinuxContextStrategyOptions
}

var _ SELinuxSecurityContextConstraintsStrategy = &mustRunAs{}

func NewMustRunAs(options *securityapi.SELinuxContextStrategyOptions) (SELinuxSecurityContextConstraintsStrategy, error) {
	if options == nil {
		return nil, fmt.Errorf("MustRunAs requires SELinuxContextStrategyOptions")
	}
	if options.SELinuxOptions == nil {
		return nil, fmt.Errorf("MustRunAs requires SELinuxOptions")
	}
	return &mustRunAs{
		opts: options,
	}, nil
}

// Generate creates the SELinuxOptions based on constraint rules.
func (s *mustRunAs) Generate(_ *api.Pod, _ *api.Container) (*api.SELinuxOptions, error) {
	return s.opts.SELinuxOptions, nil
}

// Validate ensures that the specified values fall within the range of the strategy.
func (s *mustRunAs) Validate(fldPath *field.Path, _ *api.Pod, _ *api.Container, seLinux *api.SELinuxOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	if seLinux == nil {
		allErrs = append(allErrs, field.Required(fldPath, ""))
		return allErrs
	}
	if seLinux.Level != s.opts.SELinuxOptions.Level {
		detail := fmt.Sprintf("must be %s", s.opts.SELinuxOptions.Level)
		allErrs = append(allErrs, field.Invalid(fldPath.Child("level"), seLinux.Level, detail))
	}
	if seLinux.Role != s.opts.SELinuxOptions.Role {
		detail := fmt.Sprintf("must be %s", s.opts.SELinuxOptions.Role)
		allErrs = append(allErrs, field.Invalid(fldPath.Child("role"), seLinux.Role, detail))
	}
	if seLinux.Type != s.opts.SELinuxOptions.Type {
		detail := fmt.Sprintf("must be %s", s.opts.SELinuxOptions.Type)
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), seLinux.Type, detail))
	}
	if seLinux.User != s.opts.SELinuxOptions.User {
		detail := fmt.Sprintf("must be %s", s.opts.SELinuxOptions.User)
		allErrs = append(allErrs, field.Invalid(fldPath.Child("user"), seLinux.User, detail))
	}

	return allErrs
}
