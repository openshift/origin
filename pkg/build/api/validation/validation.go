package validation

import (
	"net/url"

	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// ValidateBuild tests required fields for a Build.
func ValidateBuild(build *buildapi.Build) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(build.ID) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("id", build.ID))
	}
	allErrs = append(allErrs, validateBuildParameters(&build.Parameters).Prefix("parameters")...)
	return allErrs
}

// ValidateBuildConfig tests required fields for a Build.
func ValidateBuildConfig(config *buildapi.BuildConfig) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(config.ID) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("id", config.ID))
	}
	allErrs = append(allErrs, validateBuildParameters(&config.Parameters).Prefix("parameters")...)
	return allErrs
}

func validateBuildParameters(params *buildapi.BuildParameters) errs.ErrorList {
	allErrs := errs.ErrorList{}

	allErrs = append(allErrs, validateSource(&params.Source).Prefix("source")...)

	if params.Revision != nil {
		allErrs = append(allErrs, validateRevision(params.Revision).Prefix("revision")...)
	}

	allErrs = append(allErrs, validateStrategy(&params.Strategy).Prefix("strategy")...)
	allErrs = append(allErrs, validateOutput(&params.Output).Prefix("output")...)

	return allErrs
}

func validateSource(input *buildapi.BuildSource) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if input.Type != buildapi.BuildSourceGit {
		allErrs = append(allErrs, errs.NewFieldRequired("type", buildapi.BuildSourceGit))
	}
	if input.Git == nil {
		allErrs = append(allErrs, errs.NewFieldRequired("git", input.Git))
	} else {
		allErrs = append(allErrs, validateGitSource(input.Git).Prefix("git")...)
	}
	return allErrs
}

func validateGitSource(git *buildapi.GitBuildSource) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(git.URI) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("uri", git.URI))
	} else if !isValidURL(git.URI) {
		allErrs = append(allErrs, errs.NewFieldInvalid("uri", git.URI))
	}
	return allErrs
}

func validateRevision(revision *buildapi.SourceRevision) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(revision.Type) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("type", revision.Type))
	}
	// TODO: validate other stuff
	return allErrs
}

func validateStrategy(strategy *buildapi.BuildStrategy) errs.ErrorList {
	allErrs := errs.ErrorList{}

	if len(strategy.Type) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("type", strategy.Type))
	}

	switch strategy.Type {
	case buildapi.STIBuildStrategyType:
		if strategy.STIStrategy == nil {
			allErrs = append(allErrs, errs.NewFieldRequired("stiStrategy", strategy.STIStrategy))
		} else {
			allErrs = append(allErrs, validateSTIStrategy(strategy.STIStrategy).Prefix("stiStrategy")...)
		}
	case buildapi.DockerBuildStrategyType:
		// DockerStrategy is currently optional
	default:
		allErrs = append(allErrs, errs.NewFieldInvalid("type", strategy.Type))
	}

	return allErrs
}

func validateSTIStrategy(strategy *buildapi.STIBuildStrategy) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(strategy.BuilderImage) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("builderImage", strategy.BuilderImage))
	}
	return allErrs
}

func validateOutput(output *buildapi.BuildOutput) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(output.ImageTag) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("imageTag", output.ImageTag))
	}
	return allErrs
}

func isValidURL(uri string) bool {
	_, err := url.Parse(uri)
	return err == nil
}
