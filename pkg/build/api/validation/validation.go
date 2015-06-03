package validation

import (
	"net/url"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ValidateBuild tests required fields for a Build.
func ValidateBuild(build *buildapi.Build) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(build.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	} else if !util.IsDNS1123Subdomain(build.Name) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("name", build.Name, "name must be a valid subdomain"))
	}
	if len(build.Namespace) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("namespace"))
	} else if !util.IsDNS1123Subdomain(build.Namespace) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", build.Namespace, "namespace must be a valid subdomain"))
	}
	allErrs = append(allErrs, validation.ValidateLabels(build.Labels, "labels")...)
	allErrs = append(allErrs, validateBuildParameters(&build.Parameters).Prefix("parameters")...)
	return allErrs
}

// ValidateBuildConfig tests required fields for a Build.
func ValidateBuildConfig(config *buildapi.BuildConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(config.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	} else if !util.IsDNS1123Subdomain(config.Name) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("name", config.Name, "name must be a valid subdomain"))
	}
	if len(config.Namespace) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("namespace"))
	} else if !util.IsDNS1123Subdomain(config.Namespace) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", config.Namespace, "namespace must be a valid subdomain"))
	}
	allErrs = append(allErrs, validation.ValidateLabels(config.Labels, "labels")...)
	// allow only one ImageChangeTrigger for now
	ictCount := 0
	for i, trg := range config.Triggers {
		allErrs = append(allErrs, validateTrigger(&trg).PrefixIndex(i).Prefix("triggers")...)
		if trg.Type == buildapi.ImageChangeBuildTriggerType {
			if ictCount++; ictCount > 1 {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("triggers", config.Triggers, "only one ImageChange trigger is allowed"))
				break
			}
		}
	}
	allErrs = append(allErrs, validateBuildParameters(&config.Parameters).Prefix("parameters")...)
	allErrs = append(allErrs, validateBuildConfigOutput(&config.Parameters.Output).Prefix("parameters.output")...)
	return allErrs
}

// ValidateBuildRequest validates a BuildRequest object
func ValidateBuildRequest(request *buildapi.BuildRequest) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(request.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	}
	if request.Revision != nil {
		allErrs = append(allErrs, validateRevision(request.Revision).Prefix("revision")...)
	}
	return allErrs
}

func validateBuildParameters(params *buildapi.BuildParameters) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	isCustomBuild := params.Strategy.Type == buildapi.CustomBuildStrategyType
	// Validate 'source' and 'output' for all build types except Custom build
	// where they are optional and validated only if present.
	if !isCustomBuild || (isCustomBuild && len(params.Source.Type) != 0) {
		allErrs = append(allErrs, validateSource(&params.Source).Prefix("source")...)

		if params.Revision != nil {
			allErrs = append(allErrs, validateRevision(params.Revision).Prefix("revision")...)
		}
	}

	allErrs = append(allErrs, validateOutput(&params.Output).Prefix("output")...)
	allErrs = append(allErrs, validateStrategy(&params.Strategy).Prefix("strategy")...)

	// TODO: validate resource requirements (prereq: https://github.com/GoogleCloudPlatform/kubernetes/pull/7059)
	return allErrs
}

func validateSource(input *buildapi.BuildSource) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if input.Type != buildapi.BuildSourceGit {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("type"))
	}
	if input.Git == nil {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("git"))
	} else {
		allErrs = append(allErrs, validateGitSource(input.Git).Prefix("git")...)
	}
	allErrs = append(allErrs, validateSecretRef(input.SourceSecret).Prefix("sourceSecret")...)
	return allErrs
}

func validateSecretRef(ref *kapi.LocalObjectReference) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if ref == nil {
		return allErrs
	}
	if len(ref.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	}
	return allErrs
}

func validateGitSource(git *buildapi.GitBuildSource) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(git.URI) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("uri"))
	} else if !isValidURL(git.URI) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("uri", git.URI, "uri is not a valid url"))
	}
	return allErrs
}

func validateRevision(revision *buildapi.SourceRevision) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(revision.Type) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("type"))
	}
	// TODO: validate other stuff
	return allErrs
}

func validateOutput(output *buildapi.BuildOutput) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	// TODO: make part of a generic ValidateObjectReference method upstream.
	if output.To != nil {
		kind, name, namespace := output.To.Kind, output.To.Name, output.To.Namespace
		if len(kind) == 0 {
			kind = "ImageStream"
			output.To.Kind = kind
		}
		if kind != "ImageStream" {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("to.kind", kind, "the target of build output must be 'ImageStream'"))
		}
		if len(name) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("to.name"))
		} else if !util.IsDNS1123Subdomain(name) {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("to.name", name, "name must be a valid subdomain"))
		}
		if len(namespace) != 0 && !util.IsDNS1123Subdomain(namespace) {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("to.namespace", namespace, "namespace must be a valid subdomain"))
		}
	}

	allErrs = append(allErrs, validateSecretRef(output.PushSecret).Prefix("pushSecret")...)

	if len(output.DockerImageReference) != 0 {
		if _, err := imageapi.ParseDockerImageReference(output.DockerImageReference); err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("dockerImageReference", output.DockerImageReference, err.Error()))
		}
	}

	return allErrs
}

func validateBuildConfigOutput(output *buildapi.BuildOutput) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(output.DockerImageReference) != 0 && output.To != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("dockerImageReference", output.DockerImageReference, "only one of 'dockerImageReference' and 'to' may be set"))
	}
	return allErrs
}

func validateStrategy(strategy *buildapi.BuildStrategy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	switch {
	case len(strategy.Type) == 0:
		allErrs = append(allErrs, fielderrors.NewFieldRequired("type"))

	case strategy.Type == buildapi.SourceBuildStrategyType:
		if strategy.SourceStrategy == nil {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("stiStrategy"))
		} else {
			allErrs = append(allErrs, validateSourceStrategy(strategy.SourceStrategy).Prefix("stiStrategy")...)
		}
	case strategy.Type == buildapi.DockerBuildStrategyType:
		// DockerStrategy is currently optional, initialize it to a default state if it's not set.
		if strategy.DockerStrategy == nil {
			strategy.DockerStrategy = &buildapi.DockerBuildStrategy{}
		}
		allErrs = append(allErrs, validateSecretRef(strategy.DockerStrategy.PullSecret).Prefix("pullSecret")...)
	case strategy.Type == buildapi.CustomBuildStrategyType:
		if strategy.CustomStrategy == nil {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("customStrategy"))
		} else {
			allErrs = append(allErrs, validateCustomStrategy(strategy.CustomStrategy).Prefix("customStrategy")...)
		}
	default:
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("type", strategy.Type, "type is not in the enumerated list"))
	}

	return allErrs
}

func validateSourceStrategy(strategy *buildapi.SourceBuildStrategy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if strategy.From == nil || len(strategy.From.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("from"))
	}
	allErrs = append(allErrs, validateSecretRef(strategy.PullSecret).Prefix("pullSecret")...)
	return allErrs
}

func validateCustomStrategy(strategy *buildapi.CustomBuildStrategy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if strategy.From == nil || len(strategy.From.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("from"))
	}
	allErrs = append(allErrs, validateSecretRef(strategy.PullSecret).Prefix("pullSecret")...)
	return allErrs
}

func validateTrigger(trigger *buildapi.BuildTriggerPolicy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(trigger.Type) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("type"))
		return allErrs
	}

	// Ensure that only parameters for the trigger's type are present
	triggerPresence := map[buildapi.BuildTriggerType]bool{
		buildapi.GithubWebHookBuildTriggerType:  trigger.GithubWebHook != nil,
		buildapi.GenericWebHookBuildTriggerType: trigger.GenericWebHook != nil,
	}
	allErrs = append(allErrs, validateTriggerPresence(triggerPresence, trigger.Type)...)

	// Validate each trigger type
	switch trigger.Type {
	case buildapi.GithubWebHookBuildTriggerType:
		if trigger.GithubWebHook == nil {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("github"))
		} else {
			allErrs = append(allErrs, validateWebHook(trigger.GithubWebHook).Prefix("github")...)
		}
	case buildapi.GenericWebHookBuildTriggerType:
		if trigger.GenericWebHook == nil {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("generic"))
		} else {
			allErrs = append(allErrs, validateWebHook(trigger.GenericWebHook).Prefix("generic")...)
		}
	case buildapi.ImageChangeBuildTriggerType:
		if trigger.ImageChange == nil {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("imageChange"))
		}
	}
	return allErrs
}

func validateTriggerPresence(params map[buildapi.BuildTriggerType]bool, t buildapi.BuildTriggerType) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	for triggerType, present := range params {
		if triggerType != t && present {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(string(triggerType), "", "triggerType wasn't found"))
		}
	}
	return allErrs
}

func validateWebHook(webHook *buildapi.WebHookTrigger) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(webHook.Secret) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("secret"))
	}
	return allErrs
}

func isValidURL(uri string) bool {
	_, err := url.Parse(uri)
	return err == nil
}
