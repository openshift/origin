package imagechange

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapitest "github.com/openshift/origin/pkg/deploy/api/test"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	nonDefaultNamespace = "nondefaultnamespace"
)

// TestHandle_unregisteredContainer ensures that an image update for which
// there is a trigger defined results in a no-op due to the config's
// containers not matching the trigger's containers.
func TestHandle_unregisteredContainer(t *testing.T) {
	controller := &ImageChangeController{
		deploymentConfigClient: &deploymentConfigClientImpl{
			updateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected deployment config update")
				return nil, nil
			},
			generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected generator call")
				return nil, nil
			},
			listDeploymentConfigsFunc: func() ([]*deployapi.DeploymentConfig, error) {
				config := deployapitest.OkDeploymentConfig(1)
				config.Triggers[0].ImageChangeParams.ContainerNames = []string{"container-3"}

				return []*deployapi.DeploymentConfig{config}, nil
			},
		},
	}

	// verify no-op
	err := controller.Handle(tagUpdate())

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// TestHandle_changeForNonAutomaticTag ensures that an image update for which
// there is a matching trigger results in a no-op due to the trigger's
// automatic flag being set to false.
func TestHandle_changeForNonAutomaticTag(t *testing.T) {
	controller := &ImageChangeController{
		deploymentConfigClient: &deploymentConfigClientImpl{
			updateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected deployment config update")
				return nil, nil
			},
			generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected generator call")
				return nil, nil
			},
			listDeploymentConfigsFunc: func() ([]*deployapi.DeploymentConfig, error) {
				config := deployapitest.OkDeploymentConfig(1)
				config.Triggers[0].ImageChangeParams.Automatic = false

				return []*deployapi.DeploymentConfig{config}, nil
			},
		},
	}

	// verify no-op
	err := controller.Handle(tagUpdate())

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// TestHandle_changeForUnregisteredTag ensures that an image update for which
// there is a matching trigger results in a no-op due to the tag specified on
// the trigger not matching the tags defined on the image repo.
func TestHandle_changeForUnregisteredTag(t *testing.T) {
	controller := &ImageChangeController{
		deploymentConfigClient: &deploymentConfigClientImpl{
			updateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected deployment config update")
				return nil, nil
			},
			generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected generator call")
				return nil, nil
			},
			listDeploymentConfigsFunc: func() ([]*deployapi.DeploymentConfig, error) {
				return []*deployapi.DeploymentConfig{imageChangeDeploymentConfig()}, nil
			},
		},
	}

	// verify no-op
	imageRepo := tagUpdate()
	imageRepo.Tags = map[string]string{
		"unknown-tag": "ref-1",
	}
	err := controller.Handle(imageRepo)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// TestHande_matchScenarios comprehensively tests trigger definitions against
// image repo updates to ensure that the image change triggers match (or don't
// match) properly.
func TestHande_matchScenarios(t *testing.T) {
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

	tagHistoryFor := func(repo, tag, value string) map[string]imageapi.TagEventList {
		return map[string]imageapi.TagEventList{
			tag: {
				Items: []imageapi.TagEvent{
					{
						DockerImageReference: fmt.Sprintf("%s:%s", repo, value),
						Image:                value,
					},
				},
			},
		}
	}

	updates := map[string]*imageapi.ImageRepository{
		"repo.1": {
			ObjectMeta: kapi.ObjectMeta{Name: "repoA", Namespace: kapi.NamespaceDefault},
			Status: imageapi.ImageRepositoryStatus{
				DockerImageRepository: "registry:8080/openshift/test-image",
				Tags: tagHistoryFor("registry:8080/openshift/test-image", "test-tag", "ref-2"),
			},
			Tags: map[string]string{"test-tag": "ref-2"},
		},
		"repo.2": {
			ObjectMeta: kapi.ObjectMeta{Name: "repoB", Namespace: kapi.NamespaceDefault},
			Status: imageapi.ImageRepositoryStatus{
				DockerImageRepository: "registry:8080/openshift/test-image",
				Tags: tagHistoryFor("registry:8080/openshift/test-image", "test-tag", "ref-3"),
			},
			Tags: map[string]string{"test-tag": "ref-3"},
		},
		"repo.3": {
			ObjectMeta: kapi.ObjectMeta{Name: "repoC", Namespace: kapi.NamespaceDefault},
			Status: imageapi.ImageRepositoryStatus{
				DockerImageRepository: "registry:8080/openshift/test-image-B",
				Tags: tagHistoryFor("registry:8080/openshift/test-image-B", "test-tag", "ref-2"),
			},
			Tags: map[string]string{"test-tag": "ref-2"},
		},
		"repo.4": {
			ObjectMeta: kapi.ObjectMeta{Name: "repoA", Namespace: kapi.NamespaceDefault},
			Status: imageapi.ImageRepositoryStatus{
				Tags: tagHistoryFor("default/repoA", "test-tag", "ref-2"),
			},
			Tags: map[string]string{"test-tag": "ref-2"},
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
			deploymentConfigClient: &deploymentConfigClientImpl{
				updateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
					if !s.matches {
						t.Fatalf("unexpected deployment config update for scenario: %v", s)
					}
					updated = true
					return config, nil
				},
				generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
					if !s.matches {
						t.Fatalf("unexpected generator call for scenario: %v", s)
					}
					generated = true
					return config, nil
				},
				listDeploymentConfigsFunc: func() ([]*deployapi.DeploymentConfig, error) {
					return []*deployapi.DeploymentConfig{config}, nil
				},
			},
		}

		t.Logf("running scenario: %v", s)
		err := controller.Handle(updates[s.repo])

		if err != nil {
			t.Fatalf("unexpected error for scenario %v: %v", s, err)
		}

		// assert updates/generations occurred
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
