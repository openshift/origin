package validation

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// TODO: These tests validate the ReplicationControllerState in a Deployment or DeploymentConfig.
//       The upstream validation API isn't factored currently to allow this; we'll make a PR to
//       upstream and fix when it goes in.

func ValidateDeploymentConfig(config *deployapi.DeploymentConfig) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}
	if len(config.Name) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("name"))
	} else if !util.IsDNS1123Subdomain(config.Name) {
		errs = append(errs, fielderrors.NewFieldInvalid("name", config.Name, "name must be a valid subdomain"))
	}
	if len(config.Namespace) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("namespace"))
	} else if !util.IsDNS1123Subdomain(config.Namespace) {
		errs = append(errs, fielderrors.NewFieldInvalid("namespace", config.Namespace, "namespace must be a valid subdomain"))
	}
	errs = append(errs, validation.ValidateLabels(config.Labels, "labels")...)

	for i := range config.Triggers {
		errs = append(errs, validateTrigger(&config.Triggers[i]).PrefixIndex(i).Prefix("triggers")...)
	}
	errs = append(errs, validateDeploymentStrategy(&config.Template.Strategy).Prefix("template.strategy")...)
	errs = append(errs, validation.ValidateReplicationControllerSpec(&config.Template.ControllerTemplate).Prefix("template.controllerTemplate")...)
	return errs
}

func ValidateDeploymentConfigUpdate(newConfig *deployapi.DeploymentConfig, oldConfig *deployapi.DeploymentConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&oldConfig.ObjectMeta, &newConfig.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, ValidateDeploymentConfig(newConfig)...)
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

	// TODO: validate resource requirements (prereq: https://github.com/GoogleCloudPlatform/kubernetes/pull/7059)

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

	if len(params.ContainerNames) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("containerNames"))
	}

	// Everything below this line is to validate image references.

	// Validate there's a reference of any kind
	if len(params.RepositoryName) == 0 && len(params.From.Name) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("from"))
		return errs
	}

	// Enforce RepositoryName/From mutual exclusivity
	if len(params.RepositoryName) > 0 && len(params.From.Name) > 0 {
		errs = append(errs, fielderrors.NewFieldInvalid("repositoryName", params.RepositoryName, "only one of repositoryName or from.Name may be specified"))
	}

	// Validate RepositoryName usage
	if len(params.RepositoryName) > 0 {
		if len(params.Tag) == 0 {
			errs = append(errs, fielderrors.NewFieldRequired("tag"))
		}
		// Nothing to validate for RepositoryName itself
		return errs
	}

	// Validate ImageStreamTag usage
	if params.From.Kind != "ImageStreamTag" {
		errs = append(errs, fielderrors.NewFieldInvalid("from.kind", params.From.Kind, "kind must be 'ImageStreamTag'"))
		return errs
	}

	if len(params.From.Name) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("from.name"))
	}

	// Ensure ImageStreamTag references aren't used in conjunction with Tag
	// (which should have been converted away).
	if len(params.Tag) > 0 {
		errs = append(errs, fielderrors.NewFieldInvalid("tag", params.Tag, "tag may not be specified when kind is ImageStreamTag"))
	}

	return errs
}
