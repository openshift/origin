package validation

import (
	"net/url"

	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/openshift/origin/pkg/build/api"
)

// ValidateBuild tests required fields for a Build.
func ValidateBuild(build *api.Build) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(build.ID) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("id", build.ID))
	}
	allErrs = append(allErrs, validateBuildInput(&build.Input).Prefix("input")...)
	return allErrs
}

// ValidateBuildConfig tests required fields for a Build.
func ValidateBuildConfig(config *api.BuildConfig) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(config.ID) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("id", config.ID))
	}
	allErrs = append(allErrs, validateBuildInput(&config.DesiredInput).Prefix("desiredInput")...)
	return allErrs
}

func validateBuildInput(input *api.BuildInput) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(input.SourceURI) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("sourceURI", input.SourceURI))
	} else if !isValidURL(input.SourceURI) {
		allErrs = append(allErrs, errs.NewFieldInvalid("sourceURI", input.SourceURI))
	}
	if len(input.ImageTag) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("imageTag", input.ImageTag))
	}
	if input.Type == api.STIBuildType {
		if len(input.BuilderImage) == 0 {
			allErrs = append(allErrs, errs.NewFieldRequired("builderImage", input.BuilderImage))
		}
	} else {
		if len(input.BuilderImage) != 0 {
			allErrs = append(allErrs, errs.NewFieldInvalid("builderImage", input.BuilderImage))
		}
	}
	return allErrs
}

func isValidURL(uri string) bool {
	_, err := url.Parse(uri)
	return err == nil
}
