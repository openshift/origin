package controller

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/controller/test"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type testIcDeploymentConfigInterface struct {
	UpdateDeploymentConfigFunc   func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	GenerateDeploymentConfigFunc func(namespace, name string) (*deployapi.DeploymentConfig, error)
}

func (i *testIcDeploymentConfigInterface) UpdateDeploymentConfig(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	return i.UpdateDeploymentConfigFunc(namespace, config)
}
func (i *testIcDeploymentConfigInterface) GenerateDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error) {
	return i.GenerateDeploymentConfigFunc(namespace, name)
}

const (
	nonDefaultNamespace = "nondefaultnamespace"
)

func TestUnregisteredContainer(t *testing.T) {
	config := unregisteredConfig()
	config.Triggers[0].ImageChangeParams.Automatic = false

	controller := &ImageChangeController{
		DeploymentConfigInterface: &testIcDeploymentConfigInterface{
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected deployment config update")
				return nil, nil
			},
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected generator call")
				return nil, nil
			},
		},
		NextImageRepository: func() *imageapi.ImageRepository {
			return tagUpdate()
		},
		DeploymentConfigStore: deploytest.NewFakeDeploymentConfigStore(config),
	}

	// verify no-op
	controller.HandleImageRepo()
}

func TestImageChangeForNonAutomaticTag(t *testing.T) {
	config := imageChangeDeploymentConfig()
	config.Triggers[0].ImageChangeParams.Automatic = false

	controller := &ImageChangeController{
		DeploymentConfigInterface: &testIcDeploymentConfigInterface{
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected deployment config update")
				return nil, nil
			},
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected generator call")
				return nil, nil
			},
		},
		NextImageRepository: func() *imageapi.ImageRepository {
			return tagUpdate()
		},
		DeploymentConfigStore: deploytest.NewFakeDeploymentConfigStore(config),
	}

	// verify no-op
	controller.HandleImageRepo()
}

func TestImageChangeForUnregisteredTag(t *testing.T) {
	config := imageChangeDeploymentConfig()

	controller := &ImageChangeController{
		DeploymentConfigInterface: &testIcDeploymentConfigInterface{
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected deployment config update")
				return nil, nil
			},
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected generator call")
				return nil, nil
			},
		},
		NextImageRepository: func() *imageapi.ImageRepository {
			imageRepo := tagUpdate()
			imageRepo.Tags = map[string]string{
				"unknown-tag": "ref-1",
			}
			return imageRepo
		},
		DeploymentConfigStore: deploytest.NewFakeDeploymentConfigStore(config),
	}

	// verify no-op
	controller.HandleImageRepo()
}

func TestImageChange(t *testing.T) {
	var (
		generatedConfig          *deployapi.DeploymentConfig
		updatedConfig            *deployapi.DeploymentConfig
		generatedConfigNamespace string
		updatedConfigNamespace   string
	)

	controller := &ImageChangeController{
		DeploymentConfigInterface: &testIcDeploymentConfigInterface{
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				updatedConfigNamespace = namespace
				updatedConfig = config
				return updatedConfig, nil
			},
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				generatedConfigNamespace = namespace
				generatedConfig = regeneratedConfig(namespace)
				return generatedConfig, nil
			},
		},
		NextImageRepository: func() *imageapi.ImageRepository {
			return tagUpdate()
		},
		DeploymentConfigStore: deploytest.NewFakeDeploymentConfigStore(imageChangeDeploymentConfig()),
	}

	controller.HandleImageRepo()

	if generatedConfig == nil {
		t.Fatalf("expected config generation to occur")
	}

	if updatedConfig == nil {
		t.Fatalf("expected an updated deployment config")
	} else if updatedConfig.Details == nil {
		t.Fatalf("expected config change details to be set")
	} else if updatedConfig.Details.Causes == nil {
		t.Fatalf("expected config change causes to be set")
	} else if updatedConfig.Details.Causes[0].Type != deployapi.DeploymentTriggerOnImageChange {
		t.Fatalf("expected ChangeLog details to be set to image change trigger, got %s", updatedConfig.Details.Causes[0].Type)
	}
	if generatedConfigNamespace != nonDefaultNamespace {
		t.Errorf("Expected generatedConfigNamespace %v, got %v", nonDefaultNamespace, generatedConfigNamespace)
	}
	if updatedConfigNamespace != nonDefaultNamespace {
		t.Errorf("Expected updatedConfigNamespace %v, got %v", nonDefaultNamespace, updatedConfigNamespace)
	}

	if e, a := updatedConfig.Name, generatedConfig.Name; e != a {
		t.Fatalf("expected updated config with id %s, got %s", e, a)
	}

	if e, a := updatedConfig.Name, generatedConfig.Name; e != a {
		t.Fatalf("expected updated config with id %s, got %s", e, a)
	}
}

// Utilities and convenience methods

func originalImageRepo() *imageapi.ImageRepository {
	return &imageapi.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test-image-repo", Namespace: nonDefaultNamespace},
		DockerImageRepository: "registry:8080/openshift/test-image",
		Tags: map[string]string{
			"test-tag": "ref-1",
		},
	}
}

func unregisteredTagUpdate() *imageapi.ImageRepository {
	return &imageapi.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test-image-repo", Namespace: nonDefaultNamespace},
		DockerImageRepository: "registry:8080/openshift/test-image",
		Tags: map[string]string{
			"test-tag":       "ref-1",
			"other-test-tag": "ref-x",
		},
	}
}

func tagUpdate() *imageapi.ImageRepository {
	return &imageapi.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test-image-repo", Namespace: nonDefaultNamespace},
		DockerImageRepository: "registry:8080/openshift/test-image",
		Tags: map[string]string{
			"test-tag": "ref-2",
		},
	}
}

func imageChangeDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "image-change-deploy-config"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
					Automatic:      true,
					ContainerNames: []string{"container-1"},
					RepositoryName: "registry:8080/openshift/test-image",
					Tag:            "test-tag",
				},
			},
		},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
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

func regeneratedConfig(namespace string) *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "image-change-deploy-config", Namespace: namespace},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
					Automatic:      true,
					ContainerNames: []string{"container-1"},
					RepositoryName: "registry:8080/openshift/test-image",
					Tag:            "test-tag",
				},
			},
		},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
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

func unregisteredConfig() *deployapi.DeploymentConfig {
	d := imageChangeDeploymentConfig()
	d.Triggers[0].ImageChangeParams.ContainerNames = []string{"container-3"}
	return d
}
