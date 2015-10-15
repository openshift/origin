package builder

import (
	"errors"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/openshift/origin/pkg/build/api"
	stiapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/validation"
	"github.com/openshift/source-to-image/pkg/build"
	kapi "k8s.io/kubernetes/pkg/api"
	"strings"
	"testing"
)

type testDockerClient struct {
	buildImageCalled  bool
	pushImageCalled   bool
	removeImageCalled bool
	errPushImage      error
}

func (client testDockerClient) BuildImage(opts docker.BuildImageOptions) error {
	return nil
}

func (client testDockerClient) PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
	client.pushImageCalled = true
	return client.errPushImage
}

func (client testDockerClient) RemoveImage(name string) error {
	return nil
}

type testStiBuilderFactory struct {
	getStrategyErr error
	buildError     error
}

type testStiConfigValidator struct {
	errors []validation.ValidationError
}

func (factory testStiBuilderFactory) GetStrategy(config *stiapi.Config) (build.Builder, error) {
	if factory.getStrategyErr != nil {
		return nil, factory.getStrategyErr
	}
	return testBuilder{buildError: factory.buildError}, nil
}

type testBuilder struct {
	buildError error
}

func (builder testBuilder) Build(config *stiapi.Config) (*stiapi.Result, error) {
	return nil, builder.buildError
}

// creates mock implemenation of STI builder, instrumenting different parts of a process to return errors
func makeStiBuilder(
	errPushImage error,
	getStrategyErr error,
	buildError error,
	validationErrors []validation.ValidationError) STIBuilder {
	return *newSTIBuilder(
		testDockerClient{
			errPushImage: errPushImage,
		},
		"/docker.socket",
		makeBuild(),
		testStiBuilderFactory{getStrategyErr: getStrategyErr, buildError: buildError},
		testStiConfigValidator{errors: validationErrors},
	)
}

func (validator testStiConfigValidator) ValidateConfig(config *stiapi.Config) []validation.ValidationError {
	return validator.errors
}

func makeBuild() *api.Build {
	return &api.Build{
		Spec: api.BuildSpec{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					URI: "http://localhost/123",
				}},
			Strategy: api.BuildStrategy{
				Type: api.SourceBuildStrategyType,
				SourceStrategy: &api.SourceBuildStrategy{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "test/builder:latest",
					},
					Incremental: true,
				}},
			Output: api.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "test/test-result:latest",
				},
			},
		},
	}
}

func TestDockerBuildError(t *testing.T) {
	expErr := errors.New("Artificial exception: Error building")
	stiBuilder := makeStiBuilder(expErr, nil, nil, make([]validation.ValidationError, 0))
	err := stiBuilder.Build()
	if err == nil {
		t.Error("Artificial error expected from build process")
	} else {
		if !strings.Contains(err.Error(), expErr.Error()) {
			t.Errorf("Artificial error expected from build process: \n Returned error: %s\n Expected error: %s", err.Error(), expErr.Error())
		}
	}
}

func TestPushError(t *testing.T) {
	expErr := errors.New("Artificial exception: Error pushing image")
	stiBuilder := makeStiBuilder(nil, nil, expErr, make([]validation.ValidationError, 0))
	err := stiBuilder.Build()
	if err == nil {
		t.Error("Artificial error expected from build process")
	} else {
		if !strings.Contains(err.Error(), expErr.Error()) {
			t.Errorf("Artificial error expected from build process: \n Returned error: %s\n Expected error: %s", err.Error(), expErr.Error())
		}
	}
}

func TestGetStrategyError(t *testing.T) {
	expErr := errors.New("Artificial exception: config error")
	stiBuilder := makeStiBuilder(nil, expErr, nil, make([]validation.ValidationError, 0))
	err := stiBuilder.Build()
	if err == nil {
		t.Error("Artificial error expected from build process")
	} else {
		if !strings.Contains(err.Error(), expErr.Error()) {
			t.Errorf("Artificial error expected from build process: \n Returned error: %s\n Expected error: %s", err.Error(), expErr.Error())
		}
	}
}
