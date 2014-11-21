package generator

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestGenerateFromMissingDeploymentConfig(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		DeploymentConfigInterface: &testDeploymentConfigInterface{
			GetDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewNotFound("deploymentConfig", id)
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "1234")

	if config != nil {
		t.Fatalf("Unexpected deployment config generated: %#v", config)
	}

	if err == nil {
		t.Fatalf("Expected an error")
	}
}

func TestGenerateFromConfigWithoutTagChange(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		DeploymentConfigInterface: &testDeploymentConfigInterface{
			GetDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				return basicDeploymentConfig(), nil
			},
		},
		ImageRepositoryInterface: &testImageRepositoryInterface{
			ListImageRepositoriesFunc: func(labels labels.Selector) (*imageapi.ImageRepositoryList, error) {
				return basicImageRepo(), nil
			},
		},
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(id string) (*deployapi.Deployment, error) {
				return &deployapi.Deployment{
					ControllerTemplate: kapi.ReplicationControllerState{
						PodTemplate: basicPodTemplate(),
					},
				}, nil
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config == nil {
		t.Fatalf("Expected non-nil config")
	}

	if config.LatestVersion != 1 {
		t.Fatalf("Expected config LatestVersion=1, got %d", config.LatestVersion)
	}
}

func TestGenerateFromConfigWithNoDeployment(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		DeploymentConfigInterface: &testDeploymentConfigInterface{
			GetDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				return basicDeploymentConfig(), nil
			},
		},
		ImageRepositoryInterface: &testImageRepositoryInterface{
			ListImageRepositoriesFunc: func(labels labels.Selector) (*imageapi.ImageRepositoryList, error) {
				return basicImageRepo(), nil
			},
		},
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(id string) (*deployapi.Deployment, error) {
				return nil, kerrors.NewNotFound("deployment", id)
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "deploy2")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config == nil {
		t.Fatalf("Expected non-nil config")
	}

	if config.LatestVersion != 1 {
		t.Fatalf("Expected config LatestVersion=1, got %d", config.LatestVersion)
	}
}

func TestGenerateFromConfigWithUpdatedImageRef(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		DeploymentConfigInterface: &testDeploymentConfigInterface{
			GetDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				return basicDeploymentConfig(), nil
			},
		},
		ImageRepositoryInterface: &testImageRepositoryInterface{
			ListImageRepositoriesFunc: func(labels labels.Selector) (*imageapi.ImageRepositoryList, error) {
				return updatedImageRepo(), nil
			},
		},
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(id string) (*deployapi.Deployment, error) {
				return &deployapi.Deployment{
					ControllerTemplate: kapi.ReplicationControllerState{
						PodTemplate: basicPodTemplate(),
					},
				}, nil
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config == nil {
		t.Fatalf("Expected non-nil config")
	}

	if config.LatestVersion != 2 {
		t.Fatalf("Expected config LatestVersion=2, got %d", config.LatestVersion)
	}

	expected := "registry:8080/repo1:ref2"
	actual := config.Template.ControllerTemplate.PodTemplate.DesiredState.Manifest.Containers[0].Image
	if expected != actual {
		t.Fatalf("Expected container image %s, got %s", expected, actual)
	}
}

type testDeploymentInterface struct {
	GetDeploymentFunc func(id string) (*deployapi.Deployment, error)
}

func (i *testDeploymentInterface) GetDeployment(ctx kapi.Context, id string) (*deployapi.Deployment, error) {
	return i.GetDeploymentFunc(id)
}

type testDeploymentConfigInterface struct {
	GetDeploymentConfigFunc func(id string) (*deployapi.DeploymentConfig, error)
}

func (i *testDeploymentConfigInterface) GetDeploymentConfig(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
	return i.GetDeploymentConfigFunc(id)
}

type testImageRepositoryInterface struct {
	ListImageRepositoriesFunc func(labels labels.Selector) (*imageapi.ImageRepositoryList, error)
}

func (i *testImageRepositoryInterface) ListImageRepositories(ctx kapi.Context, labels labels.Selector) (*imageapi.ImageRepositoryList, error) {
	return i.ListImageRepositoriesFunc(labels)
}

func basicPodTemplate() kapi.PodTemplate {
	return kapi.PodTemplate{
		DesiredState: kapi.PodState{
			Manifest: kapi.ContainerManifest{
				Containers: []kapi.Container{
					{
						Name:  "container1",
						Image: "registry:8080/repo1:ref1",
					},
					{
						Name:  "container2",
						Image: "registry:8080/repo1:ref2",
					},
				},
			},
		},
	}
}
func basicDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta:    kapi.ObjectMeta{Name: "deploy1"},
		LatestVersion: 1,
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
					ContainerNames: []string{
						"container1",
					},
					RepositoryName: "registry:8080/repo1",
					Tag:            "tag1",
				},
			},
		},
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerState{
				PodTemplate: basicPodTemplate(),
			},
		},
	}
}

func basicImageRepo() *imageapi.ImageRepositoryList {
	return &imageapi.ImageRepositoryList{
		Items: []imageapi.ImageRepository{
			{
				ObjectMeta:            kapi.ObjectMeta{Name: "imageRepo1"},
				DockerImageRepository: "registry:8080/repo1",
				Tags: map[string]string{
					"tag1": "ref1",
				},
			},
		},
	}
}

func updatedImageRepo() *imageapi.ImageRepositoryList {
	return &imageapi.ImageRepositoryList{
		Items: []imageapi.ImageRepository{
			{
				ObjectMeta:            kapi.ObjectMeta{Name: "imageRepo1"},
				DockerImageRepository: "registry:8080/repo1",
				Tags: map[string]string{
					"tag1": "ref2",
				},
			},
		},
	}
}
