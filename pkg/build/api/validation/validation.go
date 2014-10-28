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
	allErrs = append(allErrs, validateBuildSource(&build.Source).Prefix("source")...)
	return allErrs
}

// ValidateBuildConfig tests required fields for a Build.
func ValidateBuildConfig(config *api.BuildConfig) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(config.ID) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("id", config.ID))
	}
	allErrs = append(allErrs, validateBuildInput(&config.DesiredInput).Prefix("desiredInput")...)
	allErrs = append(allErrs, validateBuildSource(&config.Source).Prefix("source")...)
	return allErrs
}

func validateBuildInput(input *api.BuildInput) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(input.ImageTag) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("imageTag", input.ImageTag))
	}
	if input.STIInput != nil {
		allErrs = append(allErrs, validateSTIBuild(input.STIInput).Prefix("stiBuild")...)
	}
	return allErrs
}

func validateBuildSource(input *api.BuildSource) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if input.Type != api.BuildSourceGit {
		allErrs = append(allErrs, errs.NewFieldRequired("Type", api.BuildSourceGit))
	}
	if input.Git == nil {
		allErrs = append(allErrs, errs.NewFieldRequired("Git", input.Git))
	} else {
		if len(input.Git.URI) == 0 {
			allErrs = append(allErrs, errs.NewFieldRequired("Git.URI", input.Git.URI))
		} else if !isValidURL(input.Git.URI) {
			allErrs = append(allErrs, errs.NewFieldInvalid("Git.URI", input.Git.URI))
		}
	}
	return allErrs
}

func validateSTIBuild(sti *api.STIBuildInput) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(sti.BuilderImage) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("builderImage", sti.BuilderImage))
	}
	return allErrs
}

func isValidURL(uri string) bool {
	_, err := url.Parse(uri)
	return err == nil
}
