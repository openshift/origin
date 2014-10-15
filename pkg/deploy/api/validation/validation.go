package validation

import (
	"strconv"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// TODO: These tests validate the ReplicationControllerState in a Deployment or DeploymentConfig.
//       The upstream validation API isn't factored currently to allow this; we'll make a PR to
//       upstream and fix when it goes in.

func ValidateDeployment(deployment *deployapi.Deployment) errors.ErrorList {
	result := validateDeploymentStrategy(&deployment.Strategy).Prefix("Strategy")
	controllerStateErrors := validation.ValidateReplicationControllerState(&deployment.ControllerTemplate)
	result = append(result, controllerStateErrors.Prefix("ControllerTemplate")...)

	return result
}

func validateDeploymentStrategy(strategy *deployapi.DeploymentStrategy) errors.ErrorList {
	result := errors.ErrorList{}

	if len(strategy.Type) == 0 {
		result = append(result, errors.NewFieldRequired("Type", ""))
	}

	if strategy.CustomPod == nil {
		result = append(result, errors.NewFieldRequired("CustomPod", nil))
	} else {
		result = append(result, validateCustomPodStrategy(strategy.CustomPod).Prefix("CustomPod")...)
	}

	return result
}

func validateCustomPodStrategy(customPod *deployapi.CustomPodDeploymentStrategy) errors.ErrorList {
	result := errors.ErrorList{}

	if len(customPod.Image) == 0 {
		result = append(result, errors.NewFieldRequired("Image", ""))
	}

	return result
}

func validateTrigger(trigger *deployapi.DeploymentTriggerPolicy) errors.ErrorList {
	result := errors.ErrorList{}

	if len(trigger.Type) == 0 {
		result = append(result, errors.NewFieldRequired("Type", ""))
	}

	if trigger.Type == deployapi.DeploymentTriggerOnImageChange {
		if trigger.ImageChangeParams == nil {
			result = append(result, errors.NewFieldRequired("ImageChangeParams", nil))
		} else {
			result = append(result, validateImageChangeParams(trigger.ImageChangeParams).Prefix("ImageChangeParams")...)
		}
	}

	return result
}

func validateImageChangeParams(params *deployapi.DeploymentTriggerImageChangeParams) errors.ErrorList {
	result := errors.ErrorList{}

	if len(params.RepositoryName) == 0 {
		result = append(result, errors.NewFieldRequired("RepositoryName", ""))
	}

	if len(params.ContainerNames) == 0 {
		result = append(result, errors.NewFieldRequired("ContainerNames", ""))
	}

	return result
}

func ValidateDeploymentConfig(config *deployapi.DeploymentConfig) errors.ErrorList {
	result := errors.ErrorList{}

	for i, _ := range config.Triggers {
		result = append(result, validateTrigger(&config.Triggers[i]).Prefix("Triggers["+strconv.Itoa(i)+"]")...)
	}

	result = append(result, validateDeploymentStrategy(&config.Template.Strategy).Prefix("Template.Strategy")...)
	controllerStateErrors := validation.ValidateReplicationControllerState(&config.Template.ControllerTemplate)
	result = append(result, controllerStateErrors.Prefix("Template.ControllerTemplate")...)

	return result
}
