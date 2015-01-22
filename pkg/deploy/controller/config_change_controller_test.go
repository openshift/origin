package controller

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapitest "github.com/openshift/origin/pkg/deploy/api/test"
	deploytest "github.com/openshift/origin/pkg/deploy/controller/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// Test the controller's response to a new DeploymentConfig with a config change trigger.
func TestNewConfigWithoutTrigger(t *testing.T) {
	generated := false
	updated := false

	controller := &DeploymentConfigChangeController{
		Codec: api.Codec,
		ChangeStrategy: &testChangeStrategy{
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				generated = true
				return nil, nil
			},
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updated = true
				return config, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			config := deployapitest.OkDeploymentConfig(1)
			config.Triggers = []deployapi.DeploymentTriggerPolicy{}
			return config
		},
		DeploymentStore: deploytest.NewFakeDeploymentStore(nil),
	}

	controller.HandleDeploymentConfig()

	if generated {
		t.Error("Unexpected generation of deploymentConfig")
	}

	if updated {
		t.Error("Unexpected update of deploymentConfig")
	}
}

func TestNewConfigWithTrigger(t *testing.T) {
	var updated *deployapi.DeploymentConfig

	controller := &DeploymentConfigChangeController{
		Codec: api.Codec,
		ChangeStrategy: &testChangeStrategy{
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return deployapitest.OkDeploymentConfig(1), nil
			},
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updated = config
				return config, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			config := deployapitest.OkDeploymentConfig(0)
			config.Triggers = []deployapi.DeploymentTriggerPolicy{deployapitest.OkConfigChangeTrigger()}
			return config
		},
		DeploymentStore: deploytest.NewFakeDeploymentStore(nil),
	}

	controller.HandleDeploymentConfig()

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

// Test the controller's response when the pod template is changed
func TestChangeWithTemplateDiff(t *testing.T) {
	var updated *deployapi.DeploymentConfig
	deployment, _ := deployutil.MakeDeployment(deployapitest.OkDeploymentConfig(1), kapi.Codec)

	controller := &DeploymentConfigChangeController{
		Codec: api.Codec,
		ChangeStrategy: &testChangeStrategy{
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return deployapitest.OkDeploymentConfig(2), nil
			},
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updated = config
				return config, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			config := deployapitest.OkDeploymentConfig(1)
			config.Triggers = []deployapi.DeploymentTriggerPolicy{deployapitest.OkConfigChangeTrigger()}
			config.Template.ControllerTemplate.Template.Spec.Containers[1].Name = "modified"
			return config
		},
		DeploymentStore: deploytest.NewFakeDeploymentStore(deployment),
	}

	controller.HandleDeploymentConfig()

	if updated == nil {
		t.Fatalf("expected config to be updated")
	}

	if e, a := 2, updated.LatestVersion; e != a {
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

func TestChangeWithoutTemplateDiff(t *testing.T) {
	config := deployapitest.OkDeploymentConfig(1)
	config.Triggers = []deployapi.DeploymentTriggerPolicy{deployapitest.OkConfigChangeTrigger()}

	deployment, _ := deployutil.MakeDeployment(deployapitest.OkDeploymentConfig(1), kapi.Codec)

	generated := false
	updated := false

	controller := &DeploymentConfigChangeController{
		Codec: api.Codec,
		ChangeStrategy: &testChangeStrategy{
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				generated = true
				return config, nil
			},
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updated = true
				return config, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return config
		},
		DeploymentStore: deploytest.NewFakeDeploymentStore(deployment),
	}

	controller.HandleDeploymentConfig()

	if generated {
		t.Error("Unexpected generation of deploymentConfig")
	}

	if updated {
		t.Error("Unexpected update of deploymentConfig")
	}
}

type testChangeStrategy struct {
	GenerateDeploymentConfigFunc func(namespace, name string) (*deployapi.DeploymentConfig, error)
	UpdateDeploymentConfigFunc   func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

func (i *testChangeStrategy) GenerateDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error) {
	return i.GenerateDeploymentConfigFunc(namespace, name)
}

func (i *testChangeStrategy) UpdateDeploymentConfig(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	return i.UpdateDeploymentConfigFunc(namespace, config)
}
