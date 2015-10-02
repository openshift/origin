package builder

import (
	"errors"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/openshift/origin/pkg/build/api"
	stiapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/validation"
	"github.com/openshift/source-to-image/pkg/build"
	kapi "k8s.io/kubernetes/pkg/api"
	"testing"
)

// mock docker client
type testDockerClient struct {
	buildImageCalled, pushImageCalled, removeImageCalled boolean
	errBuildImage, errPushImage, errRemoveImage          error
}

func (client testDockerClient) BuildImage(opts docker.BuildImageOptions) error {
	client.buildImageCalled = true
	return client.errBuildImage
}

func (client testDockerClient) PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
	client.pushImageCalled = true
	return client.errPushImage
}

func (client testDockerClient) RemoveImage(name string) error {
	client.removeImageCalled = true
	return client.errRemoveImage
}

// Mock S2I Builder factory implementation
type testStiBuilderFactory struct {
	// error to return
	err error
}

// Mock S2I Config Validator implementation
type testStiConfigValidator struct {
	// error to return
	err error
}

// Mock S2I Builder implemenation
type testBuilder struct{}

// mock S2I builder factory implementation. Just returns mock S2I builder instances ot error (if set)
func (factory *testStiBuilderFactory) GetStrategy(config *stiapi.Config) (build.Builder, error) {
	// if there is error set, return this error
	if factory.err != nil {
		return nil, error
	}
	return new(testBuilder), nil
}

// mock implementation for config validator. returns error if set or nil
func (validator *testStiConfigValidator) ValidateConfig(config *stiapi.Config) []validation.ValidationError {
	return validator.err
}

// mock implementation for S2I build process. returns nil as a result and error (if set)
func (_ *testBuilder) Build(config *stiapi.Config) (*stiapi.Result, error) {
	return nil, nil
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

// test docker registry image push error
func TestError(t *testing.T) {
	build := makeBuild()
	var dockerClient testDockerClient = testDockerClient{
		errPushImage: errors.New("Artificial exception: Error pushing image"),
	}
	builder := newSTIBuilder(dockerClient, "docker.socket", build,
		new(testStiBuilderFactory), new(testStiConfigValidator))
	err := builder.Build()
	if err != nil {
		t.Error(err)
	}
}
