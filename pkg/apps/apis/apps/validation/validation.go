package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/validation"
	kapivalidation "k8s.io/kubernetes/pkg/apis/core/validation"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageval "github.com/openshift/origin/pkg/image/apis/image/validation"
)

func ValidateDeploymentConfig(config *appsapi.DeploymentConfig) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&config.ObjectMeta, true, validation.NameIsDNSSubdomain, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateDeploymentConfigSpec(config.Spec)...)
	allErrs = append(allErrs, ValidateDeploymentConfigStatus(config.Status)...)
	return allErrs
}

func ValidateDeploymentConfigSpec(spec appsapi.DeploymentConfigSpec) field.ErrorList {
	allErrs := field.ErrorList{}
	specPath := field.NewPath("spec")
	for i := range spec.Triggers {
		allErrs = append(allErrs, validateTrigger(&spec.Triggers[i], specPath.Child("triggers").Index(i))...)
	}

	var podSpec *kapi.PodSpec
	if spec.Template != nil {
		podSpec = &spec.Template.Spec
	}

	// gate calls to validation functions that depend on a podSpec
	// if we have don't have an actual podSpec here.
	if podSpec != nil {
		allErrs = append(allErrs, validateDeploymentStrategy(&spec.Strategy, podSpec, specPath.Child("strategy"))...)
	}

	if spec.RevisionHistoryLimit != nil {
		allErrs = append(allErrs, kapivalidation.ValidateNonnegativeField(int64(*spec.RevisionHistoryLimit), specPath.Child("revisionHistoryLimit"))...)
	}
	allErrs = append(allErrs, kapivalidation.ValidateNonnegativeField(int64(spec.MinReadySeconds), specPath.Child("minReadySeconds"))...)
	timeoutSeconds := appsapi.DefaultRollingTimeoutSeconds
	if spec.Strategy.RollingParams != nil && spec.Strategy.RollingParams.TimeoutSeconds != nil {
		timeoutSeconds = *(spec.Strategy.RollingParams.TimeoutSeconds)
	} else if spec.Strategy.RecreateParams != nil && spec.Strategy.RecreateParams.TimeoutSeconds != nil {
		timeoutSeconds = *(spec.Strategy.RecreateParams.TimeoutSeconds)
	}
	if timeoutSeconds > 0 && int64(spec.MinReadySeconds) >= timeoutSeconds {
		allErrs = append(allErrs, field.Invalid(specPath.Child("minReadySeconds"), spec.MinReadySeconds, fmt.Sprintf("must be less than the deployment timeout (%ds)", timeoutSeconds)))
	}
	if spec.Strategy.ActiveDeadlineSeconds != nil && int64(spec.MinReadySeconds) >= *(spec.Strategy.ActiveDeadlineSeconds) {
		allErrs = append(allErrs, field.Invalid(specPath.Child("minReadySeconds"), spec.MinReadySeconds, fmt.Sprintf("must be less than activeDeadlineSeconds (%ds - used by the deployer pod)", *(spec.Strategy.ActiveDeadlineSeconds))))
	}
	if spec.Template == nil {
		allErrs = append(allErrs, field.Required(specPath.Child("template"), ""))
	} else {
		originalContainerImageNames := getContainerImageNames(spec.Template)
		defer setContainerImageNames(spec.Template, originalContainerImageNames)
		handleEmptyImageReferences(spec.Template, spec.Triggers)
		allErrs = append(allErrs, validation.ValidatePodTemplateSpecForRC(spec.Template, spec.Selector, spec.Replicas, specPath.Child("template"))...)
	}
	if spec.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("replicas"), spec.Replicas, "replicas cannot be negative"))
	}
	if len(spec.Selector) == 0 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("selector"), spec.Selector, "selector cannot be empty"))
	}
	return allErrs
}

func getContainerImageNames(template *kapi.PodTemplateSpec) []string {
	originalContainerImageNames := make([]string, len(template.Spec.Containers))
	for i := range template.Spec.Containers {
		originalContainerImageNames[i] = template.Spec.Containers[i].Image
	}
	return originalContainerImageNames
}

func setContainerImageNames(template *kapi.PodTemplateSpec, originalNames []string) {
	for i := range template.Spec.Containers {
		template.Spec.Containers[i].Image = originalNames[i]
	}
}

func handleEmptyImageReferences(template *kapi.PodTemplateSpec, triggers []appsapi.DeploymentTriggerPolicy) {
	// if we have both an ICT defined and an empty Template->PodSpec->Container->Image field, we are going
	// to modify this method's local copy (a pointer was NOT used for the parameter) by setting the field to a non-empty value to
	// work around the k8s validation as our ICT will supply the image field value
	containerEmptyImageInICT := make(map[string]bool)
	for _, container := range template.Spec.Containers {
		if len(container.Image) == 0 {
			containerEmptyImageInICT[container.Name] = false
		}
	}

	if len(containerEmptyImageInICT) == 0 {
		return
	}

	needToChangeImageField := false
	for _, trigger := range triggers {
		// note, the validateTrigger call above will add an error if ImageChangeParams is nil, but
		// we can still fall down this path so account for it being nil
		if trigger.Type != appsapi.DeploymentTriggerOnImageChange || trigger.ImageChangeParams == nil {
			continue
		}

		for _, container := range trigger.ImageChangeParams.ContainerNames {
			if _, ok := containerEmptyImageInICT[container]; ok {
				needToChangeImageField = true
				containerEmptyImageInICT[container] = true
			}
		}
	}

	if needToChangeImageField {
		for i, container := range template.Spec.Containers {
			// only update containers listed in the ict
			match, ok := containerEmptyImageInICT[container.Name]
			if match && ok {
				template.Spec.Containers[i].Image = "unset"
			}
		}
	}

}

func ValidateDeploymentConfigStatus(status appsapi.DeploymentConfigStatus) field.ErrorList {
	allErrs := field.ErrorList{}
	statusPath := field.NewPath("status")
	allErrs = append(allErrs, kapivalidation.ValidateNonnegativeField(int64(status.LatestVersion), statusPath.Child("latestVersion"))...)
	allErrs = append(allErrs, kapivalidation.ValidateNonnegativeField(int64(status.ObservedGeneration), statusPath.Child("observedGeneration"))...)
	allErrs = append(allErrs, kapivalidation.ValidateNonnegativeField(int64(status.Replicas), statusPath.Child("replicas"))...)
	allErrs = append(allErrs, kapivalidation.ValidateNonnegativeField(int64(status.ReadyReplicas), statusPath.Child("readyReplicas"))...)
	allErrs = append(allErrs, kapivalidation.ValidateNonnegativeField(int64(status.AvailableReplicas), statusPath.Child("availableReplicas"))...)
	allErrs = append(allErrs, kapivalidation.ValidateNonnegativeField(int64(status.UnavailableReplicas), statusPath.Child("unavailableReplicas"))...)
	return allErrs
}

func ValidateDeploymentConfigUpdate(newConfig *appsapi.DeploymentConfig, oldConfig *appsapi.DeploymentConfig) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newConfig.ObjectMeta, &oldConfig.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateDeploymentConfig(newConfig)...)
	allErrs = append(allErrs, ValidateDeploymentConfigStatusUpdate(newConfig, oldConfig)...)
	return allErrs
}

func ValidateDeploymentConfigStatusUpdate(newConfig *appsapi.DeploymentConfig, oldConfig *appsapi.DeploymentConfig) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newConfig.ObjectMeta, &oldConfig.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateDeploymentConfigStatus(newConfig.Status)...)
	statusPath := field.NewPath("status")
	if newConfig.Status.LatestVersion < oldConfig.Status.LatestVersion {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("latestVersion"), newConfig.Status.LatestVersion, "latestVersion cannot be decremented"))
	} else if newConfig.Status.LatestVersion > (oldConfig.Status.LatestVersion + 1) {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("latestVersion"), newConfig.Status.LatestVersion, "latestVersion can only be incremented by 1"))
	}
	if newConfig.Status.ObservedGeneration < oldConfig.Status.ObservedGeneration {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("observedGeneration"), newConfig.Status.ObservedGeneration, "observedGeneration cannot be decremented"))
	}
	return allErrs
}

func ValidateDeploymentConfigRollback(rollback *appsapi.DeploymentConfigRollback) field.ErrorList {
	result := field.ErrorList{}

	if len(rollback.Name) == 0 {
		result = append(result, field.Required(field.NewPath("name"), "name of the deployment config is missing"))
	} else if len(kvalidation.IsDNS1123Subdomain(rollback.Name)) != 0 {
		result = append(result, field.Invalid(field.NewPath("name"), rollback.Name, "name of the deployment config is invalid"))
	}

	specPath := field.NewPath("spec")
	if rollback.Spec.Revision < 0 {
		result = append(result, field.Invalid(specPath.Child("revision"), rollback.Spec.Revision, "must be non-negative"))
	}

	return result
}

func ValidateDeploymentConfigRollbackDeprecated(rollback *appsapi.DeploymentConfigRollback) field.ErrorList {
	result := field.ErrorList{}

	fromPath := field.NewPath("spec", "from")
	if len(rollback.Spec.From.Name) == 0 {
		result = append(result, field.Required(fromPath.Child("name"), ""))
	}

	if len(rollback.Spec.From.Kind) == 0 {
		rollback.Spec.From.Kind = "ReplicationController"
	}

	if rollback.Spec.From.Kind != "ReplicationController" {
		result = append(result, field.Invalid(fromPath.Child("kind"), rollback.Spec.From.Kind, "the kind of the rollback target must be 'ReplicationController'"))
	}

	return result
}

func validateDeploymentStrategy(strategy *appsapi.DeploymentStrategy, pod *kapi.PodSpec, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if len(strategy.Type) == 0 {
		errs = append(errs, field.Required(fldPath.Child("type"), ""))
	}

	if strategy.CustomParams != nil {
		errs = append(errs, validateCustomParams(strategy.CustomParams, fldPath.Child("customParams"))...)
	}

	switch strategy.Type {
	case appsapi.DeploymentStrategyTypeRecreate:
		if strategy.RecreateParams != nil {
			errs = append(errs, validateRecreateParams(strategy.RecreateParams, pod, fldPath.Child("recreateParams"))...)
		}
	case appsapi.DeploymentStrategyTypeRolling:
		if strategy.RollingParams == nil {
			errs = append(errs, field.Required(fldPath.Child("rollingParams"), ""))
		} else {
			errs = append(errs, validateRollingParams(strategy.RollingParams, pod, fldPath.Child("rollingParams"))...)
		}
	case appsapi.DeploymentStrategyTypeCustom:
		if strategy.CustomParams == nil {
			errs = append(errs, field.Required(fldPath.Child("customParams"), ""))
		}
		if strategy.RollingParams != nil {
			errs = append(errs, validateRollingParams(strategy.RollingParams, pod, fldPath.Child("rollingParams"))...)
		}
		if strategy.RecreateParams != nil {
			errs = append(errs, validateRecreateParams(strategy.RecreateParams, pod, fldPath.Child("recreateParams"))...)
		}
	case "":
		errs = append(errs, field.Required(fldPath.Child("type"), "strategy type is required"))
	default:
		errs = append(errs, field.Invalid(fldPath.Child("type"), strategy.Type, "unsupported strategy type, use \"Custom\" instead and specify your own strategy"))
	}

	if strategy.Labels != nil {
		errs = append(errs, unversionedvalidation.ValidateLabels(strategy.Labels, fldPath.Child("labels"))...)
	}
	if strategy.Annotations != nil {
		errs = append(errs, validation.ValidateAnnotations(strategy.Annotations, fldPath.Child("annotations"))...)
	}

	errs = append(errs, validation.ValidateResourceRequirements(&strategy.Resources, fldPath.Child("resources"))...)

	if strategy.ActiveDeadlineSeconds != nil {
		errs = append(errs, kapivalidation.ValidateNonnegativeField(*strategy.ActiveDeadlineSeconds, fldPath.Child("activeDeadlineSeconds"))...)
		var timeoutSeconds *int64
		if strategy.RollingParams != nil {
			timeoutSeconds = strategy.RollingParams.TimeoutSeconds
		} else if strategy.RecreateParams != nil {
			timeoutSeconds = strategy.RecreateParams.TimeoutSeconds
		}
		if timeoutSeconds != nil && *strategy.ActiveDeadlineSeconds <= *timeoutSeconds {
			errs = append(errs, field.Invalid(fldPath.Child("activeDeadlineSeconds"), *strategy.ActiveDeadlineSeconds, "activeDeadlineSeconds must be greater than timeoutSeconds"))
		}
	}

	return errs
}

func validateCustomParams(params *appsapi.CustomDeploymentStrategyParams, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	errs = append(errs, validateEnv(params.Environment, fldPath.Child("environment"))...)

	return errs
}

func validateRecreateParams(params *appsapi.RecreateDeploymentStrategyParams, pod *kapi.PodSpec, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if params.TimeoutSeconds != nil && *params.TimeoutSeconds < 1 {
		errs = append(errs, field.Invalid(fldPath.Child("timeoutSeconds"), *params.TimeoutSeconds, "must be >0"))
	}

	if params.Pre != nil {
		errs = append(errs, validateLifecycleHook(params.Pre, pod, fldPath.Child("pre"))...)
	}
	if params.Mid != nil {
		errs = append(errs, validateLifecycleHook(params.Mid, pod, fldPath.Child("mid"))...)
	}
	if params.Post != nil {
		errs = append(errs, validateLifecycleHook(params.Post, pod, fldPath.Child("post"))...)
	}

	return errs
}

func validateLifecycleHook(hook *appsapi.LifecycleHook, pod *kapi.PodSpec, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if len(hook.FailurePolicy) == 0 {
		errs = append(errs, field.Required(fldPath.Child("failurePolicy"), ""))
	}

	switch {
	case hook.ExecNewPod != nil && len(hook.TagImages) > 0:
		errs = append(errs, field.Invalid(fldPath, "<hook>", "only one of 'execNewPod' or 'tagImages' may be specified"))
	case hook.ExecNewPod != nil:
		errs = append(errs, validateExecNewPod(hook.ExecNewPod, fldPath.Child("execNewPod"))...)
	case len(hook.TagImages) > 0:
		for i, image := range hook.TagImages {
			if len(image.ContainerName) == 0 {
				errs = append(errs, field.Required(fldPath.Child("tagImages").Index(i).Child("containerName"), "a containerName is required"))
			} else {
				if _, err := appsapi.TemplateImageForContainer(pod, appsapi.IgnoreTriggers, image.ContainerName); err != nil {
					errs = append(errs, field.Invalid(fldPath.Child("tagImages").Index(i).Child("containerName"), image.ContainerName, err.Error()))
				}
			}
			if image.To.Kind != "ImageStreamTag" {
				errs = append(errs, field.Invalid(fldPath.Child("tagImages").Index(i).Child("to", "kind"), image.To.Kind, "Must be 'ImageStreamTag'"))
			}
			if len(image.To.Name) == 0 {
				errs = append(errs, field.Required(fldPath.Child("tagImages").Index(i).Child("to", "name"), "a destination tag name is required"))
			}
		}
	default:
		errs = append(errs, field.Invalid(fldPath, "<empty>", "One of execNewPod or tagImages must be specified"))
	}

	return errs
}

func validateExecNewPod(hook *appsapi.ExecNewPodHook, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if len(hook.Command) == 0 {
		errs = append(errs, field.Required(fldPath.Child("command"), ""))
	}

	if len(hook.ContainerName) == 0 {
		errs = append(errs, field.Required(fldPath.Child("containerName"), ""))
	}

	if len(hook.Env) > 0 {
		errs = append(errs, validateEnv(hook.Env, fldPath.Child("env"))...)
	}

	errs = append(errs, validateHookVolumes(hook.Volumes, fldPath.Child("volumes"))...)

	return errs
}

func validateEnv(vars []kapi.EnvVar, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, ev := range vars {
		vErrs := field.ErrorList{}
		idxPath := fldPath.Index(i).Child("name")
		if len(ev.Name) == 0 {
			vErrs = append(vErrs, field.Required(idxPath, ""))
		}
		if errs := kvalidation.IsCIdentifier(ev.Name); len(errs) > 0 {
			vErrs = append(vErrs, field.Invalid(idxPath, ev.Name, strings.Join(errs, ", ")))
		}
		allErrs = append(allErrs, vErrs...)
	}
	return allErrs
}

func validateHookVolumes(volumes []string, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	for i, vol := range volumes {
		vErrs := field.ErrorList{}
		if len(vol) == 0 {
			vErrs = append(vErrs, field.Invalid(fldPath.Index(i), "", "must not be empty"))
		}
		errs = append(errs, vErrs...)
	}
	return errs
}

func validateRollingParams(params *appsapi.RollingDeploymentStrategyParams, pod *kapi.PodSpec, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if params.IntervalSeconds != nil && *params.IntervalSeconds < 1 {
		errs = append(errs, field.Invalid(fldPath.Child("intervalSeconds"), *params.IntervalSeconds, "must be >0"))
	}

	if params.UpdatePeriodSeconds != nil && *params.UpdatePeriodSeconds < 1 {
		errs = append(errs, field.Invalid(fldPath.Child("updatePeriodSeconds"), *params.UpdatePeriodSeconds, "must be >0"))
	}

	if params.TimeoutSeconds != nil && *params.TimeoutSeconds < 1 {
		errs = append(errs, field.Invalid(fldPath.Child("timeoutSeconds"), *params.TimeoutSeconds, "must be >0"))
	}

	// Most of this is lifted from the upstream experimental deployments API. We
	// can't reuse it directly yet, but no use reinventing the logic, so copy-
	// pasted and adapted here.
	errs = append(errs, ValidatePositiveIntOrPercent(params.MaxUnavailable, fldPath.Child("maxUnavailable"))...)
	errs = append(errs, ValidatePositiveIntOrPercent(params.MaxSurge, fldPath.Child("maxSurge"))...)
	if getIntOrPercentValue(params.MaxUnavailable) == 0 && getIntOrPercentValue(params.MaxSurge) == 0 {
		// Both MaxSurge and MaxUnavailable cannot be zero.
		errs = append(errs, field.Invalid(fldPath.Child("maxUnavailable"), params.MaxUnavailable, "cannot be 0 when maxSurge is 0 as well"))
	}
	// Validate that MaxUnavailable is not more than 100%.
	errs = append(errs, IsNotMoreThan100Percent(params.MaxUnavailable, fldPath.Child("maxUnavailable"))...)

	if params.Pre != nil {
		errs = append(errs, validateLifecycleHook(params.Pre, pod, fldPath.Child("pre"))...)
	}
	if params.Post != nil {
		errs = append(errs, validateLifecycleHook(params.Post, pod, fldPath.Child("post"))...)
	}

	return errs
}

func validateTrigger(trigger *appsapi.DeploymentTriggerPolicy, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if len(trigger.Type) == 0 {
		errs = append(errs, field.Required(fldPath.Child("type"), ""))
	}

	if trigger.Type == appsapi.DeploymentTriggerOnImageChange {
		if trigger.ImageChangeParams == nil {
			errs = append(errs, field.Required(fldPath.Child("imageChangeParams"), ""))
		} else {
			errs = append(errs, validateImageChangeParams(trigger.ImageChangeParams, fldPath.Child("imageChangeParams"))...)
		}
	}

	return errs
}

func validateImageChangeParams(params *appsapi.DeploymentTriggerImageChangeParams, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	fromPath := fldPath.Child("from")
	if len(params.From.Name) == 0 {
		errs = append(errs, field.Required(fromPath, ""))
	} else {
		if params.From.Kind != "ImageStreamTag" {
			errs = append(errs, field.Invalid(fromPath.Child("kind"), params.From.Kind, "kind must be an ImageStreamTag"))
		}
		if err := validateImageStreamTagName(params.From.Name); err != nil {
			errs = append(errs, field.Invalid(fromPath.Child("name"), params.From.Name, err.Error()))
		}
		if len(params.From.Namespace) != 0 && len(kvalidation.IsDNS1123Subdomain(params.From.Namespace)) != 0 {
			errs = append(errs, field.Invalid(fromPath.Child("namespace"), params.From.Namespace, "namespace must be a valid subdomain"))
		}
	}

	if len(params.ContainerNames) == 0 {
		errs = append(errs, field.Required(fldPath.Child("containerNames"), ""))
	}

	return errs
}

func validateImageStreamTagName(istag string) error {
	name, _, ok := imageapi.SplitImageStreamTag(istag)
	if !ok {
		return fmt.Errorf("must be in the form of <name>:<tag>")
	}
	if reasons := imageval.ValidateImageStreamName(name, false); len(reasons) != 0 {
		return errors.New(strings.Join(reasons, ", "))
	}
	return nil
}

func ValidatePositiveIntOrPercent(intOrPercent intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if intOrPercent.Type == intstr.String {
		if !IsValidPercent(intOrPercent.StrVal) {
			allErrs = append(allErrs, field.Invalid(fldPath, intOrPercent, "value should be int(5) or percentage(5%)"))
		}

	} else if intOrPercent.Type == intstr.Int {
		allErrs = append(allErrs, ValidatePositiveField(int64(intOrPercent.IntVal), fldPath)...)
	}
	return allErrs
}

func getPercentValue(intOrStringValue intstr.IntOrString) (int, bool) {
	if intOrStringValue.Type != intstr.String || !IsValidPercent(intOrStringValue.StrVal) {
		return 0, false
	}
	value, _ := strconv.Atoi(intOrStringValue.StrVal[:len(intOrStringValue.StrVal)-1])
	return value, true
}

func getIntOrPercentValue(intOrStringValue intstr.IntOrString) int {
	value, isPercent := getPercentValue(intOrStringValue)
	if isPercent {
		return value
	}
	return int(intOrStringValue.IntVal)
}

func IsNotMoreThan100Percent(intOrStringValue intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	value, isPercent := getPercentValue(intOrStringValue)
	if !isPercent || value <= 100 {
		return nil
	}
	allErrs = append(allErrs, field.Invalid(fldPath, intOrStringValue, "should not be more than 100%"))
	return allErrs
}

func ValidatePositiveField(value int64, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if value < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value, isNegativeErrorMsg))
	}
	return allErrs
}

const percentFmt string = "[0-9]+%"

var percentRegexp = regexp.MustCompile("^" + percentFmt + "$")

func IsValidPercent(percent string) bool {
	return percentRegexp.MatchString(percent)
}

const isNegativeErrorMsg string = `must be non-negative`

func ValidateDeploymentRequest(req *appsapi.DeploymentRequest) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(req.Name) == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("name"), req.Name, "name of the deployment config is missing"))
	} else if len(kvalidation.IsDNS1123Subdomain(req.Name)) != 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("name"), req.Name, "name of the deployment config is invalid"))
	}

	return allErrs
}

func ValidateRequestForDeploymentConfig(req *appsapi.DeploymentRequest, config *appsapi.DeploymentConfig) field.ErrorList {
	allErrs := ValidateDeploymentRequest(req)

	if config.Spec.Paused {
		// TODO: Enable deployment requests for paused deployment configs
		// See https://github.com/openshift/origin/issues/9903
		details := fmt.Sprintf("deployment config %q is paused - unpause to request a new deployment", config.Name)
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("paused"), config.Spec.Paused, details))
	}

	return allErrs
}

func ValidateDeploymentLogOptions(opts *appsapi.DeploymentLogOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	// TODO: Replace by validating PodLogOptions via DeploymentLogOptions once it's bundled in
	popts := appsapi.DeploymentToPodLogOptions(opts)
	if errs := validation.ValidatePodLogOptions(popts); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if opts.Version != nil && *opts.Version <= 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("version"), *opts.Version, "deployment version must be greater than 0"))
	}
	if opts.Version != nil && opts.Previous {
		allErrs = append(allErrs, field.Invalid(field.NewPath("previous"), opts.Previous, "cannot use previous when a version is specified"))
	}

	return allErrs
}
