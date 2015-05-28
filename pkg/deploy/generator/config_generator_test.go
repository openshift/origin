package generator

import (
	"flag"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func init() {
	flag.Set("v", "5")
}

func TestGenerate_fromMissingDeploymentConfig(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewNotFound("deploymentConfig", id)
			},
		},
	}

	config, err := generator.Generate(kapi.NewDefaultContext(), "1234")

	if config != nil {
		t.Fatalf("Unexpected DeploymentConfig generated: %#v", config)
	}

	if err == nil {
		t.Fatalf("Expected an error")
	}
}

func TestGenerate_fromConfigWithoutTagChange(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			ISFn: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
				stream := makeStream(
					"test-image-stream",
					imageapi.DefaultImageTag,
					"registry:8080/repo1:ref1",
					"00000000000000000000000000000001",
				)

				return stream, nil
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

func TestGenerate_deprecatedFromConfigWithoutTagChange(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				config := deploytest.OkDeploymentConfig(1)
				config.Triggers[0] = deploytest.OkImageChangeTriggerDeprecated()
				return config, nil
			},
			LISFn: func(ctx kapi.Context) (*imageapi.ImageStreamList, error) {
				stream := makeStream(
					"test-image-stream",
					imageapi.DefaultImageTag,
					"registry:8080/repo1:ref1",
					"00000000000000000000000000000001",
				)
				stream.Status.DockerImageRepository = "registry:8080/repo1:ref1"
				return &imageapi.ImageStreamList{
					Items: []imageapi.ImageStream{*stream},
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

func TestGenerate_fromZeroConfigWithoutTagChange(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(0), nil
			},
			ISFn: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
				stream := makeStream(
					"test-image-stream",
					imageapi.DefaultImageTag,
					"registry:8080/repo1:ref1",
					"00000000000000000000000000000001",
				)

				return stream, nil
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

func TestGenerate_fromConfigWithUpdatedImageRef(t *testing.T) {
	newRepoName := "registry:8080/openshift/test-image@sha256:00000000000000000000000000000002"
	newImageID := "00000000000000000000000000000002"

	generator := &DeploymentConfigGenerator{
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			ISFn: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
				stream := makeStream(
					"test-image-stream",
					imageapi.DefaultImageTag,
					newRepoName,
					newImageID,
				)

				return stream, nil
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

	if e, a := newRepoName, config.Template.ControllerTemplate.Template.Spec.Containers[0].Image; e != a {
		t.Fatalf("Expected container image %s, got %s", e, a)
	}

	if e, a := newRepoName, config.Triggers[0].ImageChangeParams.LastTriggeredImage; e != a {
		t.Fatalf("Expected LastTriggeredImage %s, got %s", e, a)
	}

	if e, a := config.Details.Causes[0].ImageTrigger.Tag, imageapi.DefaultImageTag; e != a {
		t.Fatalf("Expected cause tag %s, got %s", e, a)
	}
	if e, a := config.Details.Causes[0].ImageTrigger.RepositoryName, newRepoName; e != a {
		t.Fatalf("Expected cause stream %s, got %s", e, a)
	}
}

func TestGenerate_reportsInvalidErrorWhenMissingRepo(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Client: Client{
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			ISFn: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
				return nil, kerrors.NewNotFound("ImageStream", name)
			},
		},
	}
	_, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")
	if err == nil || !kerrors.IsInvalid(err) {
		t.Fatalf("Unexpected error type: %v", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerate_reportsNotFoundErrorWhenMissingDeploymentConfig(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Client: Client{
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewNotFound("DeploymentConfig", name)
			},
			ISFn: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
				return nil, kerrors.NewNotFound("ImageStream", name)
			},
		},
	}
	_, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")
	if err == nil || !kerrors.IsNotFound(err) {
		t.Fatalf("Unexpected error type: %v", err)
	}
	if !strings.Contains(err.Error(), "DeploymentConfig \"deploy1\" not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func makeStream(name, tag, dir, image string) *imageapi.ImageStream {
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
