package validation

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// TODO: These tests validate the ReplicationControllerState in a Deployment or DeploymentConfig.
//       The upstream validation API isn't factored currently to allow this; we'll make a PR to
//       upstream and fix when it goes in.

func ValidateDeployment(deployment *deployapi.Deployment) errors.ErrorList {
	result := validateDeploymentStrategy(&deployment.Strategy).Prefix("Strategy")

	// TODO: validate ReplicationControllerState

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
		if len(strategy.CustomPod.Image) == 0 {
			result = append(result, errors.NewFieldRequired("CustomPod.Image", ""))
		}
	}

	return result
}

func validateTriggerPolicy(policy *deployapi.DeploymentTriggerPolicy) errors.ErrorList {
	result := errors.ErrorList{}

	if len(policy.Type) == 0 {
		result = append(result, errors.NewFieldRequired("Type", ""))
	}

	return result
}

func ValidateDeploymentConfig(config *deployapi.DeploymentConfig) errors.ErrorList {
	result := errors.ErrorList{}
	result = append(result, validateTriggerPolicy(&config.TriggerPolicy).Prefix("TriggerPolicy")...)
	result = append(result, validateDeploymentStrategy(&config.Template.Strategy).Prefix("Template.Strategy")...)

	// TODO: validate ReplicationControllerState

	return result
}
