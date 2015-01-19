package generator

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestGenerateFromMissingDeploymentConfig(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
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
		Codec: api.Codec,
		DeploymentConfigInterface: &testDeploymentConfigInterface{
			GetDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
		},
		ImageRepositoryInterface: &testImageRepositoryInterface{
			ListImageRepositoriesFunc: func(labels labels.Selector) (*imageapi.ImageRepositoryList, error) {
				return okImageRepoList(), nil
			},
		},
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(id string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
				return deployment, nil
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
		Codec: api.Codec,
		DeploymentConfigInterface: &testDeploymentConfigInterface{
			GetDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
		},
		ImageRepositoryInterface: &testImageRepositoryInterface{
			ListImageRepositoriesFunc: func(labels labels.Selector) (*imageapi.ImageRepositoryList, error) {
				return okImageRepoList(), nil
			},
		},
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(id string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("replicationController", id)
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
		Codec: api.Codec,
		DeploymentConfigInterface: &testDeploymentConfigInterface{
			GetDeploymentConfigFunc: func(id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
		},
		ImageRepositoryInterface: &testImageRepositoryInterface{
			ListImageRepositoriesFunc: func(labels labels.Selector) (*imageapi.ImageRepositoryList, error) {
				list := okImageRepoList()
				list.Items[0].Tags["tag1"] = "ref2"
				return list, nil
			},
		},
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(id string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
				return deployment, nil
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
	actual := config.Template.ControllerTemplate.Template.Spec.Containers[0].Image
	if expected != actual {
		t.Fatalf("Expected container image %s, got %s", expected, actual)
	}
}

type testDeploymentInterface struct {
	GetDeploymentFunc func(id string) (*kapi.ReplicationController, error)
}

func (i *testDeploymentInterface) GetDeployment(ctx kapi.Context, id string) (*kapi.ReplicationController, error) {
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

func okImageRepoList() *imageapi.ImageRepositoryList {
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
