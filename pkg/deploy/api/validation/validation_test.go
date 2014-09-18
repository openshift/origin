package validation

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/openshift/origin/pkg/deploy/api"
)

// Convenience methods

func manualTrigger() api.DeploymentTriggerPolicy {
	return api.DeploymentTriggerPolicy{
		Type: api.DeploymentTriggerManual,
	}
}

func okTemplate() api.DeploymentTemplate {
	return api.DeploymentTemplate{
		Strategy: okStrategy(),
	}
}

func okStrategy() api.DeploymentStrategy {
	return api.DeploymentStrategy{
		Type:      "customPod",
		CustomPod: okCustomPod(),
	}
}

func okCustomPod() *api.CustomPodDeploymentStrategy {
	return &api.CustomPodDeploymentStrategy{
		Image: "openshift/kube-deploy",
	}
}

// TODO: test validation errors for ReplicationControllerTemplates

func TestValidateDeploymentOK(t *testing.T) {
	errs := ValidateDeployment(&api.Deployment{
		Strategy: okStrategy(),
	})
	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %#v", errs)
	}
}

func TestValidateDeploymentMissingFields(t *testing.T) {
	errorCases := map[string]struct {
		D api.Deployment
		T errors.ValidationErrorType
		F string
	}{
		"missing Strategy.Type": {
			api.Deployment{
				Strategy: api.DeploymentStrategy{
					CustomPod: okCustomPod(),
				},
			},
			errors.ValidationErrorTypeRequired,
			"Strategy.Type",
		},
		"missing Strategy.CustomPod": {
			api.Deployment{
				Strategy: api.DeploymentStrategy{
					Type: "customPod",
				},
			},
			errors.ValidationErrorTypeRequired,
			"Strategy.CustomPod",
		},
		"missing Strategy.CustomPod.Image": {
			api.Deployment{
				Strategy: api.DeploymentStrategy{
					Type:      "customPod",
					CustomPod: &api.CustomPodDeploymentStrategy{},
				},
			},
			errors.ValidationErrorTypeRequired,
			"Strategy.CustomPod.Image",
		},
	}

	for k, v := range errorCases {
		errs := ValidateDeployment(&v.D)
		if len(errs) == 0 {
			t.Errorf("Expected failure for scenario %s", k)
		}
		for i := range errs {
			if errs[i].(errors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(errors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateDeploymentConfigOK(t *testing.T) {
	errs := ValidateDeploymentConfig(&api.DeploymentConfig{
		TriggerPolicy: manualTrigger(),
		Template:      okTemplate(),
	})

	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %#v", errs)
	}
}

func TestValidateDeploymentConfigMissingFields(t *testing.T) {
	errorCases := map[string]struct {
		D api.DeploymentConfig
		T errors.ValidationErrorType
		F string
	}{
		"missing TriggerPolicy.Type": {
			api.DeploymentConfig{Template: okTemplate()},
			errors.ValidationErrorTypeRequired,
			"TriggerPolicy.Type",
		},
		"missing Strategy.Type": {
			api.DeploymentConfig{
				TriggerPolicy: manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						CustomPod: okCustomPod(),
					},
				},
			},
			errors.ValidationErrorTypeRequired,
			"Template.Strategy.Type",
		},
		"missing Strategy.CustomPod": {
			api.DeploymentConfig{
				TriggerPolicy: manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: "customPod",
					},
				},
			},
			errors.ValidationErrorTypeRequired,
			"Template.Strategy.CustomPod",
		},
		"missing Template.Strategy.CustomPod.Image": {
			api.DeploymentConfig{
				TriggerPolicy: manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type:      "customPod",
						CustomPod: &api.CustomPodDeploymentStrategy{},
					},
				},
			},
			errors.ValidationErrorTypeRequired,
			"Template.Strategy.CustomPod.Image",
		},
	}

	for k, v := range errorCases {
		errs := ValidateDeploymentConfig(&v.D)
		if len(errs) == 0 {
			t.Errorf("Expected failure for scenario %s", k)
		}
		for i := range errs {
			if errs[i].(errors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(errors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}
