package controller

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
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
		generatedName string
		updated       *deployapi.DeploymentConfig
	)

	controller := &DeploymentConfigChangeController{
		Codec: api.Codec,
		ChangeStrategy: &testChangeStrategy{
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				generatedName = name
				return generatedConfig(), nil
			},
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
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

	if generatedName != "test-deploy-config" {
		t.Fatalf("Unexpected generated config id.  Expected test-deploy-config, got: %v", generatedName)
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
		generatedName string
		updated       *deployapi.DeploymentConfig
	)

	controller := &DeploymentConfigChangeController{
		Codec: api.Codec,
		ChangeStrategy: &testChangeStrategy{
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				generatedName = name
				return generatedExistingConfig(), nil
			},
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updated = config
				return config, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return diffedConfig()
		},
		DeploymentStore: deploytest.NewFakeDeploymentStore(matchingInitialDeployment(generatedConfig())),
	}

	controller.HandleDeploymentConfig()

	if generatedName != "test-deploy-config" {
		t.Fatalf("Unexpected generated config id.  Expected test-deploy-config, got: %v", generatedName)
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
	config := existingConfigWithTrigger()
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
		DeploymentStore: deploytest.NewFakeDeploymentStore(matchingInitialDeployment(config)),
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
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{
					"name": "test-pod",
				},
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
						Labels: map[string]string{
							"name": "test-pod",
						},
					},
					Spec: kapi.PodSpec{
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
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{
					"name": "test-pod-2",
				},
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
						Labels: map[string]string{
							"name": "test-pod-2",
						},
					},
					Spec: kapi.PodSpec{
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
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{
					"name": "test-pod",
				},
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
						Labels: map[string]string{
							"name": "test-pod",
						},
					},
					Spec: kapi.PodSpec{
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
	}
}

func generatedConfig() *deployapi.DeploymentConfig {
	config := generatedExistingConfig()
	config.LatestVersion = 0
	return config
}

func matchingInitialDeployment(config *deployapi.DeploymentConfig) *kapi.ReplicationController {
	encodedConfig, _ := deployutil.EncodeDeploymentConfig(config, api.Codec)

	return &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: deployutil.LatestDeploymentIDForConfig(config),
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation:        config.Name,
				deployapi.DeploymentStatusAnnotation:        string(deployapi.DeploymentStatusNew),
				deployapi.DeploymentEncodedConfigAnnotation: encodedConfig,
			},
		},
		Spec: config.Template.ControllerTemplate,
	}
}
