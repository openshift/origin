package validation

import (
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/openshift/origin/pkg/deploy/api"
)

// Convenience methods

func manualTrigger() []api.DeploymentTriggerPolicy {
	return []api.DeploymentTriggerPolicy{
		api.DeploymentTriggerPolicy{
			Type: api.DeploymentTriggerManual,
		},
	}
}

func okControllerTemplate() kubeapi.ReplicationControllerState {
	return kubeapi.ReplicationControllerState{
		ReplicaSelector: okSelector(),
		PodTemplate:     okPodTemplate(),
	}
}

func okSelector() map[string]string {
	return map[string]string{"a": "b"}
}

func okPodTemplate() kubeapi.PodTemplate {
	return kubeapi.PodTemplate{
		DesiredState: kubeapi.PodState{
			Manifest: kubeapi.ContainerManifest{
				Version: "v1beta1",
			},
		},
		Labels: okSelector(),
	}
}

func okDeploymentTemplate() api.DeploymentTemplate {
	return api.DeploymentTemplate{
		Strategy:           okStrategy(),
		ControllerTemplate: okControllerTemplate(),
	}
}

func okStrategy() api.DeploymentStrategy {
	return api.DeploymentStrategy{
		Type:      api.DeploymentStrategyTypeCustomPod,
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
		Strategy:           okStrategy(),
		ControllerTemplate: okControllerTemplate(),
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
		"missing strategy.type": {
			api.Deployment{
				Strategy: api.DeploymentStrategy{
					CustomPod: okCustomPod(),
				},
				ControllerTemplate: okControllerTemplate(),
			},
			errors.ValidationErrorTypeRequired,
			"strategy.type",
		},
		"missing strategy.customPod": {
			api.Deployment{
				Strategy: api.DeploymentStrategy{
					Type: api.DeploymentStrategyTypeCustomPod,
				},
				ControllerTemplate: okControllerTemplate(),
			},
			errors.ValidationErrorTypeRequired,
			"strategy.customPod",
		},
		"missing strategy.customPod.image": {
			api.Deployment{
				Strategy: api.DeploymentStrategy{
					Type:      api.DeploymentStrategyTypeCustomPod,
					CustomPod: &api.CustomPodDeploymentStrategy{},
				},
				ControllerTemplate: okControllerTemplate(),
			},
			errors.ValidationErrorTypeRequired,
			"strategy.customPod.image",
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
		Triggers: manualTrigger(),
		Template: okDeploymentTemplate(),
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
		"missing trigger.type": {
			api.DeploymentConfig{
				Triggers: []api.DeploymentTriggerPolicy{
					{
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							ContainerNames: []string{"foo"},
						},
					},
				},
				Template: okDeploymentTemplate(),
			},
			errors.ValidationErrorTypeRequired,
			"triggers[0].type",
		},
		"missing Trigger imageChangeParams.repositoryName": {
			api.DeploymentConfig{
				Triggers: []api.DeploymentTriggerPolicy{
					{
						Type: api.DeploymentTriggerOnImageChange,
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							ContainerNames: []string{"foo"},
						},
					},
				},
				Template: okDeploymentTemplate(),
			},
			errors.ValidationErrorTypeRequired,
			"triggers[0].imageChangeParams.repositoryName",
		},
		"missing Trigger imageChangeParams.containerNames": {
			api.DeploymentConfig{
				Triggers: []api.DeploymentTriggerPolicy{
					{
						Type: api.DeploymentTriggerOnImageChange,
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							RepositoryName: "foo",
						},
					},
				},
				Template: okDeploymentTemplate(),
			},
			errors.ValidationErrorTypeRequired,
			"triggers[0].imageChangeParams.containerNames",
		},
		"missing strategy.type": {
			api.DeploymentConfig{
				Triggers: manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						CustomPod: okCustomPod(),
					},
					ControllerTemplate: okControllerTemplate(),
				},
			},
			errors.ValidationErrorTypeRequired,
			"template.strategy.type",
		},
		"missing strategy.customPod": {
			api.DeploymentConfig{
				Triggers: manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeCustomPod,
					},
					ControllerTemplate: okControllerTemplate(),
				},
			},
			errors.ValidationErrorTypeRequired,
			"template.strategy.customPod",
		},
		"missing template.strategy.customPod.Image": {
			api.DeploymentConfig{
				Triggers: manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type:      api.DeploymentStrategyTypeCustomPod,
						CustomPod: &api.CustomPodDeploymentStrategy{},
					},
					ControllerTemplate: okControllerTemplate(),
				},
			},
			errors.ValidationErrorTypeRequired,
			"template.strategy.customPod.image",
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
