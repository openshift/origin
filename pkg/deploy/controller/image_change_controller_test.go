package controller

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapitest "github.com/openshift/origin/pkg/deploy/api/test"
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
	config := deployapitest.OkDeploymentConfig(1)
	config.Triggers[0].ImageChangeParams.ContainerNames = []string{"container-3"}

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
	config := deployapitest.OkDeploymentConfig(1)
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

func TestImageChangeMatchScenarios(t *testing.T) {
	params := map[string]*deployapi.DeploymentTriggerImageChangeParams{
		"params.1": {
			Automatic:      true,
			ContainerNames: []string{"container-1"},
			From:           kapi.ObjectReference{Namespace: kapi.NamespaceDefault, Name: "repoA"},
			Tag:            "test-tag",
		},
		"params.2": {
			Automatic:      true,
			ContainerNames: []string{"container-1"},
			From:           kapi.ObjectReference{Name: "repoA"},
			Tag:            "test-tag",
		},
		"params.3": {
			Automatic:      true,
			ContainerNames: []string{"container-1"},
			RepositoryName: "registry:8080/openshift/test-image",
			Tag:            "test-tag",
		},
	}

	updates := map[string]*imageapi.ImageRepository{
		"repo.1": {
			ObjectMeta: kapi.ObjectMeta{Name: "repoA", Namespace: kapi.NamespaceDefault},
			Status:     imageapi.ImageRepositoryStatus{"registry:8080/openshift/test-image"},
			Tags:       map[string]string{"test-tag": "ref-2"},
		},
		"repo.2": {
			ObjectMeta: kapi.ObjectMeta{Name: "repoB", Namespace: kapi.NamespaceDefault},
			Status:     imageapi.ImageRepositoryStatus{"registry:8080/openshift/test-image"},
			Tags:       map[string]string{"test-tag": "ref-3"},
		},
		"repo.3": {
			ObjectMeta: kapi.ObjectMeta{Name: "repoC", Namespace: kapi.NamespaceDefault},
			Status:     imageapi.ImageRepositoryStatus{"registry:8080/openshift/test-image-B"},
			Tags:       map[string]string{"test-tag": "ref-2"},
		},
		"repo.4": {
			ObjectMeta: kapi.ObjectMeta{Name: "repoA", Namespace: kapi.NamespaceDefault},
			Tags:       map[string]string{"test-tag": "ref-2"},
		},
	}

	scenarios := []struct {
		param   string
		repo    string
		matches bool
		causes  []string
	}{
		{"params.1", "repo.1", true, []string{"registry:8080/openshift/test-image:ref-2"}},
		{"params.1", "repo.2", false, []string{}},
		{"params.1", "repo.3", false, []string{}},
		// This case relies on a brittle assumption that we'll sometimes has an empty
		// imageRepo.Status.DockerImageRepository, but we'll still feed it through the
		// generator anyway (which could return a config with no diffs).
		{"params.1", "repo.4", true, []string{}},
		{"params.2", "repo.1", true, []string{"registry:8080/openshift/test-image:ref-2"}},
		{"params.2", "repo.2", false, []string{}},
		{"params.2", "repo.3", false, []string{}},
		// Same as params.1 -> repo.4, see above
		{"params.2", "repo.4", true, []string{}},
		{"params.3", "repo.1", true, []string{"registry:8080/openshift/test-image"}},
		{"params.3", "repo.2", true, []string{"registry:8080/openshift/test-image"}},
		{"params.3", "repo.3", false, []string{}},
		{"params.3", "repo.4", false, []string{}},
	}

	for _, s := range scenarios {
		config := imageChangeDeploymentConfig()
		config.Namespace = kapi.NamespaceDefault
		config.Triggers = []deployapi.DeploymentTriggerPolicy{
			{
				Type:              deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: params[s.param],
			},
		}

		updated := false
		generated := false

		controller := &ImageChangeController{
			DeploymentConfigInterface: &testIcDeploymentConfigInterface{
				UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
					if !s.matches {
						t.Fatalf("unexpected deployment config update for scenario: %v", s)
					}
					updated = true
					return config, nil
				},
				GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
					if !s.matches {
						t.Fatalf("unexpected generator call for scenario: %v", s)
					}
					generated = true
					return config, nil
				},
			},
			NextImageRepository: func() *imageapi.ImageRepository {
				return updates[s.repo]
			},
			DeploymentConfigStore: deploytest.NewFakeDeploymentConfigStore(config),
		}

		t.Logf("running scenario: %v", s)
		controller.HandleImageRepo()

		// assert updates/generations occured
		if s.matches && !updated {
			t.Fatalf("expected update for scenario: %v", s)
		}

		if s.matches && !generated {
			t.Fatalf("expected generation for scenario: %v", s)
		}

		// assert causes are correct relative to expected updates
		if updated {
			if e, a := len(s.causes), len(config.Details.Causes); e != a {
				t.Fatalf("expected cause length %d, got %d", e, a)
			}

			for i, cause := range config.Details.Causes {
				if e, a := s.causes[i], cause.ImageTrigger.RepositoryName; e != a {
					t.Fatalf("expected cause repositoryName %s, got %s", e, a)
				}
			}
		} else {
			if config.Details != nil && len(config.Details.Causes) != 0 {
				t.Fatalf("expected cause length 0, got %d", len(config.Details.Causes))
			}
		}
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
