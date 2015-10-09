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

// mock docker client
type testDockerClient struct {
	// flag if BuildImage was called
	buildImageCalled bool
	// flag if PushImage was called
	pushImageCalled bool
	// flag if RemoveImage was called
	removeImageCalled bool
	// emulated error to return from PushImage call
	errPushImage error
}

// Registers a call and returns an error (if any)
func (client testDockerClient) BuildImage(opts docker.BuildImageOptions) error {
	return nil
}

// Registers PushImage call and returns an error (if any)
func (client testDockerClient) PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
	client.pushImageCalled = true
	return client.errPushImage
}

// Registers RemoveImage call and returns an error (if any)
func (client testDockerClient) RemoveImage(name string) error {
	return nil
}

// Mock S2I Builder factory implementation
type testStiBuilderFactory struct {
	// error to return from GetStrategy function
	getStrategyErr error
	// error to return from Build function
	buildError error
}

// Mock S2I Config Validator implementation
type testStiConfigValidator struct {
	// errors to return
	errors []validation.ValidationError
}

// Mock S2I builder factory implementation. Just returns mock S2I builder instances ot error (if set)
func (factory testStiBuilderFactory) GetStrategy(config *stiapi.Config) (build.Builder, error) {
	// if there is error set, return this error
	if factory.getStrategyErr != nil {
		return nil, factory.getStrategyErr
	}
	return testBuilder{buildError: factory.buildError}, nil
}

// mock STI builder
type testBuilder struct {
	// error to return from build process
	buildError error
}

// provide mock implementation for STI builder, returns nil result and error if any
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

// Mock implementation for config validator. returns error if set or nil
func (validator testStiConfigValidator) ValidateConfig(config *stiapi.Config) []validation.ValidationError {
	return validator.errors
}

// create simple mock build config
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

// Test docker registry image build error
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

// Test docker registry image push error
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

// Test error creating sti builder
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
