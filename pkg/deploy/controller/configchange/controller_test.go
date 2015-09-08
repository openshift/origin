package configchange

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapitest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// TestHandle_newConfigNoTriggers ensures that a change to a config with no
// triggers doesn't result in a new config version bump.
func TestHandle_newConfigNoTriggers(t *testing.T) {
	controller := &DeploymentConfigChangeController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		changeStrategy: &changeStrategyImpl{
			generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected generation of deploymentConfig")
				return nil, nil
			},
			updateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected update of deploymentConfig")
				return config, nil
			},
		},
	}

	config := deployapitest.OkDeploymentConfig(1)
	config.Triggers = []deployapi.DeploymentTriggerPolicy{}
	err := controller.Handle(config)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_newConfigTriggers ensures that the creation of a new config
// (with version 0) with a config change trigger results in a version bump and
// cause update for initial deployment.
func TestHandle_newConfigTriggers(t *testing.T) {
	var updated *deployapi.DeploymentConfig

	controller := &DeploymentConfigChangeController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		changeStrategy: &changeStrategyImpl{
			generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return deployapitest.OkDeploymentConfig(1), nil
			},
			updateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updated = config
				return config, nil
			},
		},
	}

	config := deployapitest.OkDeploymentConfig(0)
	config.Triggers = []deployapi.DeploymentTriggerPolicy{deployapitest.OkConfigChangeTrigger()}
	err := controller.Handle(config)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated == nil {
		t.Fatalf("expected config to be updated")
	}

	if e, a := 1, updated.LatestVersion; e != a {
		t.Fatalf("expected update to latestversion=%d, got %d", e, a)
	}

	if updated.Details == nil {
		t.Fatalf("expected config change details to be set")
	} else if updated.Details.Causes == nil {
		t.Fatalf("expected config change causes to be set")
	} else if updated.Details.Causes[0].Type != deployapi.DeploymentTriggerOnConfigChange {
		t.Fatalf("expected config change cause to be set to config change trigger, got %s", updated.Details.Causes[0].Type)
	}
}

// TestHandle_changeWithTemplateDiff ensures that a pod template change to a
// config with a config change trigger results in a version bump and cause
// update.
func TestHandle_changeWithTemplateDiff(t *testing.T) {
	scenarios := []struct {
		name           string
		modify         func(*deployapi.DeploymentConfig)
		changeExpected bool
	}{
		{
			name:           "container name change",
			changeExpected: true,
			modify: func(config *deployapi.DeploymentConfig) {
				config.Template.ControllerTemplate.Template.Spec.Containers[1].Name = "modified"
			},
		},
		{
			name:           "template label change",
			changeExpected: true,
			modify: func(config *deployapi.DeploymentConfig) {
				config.Template.ControllerTemplate.Template.Labels["newkey"] = "value"
			},
		},
		{
			name:           "no diff",
			changeExpected: false,
			modify:         func(config *deployapi.DeploymentConfig) {},
		},
	}

	for _, s := range scenarios {
		t.Logf("running scenario: %s", s.name)

		config := deployapitest.OkDeploymentConfig(1)
		config.Triggers = []deployapi.DeploymentTriggerPolicy{deployapitest.OkConfigChangeTrigger()}
		deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
		var updated *deployapi.DeploymentConfig

		controller := &DeploymentConfigChangeController{
			decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
				return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
			},
			changeStrategy: &changeStrategyImpl{
				generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
					return deployapitest.OkDeploymentConfig(2), nil
				},
				updateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
					updated = config
					return config, nil
				},
				getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
					return deployment, nil
				},
			},
		}

		s.modify(config)
		err := controller.Handle(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if s.changeExpected {
			if updated == nil {
				t.Errorf("expected config to be updated")
				continue
			}
			if e, a := 2, updated.LatestVersion; e != a {
				t.Errorf("expected update to latestversion=%d, got %d", e, a)
			}

			if updated.Details == nil {
				t.Errorf("expected config change details to be set")
			} else if updated.Details.Causes == nil {
				t.Errorf("expected config change causes to be set")
			} else if updated.Details.Causes[0].Type != deployapi.DeploymentTriggerOnConfigChange {
				t.Errorf("expected config change cause to be set to config change trigger, got %s", updated.Details.Causes[0].Type)
			}
		} else {
			if updated != nil {
				t.Errorf("unexpected update to version %d", updated.LatestVersion)
			}
		}
	}
}
