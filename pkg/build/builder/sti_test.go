package builder

import (
	"errors"
	"strings"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/build/api"
	s2iapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/validation"
	s2ibuild "github.com/openshift/source-to-image/pkg/build"
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

// Mock S2I builder factory implementation. Just returns mock S2I builder instances ot error (if set)
func (factory testStiBuilderFactory) Builder(config *s2iapi.Config, overrides s2ibuild.Overrides) (s2ibuild.Builder, error) {
	// if there is error set, return this error
	if factory.getStrategyErr != nil {
		return nil, factory.getStrategyErr
	}
	return testBuilder{buildError: factory.buildError}, nil
}

type testBuilder struct {
	buildError error
}

// Build is a mock implementation for STI builder, returns nil result and error if any
func (builder testBuilder) Build(config *s2iapi.Config) (*s2iapi.Result, error) {
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

// ValidateConfig is a mock implementation for config validator. returns error if set or nil
func (validator testStiConfigValidator) ValidateConfig(config *s2iapi.Config) []validation.ValidationError {
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
		Status: api.BuildStatus{
			OutputDockerImageReference: "test/test-result:latest",
		},
	}
}

func TestDockerBuildError(t *testing.T) {
	expErr := errors.New("Artificial exception: Error building")
	s2iBuilder := makeStiBuilder(expErr, nil, nil, make([]validation.ValidationError, 0))
	err := s2iBuilder.Build()
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
	s2iBuilder := makeStiBuilder(nil, nil, expErr, make([]validation.ValidationError, 0))
	err := s2iBuilder.Build()
	if err == nil {
		t.Error("Artificial error expected from build process")
	} else {
		if !strings.Contains(err.Error(), expErr.Error()) {
			t.Errorf("Artificial error expected from build process: \n Returned error: %s\n Expected error: %s", err.Error(), expErr.Error())
		}
	}
}

// Test error creating s2i builder
func TestGetStrategyError(t *testing.T) {
	expErr := errors.New("Artificial exception: config error")
	s2iBuilder := makeStiBuilder(nil, expErr, nil, make([]validation.ValidationError, 0))
	err := s2iBuilder.Build()
	if err == nil {
		t.Error("Artificial error expected from build process")
	} else {
		if !strings.Contains(err.Error(), expErr.Error()) {
			t.Errorf("Artificial error expected from build process: \n Returned error: %s\n Expected error: %s", err.Error(), expErr.Error())
		}
	}
}
