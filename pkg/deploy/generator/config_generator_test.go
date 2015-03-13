package generator

import (
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestGenerateFromMissingDeploymentConfig(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
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
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			LIRFn: func(ctx kapi.Context) (*imageapi.ImageRepositoryList, error) {
				return okImageRepoList(), nil
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

func TestGenerateFromZeroConfigWithoutTagChange(t *testing.T) {
	dc := basicDeploymentConfig()
	dc.LatestVersion = 0
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return dc, nil
			},
			LIRFn: func(ctx kapi.Context) (*imageapi.ImageRepositoryList, error) {
				return okImageRepoList(), nil
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
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			LIRFn: func(ctx kapi.Context) (*imageapi.ImageRepositoryList, error) {
				return okImageRepoList(), nil
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
		Client: Client{
			DCFn: func(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
			LIRFn: func(ctx kapi.Context) (*imageapi.ImageRepositoryList, error) {
				list := okImageRepoList()
				list.Items[0].Tags["tag1"] = "ref2"
				list.Items[0].Status.Tags["tag1"].Items[0].Image = "ref2"
				list.Items[0].Status.Tags["tag1"].Items[0].DockerImageReference = "registry:8080/repo1@ref2"
				return list, nil
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

	expected := "registry:8080/repo1@ref2"
	actual := config.Template.ControllerTemplate.Template.Spec.Containers[0].Image
	if expected != actual {
		t.Fatalf("Expected container image %s, got %s", expected, actual)
	}
}

func TestGenerateReportsInvalidErrorWhenMissingRepo(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return referenceDeploymentConfig(), nil
			},
			IRFn: func(ctx kapi.Context, name string) (*imageapi.ImageRepository, error) {
				return nil, kerrors.NewNotFound("ImageRepository", name)
			},
		},
	}
	_, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")
	if err == nil || !kerrors.IsInvalid(err) {
		t.Fatalf("Unexpected error type: %v", err)
	}
	if !strings.Contains(err.Error(), "not found 'repo1'") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerateReportsNotFoundErrorWhenMissingDeploymentConfig(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewNotFound("DeploymentConfig", name)
			},
			IRFn: func(ctx kapi.Context, name string) (*imageapi.ImageRepository, error) {
				return nil, kerrors.NewNotFound("ImageRepository", name)
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
func TestGenerateReportsErrorWhenRepoHasNoImage(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return referenceDeploymentConfig(), nil
			},
			IRFn: func(ctx kapi.Context, name string) (*imageapi.ImageRepository, error) {
				return &emptyImageRepo().Items[0], nil
			},
		},
	}
	_, err := generator.Generate(kapi.NewDefaultContext(), "deploy1")
	if err == nil || !kerrors.IsInvalid(err) {
		t.Fatalf("Unexpected error type: %v", err)
	}
	if !strings.Contains(err.Error(), "image repository /imageRepo1 does not have a Docker") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerateDeploymentConfigWithFrom(t *testing.T) {
	generator := &DeploymentConfigGenerator{
		Codec: api.Codec,
		Client: Client{
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return referenceDeploymentConfig(), nil
			},
			IRFn: func(ctx kapi.Context, name string) (*imageapi.ImageRepository, error) {
				return &internalImageRepo().Items[0], nil
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

	expected := "internal/namespace/imageRepo1@ref1"
	actual := config.Template.ControllerTemplate.Template.Spec.Containers[0].Image
	if expected != actual {
		t.Fatalf("Expected container image %s, got %s", expected, actual)
	}
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
				Status: imageapi.ImageRepositoryStatus{
					DockerImageRepository: "registry:8080/repo1",
					Tags: map[string]imageapi.TagEventList{
						"tag1": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: "registry:8080/repo1:ref1",
									Image:                "ref1",
								},
							},
						},
					},
				},
			},
		},
	}
}

func basicPodTemplate() *kapi.PodTemplateSpec {
	return &kapi.PodTemplateSpec{
		Spec: kapi.PodSpec{
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
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Template: basicPodTemplate(),
			},
		},
	}
}

func referenceDeploymentConfig() *deployapi.DeploymentConfig {
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
					From: kapi.ObjectReference{
						Name: "repo1",
					},
					Tag: "tag1",
				},
			},
		},
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Template: basicPodTemplate(),
			},
		},
	}
}

func basicDeployment() *kapi.ReplicationController {
	config := basicDeploymentConfig()
	encodedConfig, _ := deployutil.EncodeDeploymentConfig(config, api.Codec)

	return &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: deployutil.LatestDeploymentNameForConfig(config),
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation:        config.Name,
				deployapi.DeploymentStatusAnnotation:        string(deployapi.DeploymentStatusNew),
				deployapi.DeploymentEncodedConfigAnnotation: encodedConfig,
			},
			Labels: config.Labels,
		},
		Spec: kapi.ReplicationControllerSpec{
			Template: basicPodTemplate(),
		},
	}
}

func internalImageRepo() *imageapi.ImageRepositoryList {
	return &imageapi.ImageRepositoryList{
		Items: []imageapi.ImageRepository{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "imageRepo1"},
				Tags: map[string]string{
					"tag1": "ref1",
				},
				Status: imageapi.ImageRepositoryStatus{
					DockerImageRepository: "internal/namespace/imageRepo1",
					Tags: map[string]imageapi.TagEventList{
						"tag1": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: "internal/namespace/imageRepo1@ref1",
									Image:                "ref1",
								},
							},
						},
					},
				},
			},
		},
	}
}

func emptyImageRepo() *imageapi.ImageRepositoryList {
	return &imageapi.ImageRepositoryList{
		Items: []imageapi.ImageRepository{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "imageRepo1"},
				Tags: map[string]string{
					"tag1": "ref1",
				},
				Status: imageapi.ImageRepositoryStatus{
					DockerImageRepository: "",
				},
			},
		},
	}
}
