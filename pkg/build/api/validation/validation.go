package validation

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/util/labelselector"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/strategicpatch"
	kvalidation "k8s.io/kubernetes/pkg/util/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	oapi "github.com/openshift/origin/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/api/v1"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imageapivalidation "github.com/openshift/origin/pkg/image/api/validation"
)

// ValidateBuild tests required fields for a Build.
func ValidateBuild(build *buildapi.Build) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&build.ObjectMeta, true, validation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateCommonSpec(&build.Spec.CommonSpec, field.NewPath("spec"))...)

	// Check that all source image references are of DockerImage kind.
	strategyPath := field.NewPath("spec", "strategy")
	switch {
	case build.Spec.Strategy.SourceStrategy != nil:
		allErrs = append(allErrs, validateBuildImageReference(&build.Spec.Strategy.SourceStrategy.From, strategyPath.Child("sourceStrategy").Child("from"))...)
		allErrs = append(allErrs, validateBuildImageReference(build.Spec.Strategy.SourceStrategy.RuntimeImage, strategyPath.Child("sourceStrategy").Child("runtimeImage"))...)
	case build.Spec.Strategy.DockerStrategy != nil:
		allErrs = append(allErrs, validateBuildImageReference(build.Spec.Strategy.DockerStrategy.From, strategyPath.Child("dockerStrategy").Child("from"))...)
	case build.Spec.Strategy.CustomStrategy != nil:
		allErrs = append(allErrs, validateBuildImageReference(&build.Spec.Strategy.CustomStrategy.From, strategyPath.Child("customStrategy").Child("from"))...)
	}

	imagesPath := field.NewPath("spec", "source", "images")
	for i, image := range build.Spec.Source.Images {
		allErrs = append(allErrs, validateBuildImageReference(&image.From, imagesPath.Index(i).Child("from"))...)
	}

	return allErrs
}

func validateBuildImageReference(reference *kapi.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if reference != nil && reference.Kind != "DockerImage" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), reference.Kind, "only DockerImage references are supported for Builds"))
	}
	return allErrs
}

func ValidateBuildUpdate(build *buildapi.Build, older *buildapi.Build) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&build.ObjectMeta, &older.ObjectMeta, field.NewPath("metadata"))...)

	allErrs = append(allErrs, ValidateBuild(build)...)

	if buildutil.IsBuildComplete(older) && older.Status.Phase != build.Status.Phase {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status", "phase"), build.Status.Phase, "phase cannot be updated from a terminal state"))
	}
	if !kapi.Semantic.DeepEqual(build.Spec, older.Spec) {
		diff, err := diffBuildSpec(build.Spec, older.Spec)
		if err != nil {
			glog.V(2).Infof("Error calculating build spec patch: %v", err)
			diff = "[unknown]"
		}
		detail := fmt.Sprintf("spec is immutable, diff: %s", diff)
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec"), "content of spec is not printed out, please refer to the details", detail))
	}

	return allErrs
}

func diffBuildSpec(newer buildapi.BuildSpec, older buildapi.BuildSpec) (string, error) {
	codec := kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion)
	newerObj := &buildapi.Build{Spec: newer}
	olderObj := &buildapi.Build{Spec: older}

	newerJSON, err := runtime.Encode(codec, newerObj)
	if err != nil {
		return "", fmt.Errorf("error encoding newer: %v", err)
	}
	olderJSON, err := runtime.Encode(codec, olderObj)
	if err != nil {
		return "", fmt.Errorf("error encoding older: %v", err)
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(olderJSON, newerJSON, &v1.Build{})
	if err != nil {
		return "", fmt.Errorf("error creating a strategic patch: %v", err)
	}
	return string(patch), nil
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
func ValidateBuildConfig(config *buildapi.BuildConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&config.ObjectMeta, true, validation.NameIsDNSSubdomain, field.NewPath("metadata"))...)

	// image change triggers that refer
	fromRefs := map[string]struct{}{}
	specPath := field.NewPath("spec")
	triggersPath := specPath.Child("triggers")
	buildFrom := buildutil.GetInputReference(config.Spec.Strategy)
	for i, trg := range config.Spec.Triggers {
		allErrs = append(allErrs, validateTrigger(&trg, buildFrom, triggersPath.Index(i))...)
		if trg.Type != buildapi.ImageChangeBuildTriggerType || trg.ImageChange == nil {
			continue
		}
		from := trg.ImageChange.From
		if from == nil {
			from = buildFrom
		}
		fromKey := refKey(config.Namespace, from)
		_, exists := fromRefs[fromKey]
		if exists {
			allErrs = append(allErrs, field.Invalid(triggersPath, config.Spec.Triggers, "multiple ImageChange triggers refer to the same image stream tag"))
		}
		fromRefs[fromKey] = struct{}{}
	}

	switch config.Spec.RunPolicy {
	case buildapi.BuildRunPolicyParallel, buildapi.BuildRunPolicySerial, buildapi.BuildRunPolicySerialLatestOnly:
	default:
		allErrs = append(allErrs, field.Invalid(specPath.Child("runPolicy"), config.Spec.RunPolicy,
			"run policy must Parallel, Serial, or SerialLatestOnly"))
	}

	allErrs = append(allErrs, validateCommonSpec(&config.Spec.CommonSpec, specPath)...)

	return allErrs
}

func ValidateBuildConfigUpdate(config *buildapi.BuildConfig, older *buildapi.BuildConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&config.ObjectMeta, &older.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBuildConfig(config)...)
	return allErrs
}

// ValidateBuildRequest validates a BuildRequest object
func ValidateBuildRequest(request *buildapi.BuildRequest) field.ErrorList {
	return validation.ValidateObjectMeta(&request.ObjectMeta, true, oapi.MinimalNameRequirements, field.NewPath("metadata"))
}

func validateCommonSpec(spec *buildapi.CommonSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	s := spec.Strategy

	if s.DockerStrategy != nil && s.JenkinsPipelineStrategy == nil && spec.Source.Git == nil && spec.Source.Binary == nil && spec.Source.Dockerfile == nil && spec.Source.Images == nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("source"), "", "must provide a value for at least one source input(git, binary, dockerfile, images)."))
	}

	allErrs = append(allErrs,
		validateSource(
			&spec.Source,
			s.CustomStrategy != nil,
			s.DockerStrategy != nil,
			s.JenkinsPipelineStrategy != nil && len(s.JenkinsPipelineStrategy.Jenkinsfile) == 0,
			fldPath.Child("source"))...,
	)

	if spec.CompletionDeadlineSeconds != nil {
		if *spec.CompletionDeadlineSeconds <= 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("completionDeadlineSeconds"), spec.CompletionDeadlineSeconds, "completionDeadlineSeconds must be a positive integer greater than 0"))
		}
	}

	allErrs = append(allErrs, validateOutput(&spec.Output, fldPath.Child("output"))...)
	allErrs = append(allErrs, validateStrategy(&spec.Strategy, fldPath.Child("strategy"))...)
	allErrs = append(allErrs, validatePostCommit(spec.PostCommit, fldPath.Child("postCommit"))...)
	allErrs = append(allErrs, ValidateNodeSelector(spec.NodeSelector, fldPath.Child("nodeSelector"))...)

	// TODO: validate resource requirements (prereq: https://github.com/kubernetes/kubernetes/pull/7059)
	return allErrs
}

const (
	maxDockerfileLengthBytes  = 60 * 1000
	maxJenkinsfileLengthBytes = 100 * 1000
)

func hasProxy(source *buildapi.GitBuildSource) bool {
	return (source.HTTPProxy != nil && len(*source.HTTPProxy) > 0) || (source.HTTPSProxy != nil && len(*source.HTTPSProxy) > 0)
}

func validateSource(input *buildapi.BuildSource, isCustomStrategy, isDockerStrategy, isJenkinsPipelineStrategyFromRepo bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure that Git and Binary source types are mutually exclusive.
	if input.Git != nil && input.Binary != nil && !isCustomStrategy {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("git"), "", "may not be set when binary is also set"))
		allErrs = append(allErrs, field.Invalid(fldPath.Child("binary"), "", "may not be set when git is also set"))
		return allErrs
	}

	// Validate individual source type details
	if input.Git != nil {
		allErrs = append(allErrs, validateGitSource(input.Git, fldPath.Child("git"))...)
	}
	if input.Binary != nil {
		allErrs = append(allErrs, validateBinarySource(input.Binary, fldPath.Child("binary"))...)
	}
	if input.Dockerfile != nil {
		allErrs = append(allErrs, validateDockerfile(*input.Dockerfile, fldPath.Child("dockerfile"))...)
	}
	if input.Images != nil {
		for i, image := range input.Images {
			allErrs = append(allErrs, validateImageSource(image, fldPath.Child("images").Index(i))...)
		}
	}
	if isJenkinsPipelineStrategyFromRepo && input.Git == nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("git"), "", "must be set when using Jenkins Pipeline strategy with Jenkinsfile from a git repo"))
	}

	allErrs = append(allErrs, validateSecrets(input.Secrets, isDockerStrategy, fldPath.Child("secrets"))...)

	allErrs = append(allErrs, validateSecretRef(input.SourceSecret, fldPath.Child("sourceSecret"))...)

	if len(input.ContextDir) != 0 {
		cleaned := path.Clean(input.ContextDir)
		if strings.HasPrefix(cleaned, "..") {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("contextDir"), input.ContextDir, "context dir must not be a relative path"))
		} else {
			if cleaned == "." {
				cleaned = ""
			}
			input.ContextDir = cleaned
		}
	}

	return allErrs
}

func validateDockerfile(dockerfile string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(dockerfile) > maxDockerfileLengthBytes {
		allErrs = append(allErrs, field.Invalid(fldPath, "", fmt.Sprintf("must be smaller than %d bytes", maxDockerfileLengthBytes)))
	}
	return allErrs
}

func validateSecretRef(ref *kapi.LocalObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if ref == nil {
		return allErrs
	}
	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
	}
	return allErrs
}

func isHTTPScheme(in string) bool {
	u, err := url.Parse(in)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func validateGitSource(git *buildapi.GitBuildSource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(git.URI) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("uri"), ""))
	} else if !IsValidURL(git.URI) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("uri"), git.URI, "uri is not a valid url"))
	}
	if git.HTTPProxy != nil && len(*git.HTTPProxy) != 0 && !IsValidURL(*git.HTTPProxy) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("httpproxy"), *git.HTTPProxy, "proxy is not a valid url"))
	}
	if git.HTTPSProxy != nil && len(*git.HTTPSProxy) != 0 && !IsValidURL(*git.HTTPSProxy) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("httpsproxy"), *git.HTTPSProxy, "proxy is not a valid url"))
	}
	if hasProxy(git) && !isHTTPScheme(git.URI) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("uri"), git.URI, "only http:// and https:// GIT protocols are allowed with HTTP or HTTPS proxy set"))
	}
	return allErrs
}

func validateSecrets(secrets []buildapi.SecretBuildSource, isDockerStrategy bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, s := range secrets {
		if len(s.Secret.Name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("secret"), ""))
		}
		if reasons := validation.ValidateSecretName(s.Secret.Name, false); len(reasons) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("secret"), s, "must be valid secret name"))
		}
		if strings.HasPrefix(path.Clean(s.DestinationDir), "..") {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("destinationDir"), s.DestinationDir, "destination dir cannot start with '..'"))
		}
		if isDockerStrategy && filepath.IsAbs(s.DestinationDir) {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("destinationDir"), s.DestinationDir, "for the docker strategy the destinationDir has to be relative path"))
		}
	}
	return allErrs
}

func validateImageSource(imageSource buildapi.ImageSource, fldPath *field.Path) field.ErrorList {
	allErrs := validateFromImageReference(&imageSource.From, fldPath.Child("from"))
	if imageSource.PullSecret != nil {
		allErrs = append(allErrs, validateSecretRef(imageSource.PullSecret, fldPath.Child("pullSecret"))...)
	}
	if len(imageSource.Paths) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("paths"), ""))
	}
	for i, path := range imageSource.Paths {
		allErrs = append(allErrs, validateImageSourcePath(path, fldPath.Child("paths").Index(i))...)

	}
	return allErrs
}

func validateImageSourcePath(imagePath buildapi.ImageSourcePath, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(imagePath.SourcePath) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("sourcePath"), ""))
	}
	if len(imagePath.DestinationDir) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("destinationDir"), ""))
	}
	if !filepath.IsAbs(imagePath.SourcePath) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("sourcePath"), imagePath.SourcePath, "must be an absolute path"))
	}
	if filepath.IsAbs(imagePath.DestinationDir) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("destinationDir"), imagePath.DestinationDir, "must be a relative path"))
	}
	if strings.HasPrefix(path.Clean(imagePath.DestinationDir), "..") {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("destinationDir"), imagePath.DestinationDir, "destination dir cannot start with '..'"))
	}
	return allErrs
}

func validateBinarySource(source *buildapi.BinaryBuildSource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(source.AsFile) != 0 {
		cleaned := strings.TrimPrefix(path.Clean(source.AsFile), "/")
		if len(cleaned) == 0 || cleaned == "." || strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "/") || strings.Contains(cleaned, "\\") {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("asFile"), source.AsFile, "file name may not contain slashes or relative path segments and must be a valid POSIX filename"))
		} else {
			source.AsFile = cleaned
		}
	}
	return allErrs
}

func validateToImageReference(reference *kapi.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	kind, name, namespace := reference.Kind, reference.Name, reference.Namespace
	switch kind {
	case "ImageStreamTag":
		if len(name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
		} else if _, _, ok := imageapi.SplitImageStreamTag(name); !ok {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), name, "ImageStreamTag object references must be in the form <name>:<tag>"))
		} else if name, _, _ := imageapi.SplitImageStreamTag(name); len(imageapivalidation.ValidateImageStreamName(name, false)) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), name, "ImageStreamTag name contains invalid syntax"))
		}
		if len(namespace) != 0 && len(kvalidation.IsDNS1123Subdomain(namespace)) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), namespace, "namespace must be a valid subdomain"))
		}

	case "DockerImage":
		if len(namespace) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), namespace, "namespace is not valid when used with a 'DockerImage'"))
		}
		if _, err := imageapi.ParseDockerImageReference(name); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), name, fmt.Sprintf("name is not a valid Docker pull specification: %v", err)))
		}
	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), ""))
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), kind, "the target of build output must be an 'ImageStreamTag' or 'DockerImage'"))

	}
	return allErrs
}

func validateFromImageReference(reference *kapi.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	kind, name, namespace := reference.Kind, reference.Name, reference.Namespace
	switch kind {
	case "ImageStreamTag":
		if len(name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
		} else if _, _, ok := imageapi.SplitImageStreamTag(name); !ok {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), name, "ImageStreamTag object references must be in the form <name>:<tag>"))
		} else if name, _, _ := imageapi.SplitImageStreamTag(name); len(imageapivalidation.ValidateImageStreamName(name, false)) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), name, "ImageStreamTag name contains invalid syntax"))
		}

		if len(namespace) != 0 && len(kvalidation.IsDNS1123Subdomain(namespace)) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), namespace, "namespace must be a valid subdomain"))
		}

	case "DockerImage":
		if len(namespace) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), namespace, "namespace is not valid when used with a 'DockerImage'"))
		}
		if len(name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
		} else if _, err := imageapi.ParseDockerImageReference(name); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), name, fmt.Sprintf("name is not a valid Docker pull specification: %v", err)))
		}
	case "ImageStreamImage":
		if len(name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
		}
		if len(namespace) != 0 && len(kvalidation.IsDNS1123Subdomain(namespace)) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), namespace, "namespace must be a valid subdomain"))
		}
	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), ""))
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), kind, "the source of a builder image must be an 'ImageStreamTag', 'ImageStreamImage', or 'DockerImage'"))

	}
	return allErrs
}

func validateOutput(output *buildapi.BuildOutput, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// TODO: make part of a generic ValidateObjectReference method upstream.
	if output.To != nil {
		allErrs = append(allErrs, validateToImageReference(output.To, fldPath.Child("to"))...)
	}

	allErrs = append(allErrs, validateSecretRef(output.PushSecret, fldPath.Child("pushSecret"))...)
	allErrs = append(allErrs, ValidateImageLabels(output.ImageLabels, fldPath.Child("imageLabels"))...)

	return allErrs
}

func validateStrategy(strategy *buildapi.BuildStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	strategyCount := 0
	if strategy.SourceStrategy != nil {
		strategyCount++
	}
	if strategy.DockerStrategy != nil {
		strategyCount++
	}
	if strategy.CustomStrategy != nil {
		strategyCount++
	}
	if strategy.JenkinsPipelineStrategy != nil {
		strategyCount++
	}
	if strategyCount != 1 {
		return append(allErrs, field.Invalid(fldPath, strategy, "must provide a value for exactly one of sourceStrategy, customStrategy, dockerStrategy, or jenkinsPipelineStrategy"))
	}

	if strategy.SourceStrategy != nil {
		allErrs = append(allErrs, validateSourceStrategy(strategy.SourceStrategy, fldPath.Child("sourceStrategy"))...)
	}
	if strategy.DockerStrategy != nil {
		allErrs = append(allErrs, validateDockerStrategy(strategy.DockerStrategy, fldPath.Child("dockerStrategy"))...)
	}
	if strategy.CustomStrategy != nil {
		allErrs = append(allErrs, validateCustomStrategy(strategy.CustomStrategy, fldPath.Child("customStrategy"))...)
	}
	if strategy.JenkinsPipelineStrategy != nil {
		allErrs = append(allErrs, validateJenkinsPipelineStrategy(strategy.JenkinsPipelineStrategy, fldPath.Child("jenkinsPipelineStrategy"))...)
	}

	return allErrs
}

func validateDockerStrategy(strategy *buildapi.DockerBuildStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if strategy.From != nil {
		allErrs = append(allErrs, validateFromImageReference(strategy.From, fldPath.Child("from"))...)
	}

	allErrs = append(allErrs, validateSecretRef(strategy.PullSecret, fldPath.Child("pullSecret"))...)

	if len(strategy.DockerfilePath) != 0 {
		cleaned, errs := validateRelativePath(strategy.DockerfilePath, "dockerfilePath", fldPath.Child("dockerfilePath"))
		allErrs = append(allErrs, errs...)
		if len(errs) == 0 {
			strategy.DockerfilePath = cleaned
		}
	}

	allErrs = append(allErrs, ValidateStrategyEnv(strategy.Env, fldPath.Child("env"))...)

	return allErrs
}

func validateRelativePath(filePath, fieldName string, fldPath *field.Path) (string, field.ErrorList) {
	allErrs := field.ErrorList{}
	cleaned := path.Clean(filePath)
	switch {
	case filepath.IsAbs(cleaned), cleaned == "..", strings.HasPrefix(cleaned, "../"):
		allErrs = append(allErrs, field.Invalid(fldPath, filePath, fieldName+" must be a relative path within your source location"))
	default:
		if cleaned == "." {
			cleaned = ""
		}
	}
	return cleaned, allErrs
}

func validateSourceStrategy(strategy *buildapi.SourceBuildStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateFromImageReference(&strategy.From, fldPath.Child("from"))...)
	allErrs = append(allErrs, validateSecretRef(strategy.PullSecret, fldPath.Child("pullSecret"))...)
	allErrs = append(allErrs, ValidateStrategyEnv(strategy.Env, fldPath.Child("env"))...)
	allErrs = append(allErrs, validateRuntimeImage(strategy, fldPath.Child("runtimeImage"))...)
	return allErrs
}

func validateCustomStrategy(strategy *buildapi.CustomBuildStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateFromImageReference(&strategy.From, fldPath.Child("from"))...)
	allErrs = append(allErrs, validateSecretRef(strategy.PullSecret, fldPath.Child("pullSecret"))...)
	allErrs = append(allErrs, ValidateStrategyEnv(strategy.Env, fldPath.Child("env"))...)
	return allErrs
}

func validateJenkinsPipelineStrategy(strategy *buildapi.JenkinsPipelineBuildStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(strategy.JenkinsfilePath) != 0 && len(strategy.Jenkinsfile) != 0 {
		return append(allErrs, field.Invalid(fldPath, strategy, "must provide a value for at most one of jenkinsfilePath, or jenkinsfile"))
	}

	if len(strategy.Jenkinsfile) > maxJenkinsfileLengthBytes {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("jenkinsfile"), "", fmt.Sprintf("must be smaller than %d bytes", maxJenkinsfileLengthBytes)))
	}

	if len(strategy.JenkinsfilePath) != 0 {
		cleaned, errs := validateRelativePath(strategy.JenkinsfilePath, "jenkinsfilePath", fldPath.Child("jenkinsfilePath"))
		allErrs = append(allErrs, errs...)
		if len(errs) == 0 {
			strategy.JenkinsfilePath = cleaned
		}
	}

	return allErrs
}

func validateTrigger(trigger *buildapi.BuildTriggerPolicy, buildFrom *kapi.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(trigger.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), ""))
		return allErrs
	}

	// Validate each trigger type
	switch trigger.Type {
	case buildapi.GitHubWebHookBuildTriggerType:
		if trigger.GitHubWebHook == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("github"), ""))
		} else {
			allErrs = append(allErrs, validateWebHook(trigger.GitHubWebHook, fldPath.Child("github"), false)...)
		}
	case buildapi.GenericWebHookBuildTriggerType:
		if trigger.GenericWebHook == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("generic"), ""))
		} else {
			allErrs = append(allErrs, validateWebHook(trigger.GenericWebHook, fldPath.Child("generic"), true)...)
		}
	case buildapi.ImageChangeBuildTriggerType:
		if trigger.ImageChange == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("imageChange"), ""))
			break
		}
		if trigger.ImageChange.From == nil {
			if buildFrom == nil || buildFrom.Kind != "ImageStreamTag" {
				invalidKindErr := field.Invalid(
					fldPath.Child("imageChange"),
					fmt.Sprintf("build from: %v", buildFrom),
					"a default ImageChange trigger can only be used when the build strategy includes an ImageStreamTag reference.")
				allErrs = append(allErrs, invalidKindErr)
				break
			}

			break
		}
		if kind := trigger.ImageChange.From.Kind; kind != "ImageStreamTag" {
			invalidKindErr := field.Invalid(
				fldPath.Child("imageChange").Child("from").Child("kind"),
				kind,
				"only an ImageStreamTag type of reference is allowed in an ImageChange trigger.")
			allErrs = append(allErrs, invalidKindErr)
			break
		}
		allErrs = append(allErrs, validateFromImageReference(trigger.ImageChange.From, fldPath.Child("from"))...)
	case buildapi.ConfigChangeBuildTriggerType:
		// doesn't require additional validation
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), trigger.Type, "invalid trigger type"))
	}
	return allErrs
}

func validateWebHook(webHook *buildapi.WebHookTrigger, fldPath *field.Path, isGeneric bool) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(webHook.Secret) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("secret"), ""))
	}
	if !isGeneric && webHook.AllowEnv {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("allowEnv"), webHook, "git webhooks cannot allow env vars"))
	}
	return allErrs
}

func IsValidURL(uri string) bool {
	_, err := url.Parse(uri)
	return err == nil
}

func ValidateBuildLogOptions(opts *buildapi.BuildLogOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	// TODO: Replace by validating PodLogOptions via BuildLogOptions once it's bundled in
	popts := buildapi.BuildToPodLogOptions(opts)
	if errs := validation.ValidatePodLogOptions(popts); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if opts.Version != nil && *opts.Version <= 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("version"), *opts.Version, "build version must be greater than 0"))
	}
	if opts.Version != nil && opts.Previous {
		allErrs = append(allErrs, field.Invalid(field.NewPath("previous"), opts.Previous, "cannot use previous when a version is specified"))
	}
	return allErrs
}

var cIdentifierErrorMsg string = `must be a C identifier (mixed-case letters, numbers, and underscores): e.g. "my_name" or "MyName"`

func ValidateStrategyEnv(vars []kapi.EnvVar, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, ev := range vars {
		idxPath := fldPath.Index(i)
		if len(ev.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), ""))
		} else if len(kvalidation.IsCIdentifier(ev.Name)) > 0 {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("name"), ev.Name, cIdentifierErrorMsg))
		}
		if ev.ValueFrom != nil {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("valueFrom"), ev.ValueFrom, "valueFrom is not supported in build strategy environment variables"))
		}
	}
	return allErrs
}

func validatePostCommit(spec buildapi.BuildPostCommitSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if spec.Script != "" && len(spec.Command) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, spec, "cannot use command and script together"))
	}
	return allErrs
}

// validateRuntimeImage verifies that the runtimeImage field in
// SourceBuildStrategy is not empty if it was specified and also checks to see
// if the incremental build flag was specified, which is incompatible since we
// can't have extended incremental builds.
func validateRuntimeImage(sourceStrategy *buildapi.SourceBuildStrategy, fldPath *field.Path) (allErrs field.ErrorList) {
	if sourceStrategy.RuntimeImage == nil {
		return
	}
	if sourceStrategy.RuntimeImage.Name == "" {
		return append(allErrs, field.Required(fldPath, "name"))
	}

	if sourceStrategy.Incremental != nil && *sourceStrategy.Incremental {
		return append(allErrs, field.Invalid(fldPath, sourceStrategy.Incremental, "incremental cannot be set to true with extended builds"))
	}
	return
}

func ValidateImageLabels(labels []buildapi.ImageLabel, fldPath *field.Path) (allErrs field.ErrorList) {
	for i, lbl := range labels {
		idxPath := fldPath.Index(i)
		if len(lbl.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), ""))
			continue
		}
		for _, msg := range kvalidation.IsConfigMapKey(lbl.Name) {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("name"), lbl.Name, msg))
		}
	}

	// find duplicates
	seen := make(map[string]bool)
	for i, lbl := range labels {
		idxPath := fldPath.Index(i)
		if seen[lbl.Name] {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("name"), lbl.Name, "duplicate name"))
			continue
		}
		seen[lbl.Name] = true
	}

	return
}

func ValidateNodeSelector(nodeSelector map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for k, v := range nodeSelector {
		_, err := labelselector.Parse(fmt.Sprintf("%s=%s", k, v))
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(k),
				nodeSelector[k], "must be a valid node selector"))
		}
	}
	return allErrs
}
