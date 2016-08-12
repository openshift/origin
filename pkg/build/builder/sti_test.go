package builder

import (
	"errors"
	"reflect"
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
	t := true
	return &api.Build{
		Spec: api.BuildSpec{
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						URI: "http://localhost/123",
					}},
				Strategy: api.BuildStrategy{
					SourceStrategy: &api.SourceBuildStrategy{
						Env: append([]kapi.EnvVar{},
							kapi.EnvVar{
								Name:  "HTTPS_PROXY",
								Value: "https://test/secure:8443",
							}, kapi.EnvVar{
								Name:  "HTTP_PROXY",
								Value: "http://test/insecure:8080",
							}),
						From: kapi.ObjectReference{
							Kind: "DockerImage",
							Name: "test/builder:latest",
						},
						Incremental: &t,
					}},
				Output: api.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "test/test-result:latest",
					},
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

func TestCopyToVolumeList(t *testing.T) {
	newArtifacts := []api.ImageSourcePath{
		{
			SourcePath:     "/path/to/source",
			DestinationDir: "path/to/destination",
		},
	}
	volumeList := s2iapi.VolumeList{
		s2iapi.VolumeSpec{
			Source:      "/path/to/source",
			Destination: "path/to/destination",
		},
	}
	newVolumeList := copyToVolumeList(newArtifacts)
	if !reflect.DeepEqual(volumeList, newVolumeList) {
		t.Errorf("Expected artifacts mapping to match %#v, got %#v instead!", volumeList, newVolumeList)
	}
}

func TestBuildEnvVars(t *testing.T) {
	// In order not complicate this function, the ordering of the expected
	// EnvironmentList structure and the one that is returned must match,
	// otherwise the DeepEqual comparison will fail, since EnvironmentList is a
	// []EnvironmentSpec and list ordering in DeepEqual matters.
	expectedEnvList := s2iapi.EnvironmentList{
		s2iapi.EnvironmentSpec{
			Name:  "OPENSHIFT_BUILD_NAME",
			Value: "openshift-test-1-build",
		}, s2iapi.EnvironmentSpec{
			Name:  "OPENSHIFT_BUILD_NAMESPACE",
			Value: "openshift-demo",
		}, s2iapi.EnvironmentSpec{
			Name:  "OPENSHIFT_BUILD_SOURCE",
			Value: "http://localhost/123",
		}, s2iapi.EnvironmentSpec{
			Name:  "HTTPS_PROXY",
			Value: "https://test/secure:8443",
		}, s2iapi.EnvironmentSpec{
			Name:  "HTTP_PROXY",
			Value: "http://test/insecure:8080",
		},
	}

	mockBuild := makeBuild()
	mockBuild.Name = "openshift-test-1-build"
	mockBuild.Namespace = "openshift-demo"
	resultedEnvList := buildEnvVars(mockBuild)
	if !reflect.DeepEqual(expectedEnvList, resultedEnvList) {
		t.Errorf("Expected EnvironmentList to match: %#v, got %#v", expectedEnvList, resultedEnvList)
	}
}

func TestScriptProxyConfig(t *testing.T) {
	newBuild := &api.Build{
		Spec: api.BuildSpec{
			CommonSpec: api.CommonSpec{
				Strategy: api.BuildStrategy{
					SourceStrategy: &api.SourceBuildStrategy{
						Env: append([]kapi.EnvVar{}, kapi.EnvVar{
							Name:  "HTTPS_PROXY",
							Value: "https://test/secure",
						}, kapi.EnvVar{
							Name:  "HTTP_PROXY",
							Value: "http://test/insecure",
						}),
					},
				},
			},
		},
	}
	resultedProxyConf, err := scriptProxyConfig(newBuild)
	if err != nil {
		t.Fatalf("An error occured while parsing the proxy config: %v", err)
	}
	if resultedProxyConf.HTTPProxy.Path != "/insecure" {
		t.Errorf("Expected HTTP Proxy path to be /insecure, got: %v", resultedProxyConf.HTTPProxy.Path)
	}
	if resultedProxyConf.HTTPSProxy.Path != "/secure" {
		t.Errorf("Expected HTTPS Proxy path to be /secure, got: %v", resultedProxyConf.HTTPSProxy.Path)
	}
}
