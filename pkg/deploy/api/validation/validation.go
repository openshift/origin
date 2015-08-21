package validation

import (
	"fmt"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// TODO: These tests validate the ReplicationControllerState in a Deployment or DeploymentConfig.
//       The upstream validation API isn't factored currently to allow this; we'll make a PR to
//       upstream and fix when it goes in.

func ValidateDeploymentConfig(config *deployapi.DeploymentConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMeta(&config.ObjectMeta, true, validation.NameIsDNSSubdomain).Prefix("metadata")...)

	for i := range config.Triggers {
		allErrs = append(allErrs, validateTrigger(&config.Triggers[i]).PrefixIndex(i).Prefix("triggers")...)
	}
	allErrs = append(allErrs, validateDeploymentStrategy(&config.Template.Strategy).Prefix("template.strategy")...)
	allErrs = append(allErrs, validation.ValidateReplicationControllerSpec(&config.Template.ControllerTemplate).Prefix("template.controllerTemplate")...)
	if config.LatestVersion < 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("latestVersion", config.LatestVersion, "latestVersion cannot be negative"))
	}
	return allErrs
}

func ValidateDeploymentConfigUpdate(newConfig *deployapi.DeploymentConfig, oldConfig *deployapi.DeploymentConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&newConfig.ObjectMeta, &oldConfig.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateDeploymentConfig(newConfig)...)
	if newConfig.LatestVersion < oldConfig.LatestVersion {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("latestVersion", newConfig.LatestVersion, "latestVersion cannot be decremented"))
	} else if newConfig.LatestVersion > (oldConfig.LatestVersion + 1) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("latestVersion", newConfig.LatestVersion, "latestVersion can only be incremented by 1"))
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

	// TODO: validate resource requirements (prereq: https://k8s.io/kubernetes/pull/7059)

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

	return errs
}

func validateEnv(vars []kapi.EnvVar) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	for i, ev := range vars {
		vErrs := fielderrors.ValidationErrorList{}
		if len(ev.Name) == 0 {
			vErrs = append(vErrs, fielderrors.NewFieldRequired("name"))
		}
		if !util.IsCIdentifier(ev.Name) {
			vErrs = append(vErrs, fielderrors.NewFieldInvalid("name", ev.Name, "must match regex "+util.CIdentifierFmt))
		}
		allErrs = append(allErrs, vErrs.PrefixIndex(i)...)
	}
	return allErrs
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

	if len(params.From.Name) != 0 {
		if len(params.From.Kind) == 0 {
			params.From.Kind = "ImageStream"
		}
		kinds := util.NewStringSet("ImageRepository", "ImageStream", "ImageStreamTag")
		if !kinds.Has(params.From.Kind) {
			msg := fmt.Sprintf("kind must be one of: %s", strings.Join(kinds.List(), ", "))
			errs = append(errs, fielderrors.NewFieldInvalid("from.kind", params.From.Kind, msg))
		}

		if !util.IsDNS1123Subdomain(params.From.Name) {
			errs = append(errs, fielderrors.NewFieldInvalid("from.name", params.From.Name, "name must be a valid subdomain"))
		}
		if len(params.From.Namespace) != 0 && !util.IsDNS1123Subdomain(params.From.Namespace) {
			errs = append(errs, fielderrors.NewFieldInvalid("from.namespace", params.From.Namespace, "namespace must be a valid subdomain"))
		}

		if len(params.RepositoryName) != 0 {
			errs = append(errs, fielderrors.NewFieldInvalid("repositoryName", params.RepositoryName, "only one of 'from', 'repository' name may be specified"))
		}
	} else {
		if len(params.RepositoryName) == 0 {
			errs = append(errs, fielderrors.NewFieldRequired("from"))
		}
	}

	if len(params.ContainerNames) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("containerNames"))
	}

	return errs
}
