package validation

import (
	"fmt"

	"github.com/openshift/source-to-image/pkg/api"
)

// ValidateConfig returns a list of error from validation.
func ValidateConfig(config *api.Config) []ValidationError {
	allErrs := []ValidationError{}
	if len(config.Source) == 0 {
		allErrs = append(allErrs, NewFieldRequired("source"))
	}
	if len(config.BuilderImage) == 0 {
		allErrs = append(allErrs, NewFieldRequired("builderImage"))
	}
	if config.DockerConfig == nil || len(config.DockerConfig.Endpoint) == 0 {
		allErrs = append(allErrs, NewFieldRequired("dockerConfig.endpoint"))
	}
	return allErrs
}

// NewFieldRequired returns a *ValidationError indicating "value required"
func NewFieldRequired(field string) ValidationError {
	return ValidationError{ValidationErrorTypeRequired, field}
}

// ValidationErrorType is a machine readable value providing more detail about why
// a field is invalid.
type ValidationErrorType string

const (
	// ValidationErrorTypeRequired is used to report required values that are not
	// provided (e.g. empty strings, null values, or empty arrays).
	ValidationErrorTypeRequired ValidationErrorType = "FieldValueRequired"
)

// ValidationError is an implementation of the 'error' interface, which represents an error of validation.
type ValidationError struct {
	Type  ValidationErrorType
	Field string
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.Type)
}
