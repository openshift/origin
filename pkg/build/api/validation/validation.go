package validation

import (
	"net/url"

	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// ValidateBuild tests required fields for a Build.
func ValidateBuild(build *buildapi.Build) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(build.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name", build.Name))
	}
	allErrs = append(allErrs, validateBuildParameters(&build.Parameters).Prefix("parameters")...)
	return allErrs
}

// ValidateBuildConfig tests required fields for a Build.
func ValidateBuildConfig(config *buildapi.BuildConfig) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(config.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name", config.Name))
	}
	for i := range config.Triggers {
		allErrs = append(allErrs, validateTrigger(&config.Triggers[i]).PrefixIndex(i).Prefix("triggers")...)
	}
	allErrs = append(allErrs, validateBuildParameters(&config.Parameters).Prefix("parameters")...)
	return allErrs
}

func validateBuildParameters(params *buildapi.BuildParameters) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	allErrs = append(allErrs, validateSource(&params.Source).Prefix("source")...)

	if params.Revision != nil {
		allErrs = append(allErrs, validateRevision(params.Revision).Prefix("revision")...)
	}

	allErrs = append(allErrs, validateStrategy(&params.Strategy).Prefix("strategy")...)
	allErrs = append(allErrs, validateOutput(&params.Output).Prefix("output")...)

	return allErrs
}

func validateSource(input *buildapi.BuildSource) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
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

func validateGitSource(git *buildapi.GitBuildSource) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(git.URI) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("uri", git.URI))
	} else if !isValidURL(git.URI) {
		allErrs = append(allErrs, errs.NewFieldInvalid("uri", git.URI))
	}
	return allErrs
}

func validateRevision(revision *buildapi.SourceRevision) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(revision.Type) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("type", revision.Type))
	}
	// TODO: validate other stuff
	return allErrs
}

func validateStrategy(strategy *buildapi.BuildStrategy) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

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

func validateSTIStrategy(strategy *buildapi.STIBuildStrategy) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(strategy.BuilderImage) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("builderImage", strategy.BuilderImage))
	}
	return allErrs
}

func validateOutput(output *buildapi.BuildOutput) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(output.ImageTag) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("imageTag", output.ImageTag))
	}
	return allErrs
}

func validateTrigger(trigger *buildapi.BuildTriggerPolicy) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(trigger.Type) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("type", ""))
		return allErrs
	}

	// Ensure that only parameters for the trigger's type are present
	triggerPresence := map[buildapi.BuildTriggerType]bool{
		buildapi.GithubWebHookType:  trigger.GithubWebHook != nil,
		buildapi.GenericWebHookType: trigger.GenericWebHook != nil,
	}
	allErrs = append(allErrs, validateTriggerPresence(triggerPresence, trigger.Type)...)

	// Validate each trigger type
	switch trigger.Type {
	case buildapi.GithubWebHookType:
		if trigger.GithubWebHook == nil {
			allErrs = append(allErrs, errs.NewFieldRequired("github", nil))
		} else {
			allErrs = append(allErrs, validateWebHook(trigger.GithubWebHook).Prefix("github")...)
		}
	case buildapi.GenericWebHookType:
		if trigger.GenericWebHook == nil {
			allErrs = append(allErrs, errs.NewFieldRequired("generic", nil))
		} else {
			allErrs = append(allErrs, validateWebHook(trigger.GenericWebHook).Prefix("generic")...)
		}
	}
	return allErrs
}

func validateTriggerPresence(params map[buildapi.BuildTriggerType]bool, t buildapi.BuildTriggerType) errs.ErrorList {
	allErrs := errs.ErrorList{}
	for triggerType, present := range params {
		if triggerType != t && present {
			allErrs = append(allErrs, errs.NewFieldInvalid(string(triggerType), ""))
		}
	}
	return allErrs
}

func validateWebHook(webHook *buildapi.WebHookTrigger) errs.ErrorList {
	allErrs := errs.ErrorList{}
	if len(webHook.Secret) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("secret", ""))
	}
	return allErrs
}

func isValidURL(uri string) bool {
	_, err := url.Parse(uri)
	return err == nil
}
