package validation

import (
	"fmt"
	"strings"

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
	if config.DockerNetworkMode != "" && !validateDockerNetworkMode(config.DockerNetworkMode) {
		allErrs = append(allErrs, NewFieldInvalidValue("dockerNetworkMode"))
	}
	return allErrs
}

// validateDockerNetworkMode checks wether the network mode conforms to the docker remote API specification (v1.19)
// Supported values are: bridge, host, and container:<name|id>
func validateDockerNetworkMode(mode api.DockerNetworkMode) bool {
	switch mode {
	case api.DockerNetworkModeBridge, api.DockerNetworkModeHost:
		return true
	}
	if strings.HasPrefix(string(mode), api.DockerNetworkModeContainerPrefix) {
		return true
	}
	return false
}

// NewFieldRequired returns a *ValidationError indicating "value required"
func NewFieldRequired(field string) ValidationError {
	return ValidationError{ValidationErrorTypeRequired, field}
}

// NewFieldInvalidValue returns a ValidationError indicating "invalid value"
func NewFieldInvalidValue(field string) ValidationError {
	return ValidationError{ValidationErrorInvalidValue, field}
}

// ValidationErrorType is a machine readable value providing more detail about why
// a field is invalid.
type ValidationErrorType string

const (
	// ValidationErrorTypeRequired is used to report required values that are not
	// provided (e.g. empty strings, null values, or empty arrays).
	ValidationErrorTypeRequired ValidationErrorType = "FieldValueRequired"

	// ValidationErrorInvalidValue is used to report values that do not conform to
	// the expected schema.
	ValidationErrorInvalidValue ValidationErrorType = "InvalidValue"
)

// ValidationError is an implementation of the 'error' interface, which represents an error of validation.
type ValidationError struct {
	Type  ValidationErrorType
	Field string
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.Type)
}
