package validation

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// TODO: These tests validate the ReplicationControllerState in a Deployment or DeploymentConfig.
//       The upstream validation API isn't factored currently to allow this; we'll make a PR to
//       upstream and fix when it goes in.

func ValidateDeployment(deployment *deployapi.Deployment) fielderrors.ValidationErrorList {
	errs := validateDeploymentStrategy(&deployment.Strategy).Prefix("strategy")
	if len(deployment.Name) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("name"))
	} else if !util.IsDNS1123Subdomain(deployment.Name) {
		errs = append(errs, fielderrors.NewFieldInvalid("name", deployment.Name, "name must be a valid subdomain"))
	}
	if len(deployment.Namespace) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("namespace"))
	} else if !util.IsDNS1123Subdomain(deployment.Namespace) {
		errs = append(errs, fielderrors.NewFieldInvalid("namespace", deployment.Namespace, "namespace must be a valid subdomain"))
	}
	errs = append(errs, validation.ValidateLabels(deployment.Labels, "labels")...)
	errs = append(errs, validation.ValidateReplicationControllerSpec(&deployment.ControllerTemplate).Prefix("controllerTemplate")...)
	return errs
}

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
	case deployapi.DeploymentStrategyTypeCustom:
		if strategy.CustomParams == nil {
			errs = append(errs, fielderrors.NewFieldRequired("customParams"))
		} else {
			errs = append(errs, validateCustomParams(strategy.CustomParams).Prefix("customParams")...)
		}
	}

	return errs
}

func validateCustomParams(params *deployapi.CustomDeploymentStrategyParams) fielderrors.ValidationErrorList {
	errs := fielderrors.ValidationErrorList{}

	if len(params.Image) == 0 {
		errs = append(errs, fielderrors.NewFieldRequired("image"))
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
			params.From.Kind = "ImageRepository"
		}
		if params.From.Kind != "ImageRepository" {
			errs = append(errs, fielderrors.NewFieldInvalid("from.kind", params.From.Kind, "only 'ImageRepository' is allowed"))
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
