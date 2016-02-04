package builder

import (
	"errors"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/generate/git"
	s2iapi "github.com/openshift/source-to-image/pkg/api"
	s2ibuild "github.com/openshift/source-to-image/pkg/build"
)

// testStiBuilderFactory is a mock implementation of builderFactory.
type testStiBuilderFactory struct {
	getStrategyErr error
	buildError     error
}

// Builder implements builderFactory. It returns a mock S2IBuilder instance that
// returns specific errors.
func (factory testStiBuilderFactory) Builder(config *s2iapi.Config, overrides s2ibuild.Overrides) (s2ibuild.Builder, error) {
	// Return a strategy error if non-nil.
	if factory.getStrategyErr != nil {
		return nil, factory.getStrategyErr
	}
	return testBuilder{buildError: factory.buildError}, nil
}

// testBuilder is a mock implementation of s2iapi.Builder.
type testBuilder struct {
	buildError error
}

// Build implements s2iapi.Builder. It always returns a mocked build error.
func (builder testBuilder) Build(config *s2iapi.Config) (*s2iapi.Result, error) {
	return nil, builder.buildError
}

type testS2IBuilderConfig struct {
	errPushImage   error
	getStrategyErr error
	buildError     error
}

// newTestS2IBuilder creates a mock implementation of S2IBuilder, instrumenting
// different parts to return specific errors according to config.
func newTestS2IBuilder(config testS2IBuilderConfig) *S2IBuilder {
	return newS2IBuilder(
		&FakeDocker{
			errPushImage: config.errPushImage,
		},
		"/docker.socket",
		testclient.NewSimpleFake().Builds(""),
		makeBuild(),
		git.NewRepository(),
		testStiBuilderFactory{
			getStrategyErr: config.getStrategyErr,
			buildError:     config.buildError,
		},
		runtimeConfigValidator{},
		nil,
	)
}

func makeBuild() *api.Build {
	return &api.Build{
		Spec: api.BuildSpec{
			Source: api.BuildSource{
				Git: &api.GitBuildSource{
					URI: "http://localhost/123",
				}},
			Strategy: api.BuildStrategy{
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
	s2iBuilder := newTestS2IBuilder(testS2IBuilderConfig{
		buildError: expErr,
	})
	if err := s2iBuilder.Build(); err != expErr {
		t.Errorf("s2iBuilder.Build() = %v; want %v", err, expErr)
	}
}

func TestPushError(t *testing.T) {
	expErr := errors.New("Artificial exception: Error pushing image")
	s2iBuilder := newTestS2IBuilder(testS2IBuilderConfig{
		errPushImage: expErr,
	})
	if err := s2iBuilder.Build(); !strings.HasSuffix(err.Error(), expErr.Error()) {
		t.Errorf("s2iBuilder.Build() = %v; want %v", err, expErr)
	}
}

func TestGetStrategyError(t *testing.T) {
	expErr := errors.New("Artificial exception: config error")
	s2iBuilder := newTestS2IBuilder(testS2IBuilderConfig{
		getStrategyErr: expErr,
	})
	if err := s2iBuilder.Build(); err != expErr {
		t.Errorf("s2iBuilder.Build() = %v; want %v", err, expErr)
	}
}
