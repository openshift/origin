package controller

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/controller/test"
)

// Test the controller's response to a new DeploymentConfig with a config change trigger.
func TestNewConfigWithoutTrigger(t *testing.T) {
	generated := false
	updated := false

	controller := &DeploymentConfigChangeController{
		ChangeStrategy: &testChangeStrategy{
			GenerateDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				generated = true
				return nil, nil
			},
			UpdateDeploymentConfigFunc: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updated = true
				return config, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return newConfigWithoutTrigger()
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
	var (
		generatedId string
		updated     *deployapi.DeploymentConfig
	)

	controller := &DeploymentConfigChangeController{
		ChangeStrategy: &testChangeStrategy{
			GenerateDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				generatedId = id
				return generatedConfig(), nil
			},
			UpdateDeploymentConfigFunc: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updated = config
				return config, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return newConfigWithTrigger()
		},
		DeploymentStore: deploytest.NewFakeDeploymentStore(nil),
	}

	controller.HandleDeploymentConfig()

	if generatedId != "test-deploy-config" {
		t.Fatalf("Unexpected generated config id.  Expected test-deploy-config, got: %v", generatedId)
	}

	if updated.Name != "test-deploy-config" {
		t.Fatalf("Unexpected updated config id.  Expected test-deploy-config, got: %v", updated.Name)
	} else if updated.Details == nil {
		t.Fatalf("expected config change details to be set")
	} else if updated.Details.Causes == nil {
		t.Fatalf("expected config change causes to be set")
	} else if updated.Details.Causes[0].Type != deployapi.DeploymentTriggerOnConfigChange {
		t.Fatalf("expected config change cause to be set to config change trigger, got %s", updated.Details.Causes[0].Type)
	}
}

// Test the controller's response when the pod template is changed
func TestChangeWithTemplateDiff(t *testing.T) {
	var (
		generatedId string
		updated     *deployapi.DeploymentConfig
	)

	controller := &DeploymentConfigChangeController{
		ChangeStrategy: &testChangeStrategy{
			GenerateDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				generatedId = id
				return generatedExistingConfig(), nil
			},
			UpdateDeploymentConfigFunc: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updated = config
				return config, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return diffedConfig()
		},
		DeploymentStore: deploytest.NewFakeDeploymentStore(matchingInitialDeployment()),
	}

	controller.HandleDeploymentConfig()

	if generatedId != "test-deploy-config" {
		t.Fatalf("Unexpected generated config id.  Expected test-deploy-config, got: %v", generatedId)
	}

	if updated.Name != "test-deploy-config" {
		t.Fatalf("Unexpected updated config id.  Expected test-deploy-config, got: %v", updated.Name)
	} else if updated.Details == nil {
		t.Fatalf("expected config change details to be set")
	} else if updated.Details.Causes == nil {
		t.Fatalf("expected config change causes to be set")
	} else if updated.Details.Causes[0].Type != deployapi.DeploymentTriggerOnConfigChange {
		t.Fatalf("expected config change cause to be set to config change trigger, got %s", updated.Details.Causes[0].Type)
	}
}

func TestChangeWithoutTemplateDiff(t *testing.T) {
	generated := false
	updated := false

	controller := &DeploymentConfigChangeController{
		ChangeStrategy: &testChangeStrategy{
			GenerateDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				generated = true
				return nil, nil
			},
			UpdateDeploymentConfigFunc: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updated = true
				return config, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return existingConfigWithTrigger()
		},
		DeploymentStore: deploytest.NewFakeDeploymentStore(matchingInitialDeployment()),
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
	GenerateDeploymentConfigFunc func(id string) (*deployapi.DeploymentConfig, error)
	UpdateDeploymentConfigFunc   func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

func (i *testChangeStrategy) GenerateDeploymentConfig(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
	return i.GenerateDeploymentConfigFunc(id)
}

func (i *testChangeStrategy) UpdateDeploymentConfig(ctx kapi.Context, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	return i.UpdateDeploymentConfigFunc(config)
}

func existingConfigWithTrigger() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "test-deploy-config"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnConfigChange,
			},
		},
		LatestVersion: 2,
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerState{
				Replicas: 1,
				ReplicaSelector: map[string]string{
					"name": "test-pod",
				},
				PodTemplate: kapi.PodTemplate{
					Labels: map[string]string{
						"name": "test-pod",
					},
					DesiredState: kapi.PodState{
						Manifest: kapi.ContainerManifest{
							Version: "v1beta1",
							Containers: []kapi.Container{
								{
									Name:  "container-1",
									Image: "registry:8080/openshift/test-image:ref-1",
								},
							},
						},
					},
				},
			},
		},
	}
}

func newConfigWithTrigger() *deployapi.DeploymentConfig {
	config := existingConfigWithTrigger()
	config.LatestVersion = 0
	return config
}

func newConfigWithoutTrigger() *deployapi.DeploymentConfig {
	config := existingConfigWithTrigger()
	config.LatestVersion = 0
	config.Triggers = []deployapi.DeploymentTriggerPolicy{}
	return config
}

func diffedConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "test-deploy-config"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnConfigChange,
			},
		},
		LatestVersion: 2,
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerState{
				Replicas: 1,
				ReplicaSelector: map[string]string{
					"name": "test-pod-2",
				},
				PodTemplate: kapi.PodTemplate{
					Labels: map[string]string{
						"name": "test-pod-2",
					},
					DesiredState: kapi.PodState{
						Manifest: kapi.ContainerManifest{
							Version: "v1beta1",
							Containers: []kapi.Container{
								{
									Name:  "container-2",
									Image: "registry:8080/openshift/test-image:ref-1",
								},
							},
						},
					},
				},
			},
		},
	}
}

func generatedExistingConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "test-deploy-config"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnConfigChange,
			},
		},
		LatestVersion: 3,
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerState{
				Replicas: 1,
				ReplicaSelector: map[string]string{
					"name": "test-pod",
				},
				PodTemplate: kapi.PodTemplate{
					Labels: map[string]string{
						"name": "test-pod",
					},
					DesiredState: kapi.PodState{
						Manifest: kapi.ContainerManifest{
							Version: "v1beta1",
							Containers: []kapi.Container{
								{
									Name:  "container-1",
									Image: "registry:8080/openshift/test-image:ref-2",
								},
							},
						},
					},
				},
			},
		},
	}
}

func generatedConfig() *deployapi.DeploymentConfig {
	config := generatedExistingConfig()
	config.LatestVersion = 0
	return config
}

func matchingInitialDeployment() *deployapi.Deployment {
	return &deployapi.Deployment{
		ObjectMeta: kapi.ObjectMeta{Name: "test-deploy-config-1"},
		Status:     deployapi.DeploymentStatusNew,
		ControllerTemplate: kapi.ReplicationControllerState{
			Replicas: 1,
			ReplicaSelector: map[string]string{
				"name": "test-pod",
			},
			PodTemplate: kapi.PodTemplate{
				Labels: map[string]string{
					"name": "test-pod",
				},
				DesiredState: kapi.PodState{
					Manifest: kapi.ContainerManifest{
						Version: "v1beta1",
						Containers: []kapi.Container{
							{
								Name:  "container-1",
								Image: "registry:8080/openshift/test-image:ref-1",
							},
						},
					},
				},
			},
		},
	}
}
