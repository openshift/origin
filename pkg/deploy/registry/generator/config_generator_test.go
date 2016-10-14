package generator

import (
	"flag"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"

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
				return nil, kerrors.NewNotFound(deployapi.Resource("DeploymentConfig"), id)
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
					"sha256:0000000000000000000000000000000000000000000000000000000000000001",
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

	if config.Status.LatestVersion != 1 {
		t.Fatalf("Expected config LatestVersion=1, got %d", config.Status.LatestVersion)
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
					"sha256:0000000000000000000000000000000000000000000000000000000000000001",
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

	if config.Status.LatestVersion != 1 {
		t.Fatalf("Expected config LatestVersion=1, got %d", config.Status.LatestVersion)
	}
}

func TestGenerate_fromConfigWithUpdatedImageRef(t *testing.T) {
	newRepoName := "registry:8080/openshift/test-image@sha256:0000000000000000000000000000000000000000000000000000000000000002"
	streamName := "test-image-stream"
	newImageID := "sha256:0000000000000000000000000000000000000000000000000000000000000002"

	generator := &DeploymentConfigGenerator{
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			ISFn: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
				stream := makeStream(
					streamName,
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

	if config.Status.LatestVersion != 2 {
		t.Fatalf("Expected config LatestVersion=2, got %d", config.Status.LatestVersion)
	}

	if expected, actual := newRepoName, config.Spec.Template.Spec.Containers[0].Image; actual != expected {
		t.Fatalf("Expected container image %q, got %q", expected, actual)
	}

	if expected, actual := newRepoName, config.Spec.Triggers[0].ImageChangeParams.LastTriggeredImage; actual != expected {
		t.Fatalf("Expected LastTriggeredImage %q, got %q", expected, actual)
	}

	name, tag, _ := imageapi.SplitImageStreamTag(config.Status.Details.Causes[0].ImageTrigger.From.Name)
	if actual, expected := tag, imageapi.DefaultImageTag; actual != expected {
		t.Fatalf("Expected cause tag %q, got %q", expected, actual)
	}
	if actual, expected := name, streamName; actual != expected {
		t.Fatalf("Expected cause stream %q, got %q", expected, actual)
	}
}

func TestGenerate_reportsInvalidErrorWhenMissingRepo(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Client: Client{
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			ISFn: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
				return nil, kerrors.NewNotFound(imageapi.Resource("ImageStream"), name)
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
				return nil, kerrors.NewNotFound(deployapi.Resource("DeploymentConfig"), name)
			},
			ISFn: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
				return nil, kerrors.NewNotFound(imageapi.Resource("ImageStream"), name)
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
