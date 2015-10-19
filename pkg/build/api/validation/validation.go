package validation

import (
	"fmt"
	"net/url"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/fielderrors"
	kvalidation "k8s.io/kubernetes/pkg/util/validation"

	oapi "github.com/openshift/origin/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ValidateBuild tests required fields for a Build.
func ValidateBuild(build *buildapi.Build) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&build.ObjectMeta, true, validation.NameIsDNSSubdomain).Prefix("metadata")...)
	allErrs = append(allErrs, validateBuildSpec(&build.Spec).Prefix("spec")...)
	return allErrs
}

func ValidateBuildUpdate(build *buildapi.Build, older *buildapi.Build) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&build.ObjectMeta, &older.ObjectMeta).Prefix("metadata")...)

	allErrs = append(allErrs, ValidateBuild(build)...)

	if !kapi.Semantic.DeepEqual(build.Spec, older.Spec) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("spec", "content of spec is not printed out, please refer to the \"details\"", "spec is immutable"))
	}

	return allErrs
}

// refKey returns a key for the given ObjectReference. If the ObjectReference
// doesn't include a namespace, the passed in namespace is used for the reference
func refKey(namespace string, ref *kapi.ObjectReference) string {
	if ref == nil || ref.Kind != "ImageStreamTag" {
		return "nil"
	}
	ns := ref.Namespace
	if ns == "" {
		ns = namespace
	}
	return fmt.Sprintf("%s/%s", ns, ref.Name)
}

// ValidateBuildConfig tests required fields for a Build.
func ValidateBuildConfig(config *buildapi.BuildConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&config.ObjectMeta, true, validation.NameIsDNSSubdomain).Prefix("metadata")...)

	// image change triggers that refer
	fromRefs := map[string]struct{}{}
	for i, trg := range config.Spec.Triggers {
		allErrs = append(allErrs, validateTrigger(&trg).PrefixIndex(i).Prefix("triggers")...)
		if trg.Type != buildapi.ImageChangeBuildTriggerType || trg.ImageChange == nil {
			continue
		}
		from := trg.ImageChange.From
		if from == nil {
			from = buildutil.GetImageStreamForStrategy(config.Spec.Strategy)
		}
		fromKey := refKey(config.Namespace, from)
		_, exists := fromRefs[fromKey]
		if exists {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("triggers", config.Spec.Triggers, "multiple ImageChange triggers refer to the same image stream tag"))
		}
		fromRefs[fromKey] = struct{}{}
	}

	allErrs = append(allErrs, validateBuildSpec(&config.Spec.BuildSpec).Prefix("spec")...)

	// validate ImageChangeTriggers of DockerStrategy builds
	strategy := config.Spec.BuildSpec.Strategy
	if strategy.Type == buildapi.DockerBuildStrategyType && strategy.DockerStrategy.From == nil {
		for _, trigger := range config.Spec.Triggers {
			if trigger.Type == buildapi.ImageChangeBuildTriggerType && (trigger.ImageChange == nil || trigger.ImageChange.From == nil) {
				allErrs = append(allErrs, fielderrors.NewFieldRequired("imageChange.from"))
			}
		}
	}

	return allErrs
}

func ValidateBuildConfigUpdate(config *buildapi.BuildConfig, older *buildapi.BuildConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&config.ObjectMeta, &older.ObjectMeta).Prefix("metadata")...)

	allErrs = append(allErrs, ValidateBuildConfig(config)...)
	return allErrs
}

// ValidateBuildRequest validates a BuildRequest object
func ValidateBuildRequest(request *buildapi.BuildRequest) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&request.ObjectMeta, true, oapi.MinimalNameRequirements).Prefix("metadata")...)

	if request.Revision != nil {
		allErrs = append(allErrs, validateRevision(request.Revision).Prefix("revision")...)
	}
	return allErrs
}

func validateBuildSpec(spec *buildapi.BuildSpec) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	hasSourceType := len(spec.Source.Type) != 0
	switch t := spec.Strategy.Type; {
	// 'source' is optional for Custom builds
	case t == buildapi.CustomBuildStrategyType && hasSourceType:
		allErrs = append(allErrs, validateSource(&spec.Source).Prefix("source")...)
	case t == buildapi.SourceBuildStrategyType:
		allErrs = append(allErrs, validateSource(&spec.Source).Prefix("source")...)
		if spec.Source.Type == buildapi.BuildSourceDockerfile {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("source.type", nil, "may not be type Dockerfile for source builds"))
		}
	case t == buildapi.DockerBuildStrategyType:
		allErrs = append(allErrs, validateSource(&spec.Source).Prefix("source")...)
	}
	if spec.Revision != nil {
		allErrs = append(allErrs, validateRevision(spec.Revision).Prefix("revision")...)
	}
	if spec.CompletionDeadlineSeconds != nil {
		if *spec.CompletionDeadlineSeconds <= 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("completionDeadlineSeconds", spec.CompletionDeadlineSeconds, "completionDeadlineSeconds must be a positive integer greater than 0"))
		}
	}

	allErrs = append(allErrs, validateOutput(&spec.Output).Prefix("output")...)
	allErrs = append(allErrs, validateStrategy(&spec.Strategy).Prefix("strategy")...)

	// TODO: validate resource requirements (prereq: https://github.com/kubernetes/kubernetes/pull/7059)
	return allErrs
}

const maxDockerfileLengthBytes = 60 * 1000

func validateSource(input *buildapi.BuildSource) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	switch input.Type {
	case buildapi.BuildSourceGit:
		if input.Git == nil {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("git"))
		} else {
			allErrs = append(allErrs, validateGitSource(input.Git).Prefix("git")...)
		}
		if input.Dockerfile != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("dockerfile", "", "may not be set when type is Git"))
		}
	case buildapi.BuildSourceDockerfile:
		if input.Dockerfile == nil {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("dockerfile"))
		} else {
			if len(*input.Dockerfile) > maxDockerfileLengthBytes {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("dockerfile", "", fmt.Sprintf("must be smaller than %d bytes", maxDockerfileLengthBytes)))
			}
		}
		if input.Git != nil {
			allErrs = append(allErrs, validateGitSource(input.Git).Prefix("git")...)
		}
	case "":
		allErrs = append(allErrs, fielderrors.NewFieldRequired("type"))
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
	if len(git.HTTPProxy) != 0 && !isValidURL(git.HTTPProxy) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("httpproxy", git.HTTPProxy, "proxy is not a valid url"))
	}
	if len(git.HTTPSProxy) != 0 && !isValidURL(git.HTTPSProxy) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("httpsproxy", git.HTTPSProxy, "proxy is not a valid url"))
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

func validateToImageReference(reference *kapi.ObjectReference) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	kind, name, namespace := reference.Kind, reference.Name, reference.Namespace
	switch kind {
	case "ImageStreamTag":
		if len(name) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
		} else if _, _, ok := imageapi.SplitImageStreamTag(name); !ok {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("name", name, "ImageStreamTag object references must be in the form <name>:<tag>"))
		}
		if len(namespace) != 0 && !kvalidation.IsDNS1123Subdomain(namespace) {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", namespace, "namespace must be a valid subdomain"))
		}

	case "DockerImage":
		if len(namespace) != 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", namespace, "namespace is not valid when used with a 'DockerImage'"))
		}
		if _, err := imageapi.ParseDockerImageReference(name); err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("name", name, fmt.Sprintf("name is not a valid Docker pull specification: %v", err)))
		}
	case "":
		allErrs = append(allErrs, fielderrors.NewFieldRequired("kind"))
	default:
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("kind", kind, "the target of build output must be an 'ImageStreamTag' or 'DockerImage'"))

	}
	return allErrs
}

func validateFromImageReference(reference *kapi.ObjectReference) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	kind, name, namespace := reference.Kind, reference.Name, reference.Namespace
	switch kind {
	case "ImageStreamTag":
		if len(name) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
		} else if _, _, ok := imageapi.SplitImageStreamTag(name); !ok {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("name", name, "ImageStreamTag object references must be in the form <name>:<tag>"))
		}

		if len(namespace) != 0 && !kvalidation.IsDNS1123Subdomain(namespace) {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", namespace, "namespace must be a valid subdomain"))
		}

	case "DockerImage":
		if len(namespace) != 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", namespace, "namespace is not valid when used with a 'DockerImage'"))
		}
		if len(name) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
		} else if _, err := imageapi.ParseDockerImageReference(name); err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("name", name, fmt.Sprintf("name is not a valid Docker pull specification: %v", err)))
		}
	case "ImageStreamImage":
		if len(name) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
		}
		if len(namespace) != 0 && !kvalidation.IsDNS1123Subdomain(namespace) {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("namespace", namespace, "namespace must be a valid subdomain"))
		}
	case "":
		allErrs = append(allErrs, fielderrors.NewFieldRequired("kind"))
	default:
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("kind", kind, "the source of a builder image must be an 'ImageStreamTag', 'ImageStreamImage', or 'DockerImage'"))

	}
	return allErrs
}

func validateOutput(output *buildapi.BuildOutput) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	// TODO: make part of a generic ValidateObjectReference method upstream.
	if output.To != nil {
		allErrs = append(allErrs, validateToImageReference(output.To).Prefix("to")...)
	}

	allErrs = append(allErrs, validateSecretRef(output.PushSecret).Prefix("pushSecret")...)

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
		if strategy.DockerStrategy == nil {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("dockerStrategy"))
		} else {
			allErrs = append(allErrs, validateDockerStrategy(strategy.DockerStrategy).Prefix("dockerStrategy")...)
		}

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

func validateDockerStrategy(strategy *buildapi.DockerBuildStrategy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if strategy.From != nil {
		allErrs = append(allErrs, validateFromImageReference(strategy.From).Prefix("from")...)
	}

	allErrs = append(allErrs, validateSecretRef(strategy.PullSecret).Prefix("pullSecret")...)
	return allErrs
}

func validateSourceStrategy(strategy *buildapi.SourceBuildStrategy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validateFromImageReference(&strategy.From).Prefix("from")...)
	allErrs = append(allErrs, validateSecretRef(strategy.PullSecret).Prefix("pullSecret")...)
	return allErrs
}

func validateCustomStrategy(strategy *buildapi.CustomBuildStrategy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validateFromImageReference(&strategy.From).Prefix("from")...)
	allErrs = append(allErrs, validateSecretRef(strategy.PullSecret).Prefix("pullSecret")...)
	return allErrs
}

func validateTrigger(trigger *buildapi.BuildTriggerPolicy) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(trigger.Type) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("type"))
		return allErrs
	}

	// Validate each trigger type
	switch trigger.Type {
	case buildapi.GitHubWebHookBuildTriggerType:
		if trigger.GitHubWebHook == nil {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("github"))
		} else {
			allErrs = append(allErrs, validateWebHook(trigger.GitHubWebHook).Prefix("github")...)
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
			break
		}
		if trigger.ImageChange.From == nil {
			break
		}
		if kind := trigger.ImageChange.From.Kind; kind != "ImageStreamTag" {
			invalidKindErr := fielderrors.NewFieldInvalid(
				"imageChange.from.kind",
				kind,
				"only an ImageStreamTag type of reference is allowed in an ImageChange trigger.")
			allErrs = append(allErrs, invalidKindErr)
			break
		}
		allErrs = append(allErrs, validateFromImageReference(trigger.ImageChange.From).Prefix("from")...)
	case buildapi.ConfigChangeBuildTriggerType:
		// doesn't require additional validation
	default:
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("type", trigger.Type, "invalid trigger type"))
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
