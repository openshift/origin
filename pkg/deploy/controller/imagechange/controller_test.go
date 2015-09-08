package imagechange

import (
	"flag"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapitest "github.com/openshift/origin/pkg/deploy/api/test"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func init() {
	flag.Set("v", "5")
}

// TestHandle_changeForNonAutomaticTag ensures that an image update for which
// there is a matching trigger results in a no-op due to the trigger's
// automatic flag being set to false.
func TestHandle_changeForNonAutomaticTag(t *testing.T) {
	controller := &ImageChangeController{
		deploymentConfigClient: &deploymentConfigClientImpl{
			updateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected DeploymentConfig update")
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
	tagUpdate := makeRepo(
		"test-image-repo",
		imageapi.DefaultImageTag,
		"registry:8080/openshift/test-image@sha256:00000000000000000000000000000001",
		"00000000000000000000000000000001",
	)
	err := controller.Handle(tagUpdate)

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
				t.Fatalf("unexpected DeploymentConfig update")
				return nil, nil
			},
			generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				t.Fatalf("unexpected generator call")
				return nil, nil
			},
			listDeploymentConfigsFunc: func() ([]*deployapi.DeploymentConfig, error) {
				return []*deployapi.DeploymentConfig{deployapitest.OkDeploymentConfig(0)}, nil
			},
		},
	}

	// verify no-op
	imageRepo := makeRepo(
		"test-image-repo",
		"unrecognized",
		"registry:8080/openshift/test-image@sha256:00000000000000000000000000000001",
		"00000000000000000000000000000001",
	)
	err := controller.Handle(imageRepo)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// TestHandle_matchScenarios comprehensively tests trigger definitions against
// image repo updates to ensure that the image change triggers match (or don't
// match) properly.
func TestHandle_matchScenarios(t *testing.T) {
	params := map[string]*deployapi.DeploymentTriggerImageChangeParams{
		"params.1": {
			Automatic:          true,
			ContainerNames:     []string{"container-1"},
			From:               kapi.ObjectReference{Namespace: kapi.NamespaceDefault, Name: "repoA"},
			Tag:                imageapi.DefaultImageTag,
			LastTriggeredImage: "",
		},
		"params.2": {
			Automatic:          true,
			ContainerNames:     []string{"container-1"},
			From:               kapi.ObjectReference{Name: "repoA"},
			Tag:                imageapi.DefaultImageTag,
			LastTriggeredImage: "",
		},
		"params.3": {
			Automatic:          false,
			ContainerNames:     []string{"container-1"},
			From:               kapi.ObjectReference{Namespace: kapi.NamespaceDefault, Name: "repoA"},
			Tag:                imageapi.DefaultImageTag,
			LastTriggeredImage: "",
		},
		// This tests the deprecated RepositoryName reference
		"params.4": {
			Automatic:      true,
			ContainerNames: []string{"container-1"},
			RepositoryName: "registry:8080/openshift/test-image",
			Tag:            imageapi.DefaultImageTag,
		},
		"params.5": {
			Automatic:          true,
			ContainerNames:     []string{"container-1"},
			From:               kapi.ObjectReference{Name: "repoA"},
			Tag:                imageapi.DefaultImageTag,
			LastTriggeredImage: "registry:8080/openshift/test-image@sha256:00000000000000000000000000000001",
		},
		"params.6": {
			Automatic:          true,
			ContainerNames:     []string{"container-1"},
			From:               kapi.ObjectReference{Namespace: kapi.NamespaceDefault, Name: "repoC"},
			Tag:                imageapi.DefaultImageTag,
			LastTriggeredImage: "",
		},
	}

	tagHistoryFor := func(tag, dir, image string) map[string]imageapi.TagEventList {
		return map[string]imageapi.TagEventList{
			tag: {
				Items: []imageapi.TagEvent{
					{
						DockerImageReference: dir,
						Image:                image,
					},
				},
			},
		}
	}

	updates := map[string]*imageapi.ImageStream{
		"update.1": {
			ObjectMeta: kapi.ObjectMeta{Name: "repoA", Namespace: kapi.NamespaceDefault},
			Status: imageapi.ImageStreamStatus{
				Tags: tagHistoryFor(
					imageapi.DefaultImageTag,
					"registry:8080/openshift/test-image@sha256:00000000000000000000000000000001",
					"00000000000000000000000000000001",
				),
			},
		},
		// This one includes a Status.DockerImageRepository for testing params
		// which use the deprecated RepositoryName reference
		"update.2": {
			ObjectMeta: kapi.ObjectMeta{Name: "repoA", Namespace: kapi.NamespaceDefault},
			Status: imageapi.ImageStreamStatus{
				DockerImageRepository: "registry:8080/openshift/test-image",
				Tags: tagHistoryFor(
					imageapi.DefaultImageTag,
					"registry:8080/openshift/test-image@sha256:00000000000000000000000000000001",
					"00000000000000000000000000000001",
				),
			},
		},
	}

	scenarios := []struct {
		param   string
		repo    string
		matches bool
	}{
		// Update from empty last image ID to a new one with explicit namespaces
		{"params.1", "update.1", true},
		// Update from empty last image ID to a new one with implicit namespaces
		{"params.2", "update.1", true},
		// Update from empty last image ID to a new one, but not marked automatic
		{"params.3", "update.1", false},
		// Deprecated RepositoryName reference where the update's
		// Status.DockerImageRepository field isn't yet available
		{"params.4", "update.1", false},
		// Deprecated RepositoryName reference with Status.DockerImageRepository
		// now available
		{"params.4", "update.2", true},
		// Updated image ID is equal to the last triggered ID
		{"params.5", "update.1", false},
		// Trigger repo reference doesn't match
		{"params.6", "update.1", false},
	}

	for _, s := range scenarios {
		config := deployapitest.OkDeploymentConfig(0)
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
						t.Fatalf("unexpected DeploymentConfig update for scenario: %v", s)
					}
					updated = true
					return config, nil
				},
				generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
					if !s.matches {
						t.Fatalf("unexpected generator call for scenario: %v", s)
					}
					generated = true
					// simulate a generation
					newConfig := deployapitest.OkDeploymentConfig(config.LatestVersion + 1)
					newConfig.Namespace = config.Namespace
					newConfig.Triggers = config.Triggers
					return newConfig, nil
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
	}
}

func makeRepo(name, tag, dir, image string) *imageapi.ImageStream {
	return &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: name},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				tag: {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: dir,
							Image:                image,
						},
					},
				},
			},
		},
	}
}
