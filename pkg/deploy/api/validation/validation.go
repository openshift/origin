package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/fielderrors"
	kvalidation "k8s.io/kubernetes/pkg/util/validation"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imageval "github.com/openshift/origin/pkg/image/api/validation"
)

func ValidateDeploymentConfig(config *deployapi.DeploymentConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&config.ObjectMeta, true, validation.NameIsDNSSubdomain).Prefix("metadata")...)

	// TODO: Refactor to validate spec and status separately
	for i := range config.Spec.Triggers {
		allErrs = append(allErrs, validateTrigger(&config.Spec.Triggers[i]).PrefixIndex(i).Prefix("spec.triggers")...)
	}
	allErrs = append(allErrs, validateDeploymentStrategy(&config.Spec.Strategy).Prefix("spec.strategy")...)
	if config.Spec.Template == nil {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("spec.template"))
	} else {
		allErrs = append(allErrs, validation.ValidatePodTemplateSpec(config.Spec.Template).Prefix("spec.template")...)
	}
	if config.Status.LatestVersion < 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("status.latestVersion", config.Status.LatestVersion, "latestVersion cannot be negative"))
	}
	if config.Spec.Replicas < 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("spec.replicas", config.Spec.Replicas, "replicas cannot be negative"))
	}
	if config.Spec.Selector == nil || len(config.Spec.Selector) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("spec.selector", config.Spec.Selector, "selector cannot be empty"))
	}
	return allErrs
}

func ValidateDeploymentConfigUpdate(newConfig *deployapi.DeploymentConfig, oldConfig *deployapi.DeploymentConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&newConfig.ObjectMeta, &oldConfig.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateDeploymentConfig(newConfig)...)
	if newConfig.Status.LatestVersion < oldConfig.Status.LatestVersion {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("status.latestVersion", newConfig.Status.LatestVersion, "latestVersion cannot be decremented"))
	} else if newConfig.Status.LatestVersion > (oldConfig.Status.LatestVersion + 1) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("status.latestVersion", newConfig.Status.LatestVersion, "latestVersion can only be incremented by 1"))
	}
	return allErrs
}

func ValidateDeploymentConfigRollback(rollback *deployapi.DeploymentConfigRollback) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	if len(rollback.Spec.From.Name) == 0 {
		result = append(result, fielderrors.NewFieldRequired("spec.from.name"))
	}

	if len(rollback.Spec.From.Kind) == 0 {
		rollback.Spec.From.Kind = "ReplicationController"
	}

	if rollback.Spec.From.Kind != "ReplicationController" {
		result = append(result, fielderrors.NewFieldInvalid("spec.from.kind", rollback.Spec.From.Kind, "the kind of the rollback target must be 'ReplicationController'"))
	}

	return result
}

func validateDeploymentStrategy(strategy *deployapi.DeploymentStrategy) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}

	if len(strategy.Type) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("type"))
	}

	switch strategy.Type {
	case deployapi.DeploymentStrategyTypeRecreate:
		if strategy.RecreateParams != nil {
			errs = append(errs, validateRecreateParams(strategy.RecreateParams).Prefix("recreateParams")...)
		}
	case deployapi.DeploymentStrategyTypeRolling:
		if strategy.RollingParams == nil {
			errs = append(errs, fielderrors.NewFieldRequired("rollingParams"))
		} else {
			errs = append(errs, validateRollingParams(strategy.RollingParams).Prefix("rollingParams")...)
		}
	case deployapi.DeploymentStrategyTypeCustom:
		if strategy.CustomParams == nil {
			errs = append(errs, fielderrors.NewFieldRequired("customParams"))
		} else {
			errs = append(errs, validateCustomParams(strategy.CustomParams).Prefix("customParams")...)
		}
	}

	if strategy.Labels != nil {
		errs = append(errs, validation.ValidateLabels(strategy.Labels, "labels")...)
	}
	if strategy.Annotations != nil {
		errs = append(errs, validation.ValidateAnnotations(strategy.Annotations, "annotations")...)
	}

	// TODO: validate resource requirements (prereq: https://github.com/kubernetes/kubernetes/pull/7059)

	return errs
}

func validateCustomParams(params *deployapi.CustomDeploymentStrategyParams) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}

	if len(params.Image) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("image"))
	}

	return errs
}

func validateRecreateParams(params *deployapi.RecreateDeploymentStrategyParams) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}

	if params.Pre != nil {
		errs = append(errs, validateLifecycleHook(params.Pre).Prefix("pre")...)
	}
	if params.Post != nil {
		errs = append(errs, validateLifecycleHook(params.Post).Prefix("post")...)
	}

	return errs
}

func validateLifecycleHook(hook *deployapi.LifecycleHook) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}

	if len(hook.FailurePolicy) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("failurePolicy"))
	}

	if hook.ExecNewPod == nil {
		errs = append(errs, fielderrors.NewFieldRequired("execNewPod"))
	} else {
		errs = append(errs, validateExecNewPod(hook.ExecNewPod).Prefix("execNewPod")...)
	}

	return errs
}

func validateExecNewPod(hook *deployapi.ExecNewPodHook) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}

	if len(hook.Command) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("command"))
	}

	if len(hook.ContainerName) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("containerName"))
	}

	if len(hook.Env) > 0 {
		errs = append(errs, validateEnv(hook.Env).Prefix("env")...)
	}

	errs = append(errs, validateHookVolumes(hook.Volumes).Prefix("volumes")...)

	return errs
}

func validateEnv(vars []kapi.EnvVar) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	for i, ev := range vars {
		vErrs := fielderrors.ValidationErrorList{}
		if len(ev.Name) == 0 {
			vErrs = append(vErrs, fielderrors.NewFieldRequired("name"))
		}
		if !kvalidation.IsCIdentifier(ev.Name) {
			vErrs = append(vErrs, fielderrors.NewFieldInvalid("name", ev.Name, "must match regex "+kvalidation.CIdentifierFmt))
		}
		allErrs = append(allErrs, vErrs.PrefixIndex(i)...)
	}
	return allErrs
}

func validateHookVolumes(volumes []string) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}
	for i, vol := range volumes {
		vErrs := fielderrors.ValidationErrorList{}
		if len(vol) == 0 {
			vErrs = append(vErrs, fielderrors.NewFieldInvalid("", "", "must not be empty"))
		}
		errs = append(errs, vErrs.PrefixIndex(i)...)
	}
	return errs
}

func validateRollingParams(params *deployapi.RollingDeploymentStrategyParams) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}

	if params.IntervalSeconds != nil && *params.IntervalSeconds < 1 {
		errs = append(errs, fielderrors.NewFieldInvalid("intervalSeconds", *params.IntervalSeconds, "must be >0"))
	}

	if params.UpdatePeriodSeconds != nil && *params.UpdatePeriodSeconds < 1 {
		errs = append(errs, fielderrors.NewFieldInvalid("updatePeriodSeconds", *params.UpdatePeriodSeconds, "must be >0"))
	}

	if params.TimeoutSeconds != nil && *params.TimeoutSeconds < 1 {
		errs = append(errs, fielderrors.NewFieldInvalid("timeoutSeconds", *params.TimeoutSeconds, "must be >0"))
	}

	if params.UpdatePercent != nil {
		p := *params.UpdatePercent
		if p == 0 || p < -100 || p > 100 {
			errs = append(errs, fielderrors.NewFieldInvalid("updatePercent", *params.UpdatePercent, "must be between 1 and 100 or between -1 and -100 (inclusive)"))
		}
	}
	// Most of this is lifted from the upstream experimental deployments API. We
	// can't reuse it directly yet, but no use reinventing the logic, so copy-
	// pasted and adapted here.
	errs = append(errs, ValidatePositiveIntOrPercent(params.MaxUnavailable, "maxUnavailable")...)
	errs = append(errs, ValidatePositiveIntOrPercent(params.MaxSurge, "maxSurge")...)
	if getIntOrPercentValue(params.MaxUnavailable) == 0 && getIntOrPercentValue(params.MaxSurge) == 0 {
		// Both MaxSurge and MaxUnavailable cannot be zero.
		errs = append(errs, fielderrors.NewFieldInvalid("maxUnavailable", params.MaxUnavailable, "cannot be 0 when maxSurge is 0 as well"))
	}
	// Validate that MaxUnavailable is not more than 100%.
	errs = append(errs, IsNotMoreThan100Percent(params.MaxUnavailable, "maxUnavailable")...)

	if params.Pre != nil {
		errs = append(errs, validateLifecycleHook(params.Pre).Prefix("pre")...)
	}
	if params.Post != nil {
		errs = append(errs, validateLifecycleHook(params.Post).Prefix("post")...)
	}

	return errs
}

func validateTrigger(trigger *deployapi.DeploymentTriggerPolicy) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}

	if len(trigger.Type) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("type"))
	}

	if trigger.Type == deployapi.DeploymentTriggerOnImageChange {
		if trigger.ImageChangeParams == nil {
			errs = append(errs, fielderrors.NewFieldRequired("imageChangeParams"))
		} else {
			errs = append(errs, validateImageChangeParams(trigger.ImageChangeParams).Prefix("imageChangeParams")...)
		}
	}

	return errs
}

func validateImageChangeParams(params *deployapi.DeploymentTriggerImageChangeParams) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}

	if len(params.From.Name) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("from"))
	} else {
		if params.From.Kind != "ImageStreamTag" {
			errs = append(errs, fielderrors.NewFieldInvalid("from.kind", params.From.Kind, "kind must be an ImageStreamTag"))
		}
		if err := validateImageStreamTagName(params.From.Name); err != nil {
			errs = append(errs, fielderrors.NewFieldInvalid("from.name", params.From.Name, err.Error()))
		}
		if len(params.From.Namespace) != 0 && !kvalidation.IsDNS1123Subdomain(params.From.Namespace) {
			errs = append(errs, fielderrors.NewFieldInvalid("from.namespace", params.From.Namespace, "namespace must be a valid subdomain"))
		}
	}

	if len(params.ContainerNames) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("containerNames"))
	}

	return errs
}

func validateImageStreamTagName(istag string) error {
	name, _, ok := imageapi.SplitImageStreamTag(istag)
	if !ok {
		return fmt.Errorf("invalid ImageStreamTag: %s", istag)
	}
	ok, reason := imageval.ValidateImageStreamName(name, false)
	if !ok {
		return errors.New(reason)
	}
	return nil
}

func ValidatePositiveIntOrPercent(intOrPercent util.IntOrString, fieldName string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if intOrPercent.Kind == util.IntstrString {
		if !IsValidPercent(intOrPercent.StrVal) {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(fieldName, intOrPercent, "value should be int(5) or percentage(5%)"))
		}

	} else if intOrPercent.Kind == util.IntstrInt {
		allErrs = append(allErrs, ValidatePositiveField(int64(intOrPercent.IntVal), fieldName)...)
	}
	return allErrs
}

func getPercentValue(intOrStringValue util.IntOrString) (int, bool) {
	if intOrStringValue.Kind != util.IntstrString || !IsValidPercent(intOrStringValue.StrVal) {
		return 0, false
	}
	value, _ := strconv.Atoi(intOrStringValue.StrVal[:len(intOrStringValue.StrVal)-1])
	return value, true
}

func getIntOrPercentValue(intOrStringValue util.IntOrString) int {
	value, isPercent := getPercentValue(intOrStringValue)
	if isPercent {
		return value
	}
	return intOrStringValue.IntVal
}

func IsNotMoreThan100Percent(intOrStringValue util.IntOrString, fieldName string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	value, isPercent := getPercentValue(intOrStringValue)
	if !isPercent || value <= 100 {
		return nil
	}
	allErrs = append(allErrs, fielderrors.NewFieldInvalid(fieldName, intOrStringValue, "should not be more than 100%"))
	return allErrs
}

func ValidatePositiveField(value int64, fieldName string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if value < 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(fieldName, value, isNegativeErrorMsg))
	}
	return allErrs
}

const percentFmt string = "[0-9]+%"

var percentRegexp = regexp.MustCompile("^" + percentFmt + "$")

func IsValidPercent(percent string) bool {
	return percentRegexp.MatchString(percent)
}

const isNegativeErrorMsg string = `must be non-negative`

func ValidateDeploymentLogOptions(opts *deployapi.DeploymentLogOptions) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	// TODO: Replace by validating PodLogOptions via DeploymentLogOptions once it's bundled in
	popts := deployapi.DeploymentToPodLogOptions(opts)
	if errs := validation.ValidatePodLogOptions(popts); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if opts.Version != nil && *opts.Version <= 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("version", *opts.Version, "deployment version must be greater than 0"))
	}

	return allErrs
}
