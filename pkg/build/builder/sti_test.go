package builder

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildfake "github.com/openshift/client-go/build/clientset/versioned/fake"
	"github.com/openshift/origin/pkg/git"
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
func (factory testStiBuilderFactory) Builder(config *s2iapi.Config, overrides s2ibuild.Overrides) (s2ibuild.Builder, s2iapi.BuildInfo, error) {
	// Return a strategy error if non-nil.
	if factory.getStrategyErr != nil {
		return nil, s2iapi.BuildInfo{}, factory.getStrategyErr
	}
	return testBuilder{buildError: factory.buildError}, s2iapi.BuildInfo{}, nil
}

// testBuilder is a mock implementation of s2iapi.Builder.
type testBuilder struct {
	buildError error
}

// Build implements s2iapi.Builder. It always returns a mocked build error.
func (builder testBuilder) Build(config *s2iapi.Config) (*s2iapi.Result, error) {
	return &s2iapi.Result{
		BuildInfo: s2iapi.BuildInfo{},
	}, builder.buildError
}

type testS2IBuilderConfig struct {
	errPushImage   error
	getStrategyErr error
	buildError     error
}

// newTestS2IBuilder creates a mock implementation of S2IBuilder, instrumenting
// different parts to return specific errors according to config.
func newTestS2IBuilder(config testS2IBuilderConfig) *S2IBuilder {
	client := &buildfake.Clientset{}
	return newS2IBuilder(
		&FakeDocker{
			errPushImage: config.errPushImage,
		},
		"unix:///var/run/docker2.sock",
		client.Build().Builds(""),
		makeBuild(),
		testStiBuilderFactory{
			getStrategyErr: config.getStrategyErr,
			buildError:     config.buildError,
		},
		runtimeConfigValidator{},
		nil,
	)
}

func makeBuild() *buildapiv1.Build {
	t := true
	return &buildapiv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-1",
			Namespace: "ns",
		},
		Spec: buildapiv1.BuildSpec{
			CommonSpec: buildapiv1.CommonSpec{
				Source: buildapiv1.BuildSource{},
				Strategy: buildapiv1.BuildStrategy{
					SourceStrategy: &buildapiv1.SourceBuildStrategy{
						Env: append([]corev1.EnvVar{},
							corev1.EnvVar{
								Name:  "HTTPS_PROXY",
								Value: "https://test/secure:8443",
							}, corev1.EnvVar{
								Name:  "HTTP_PROXY",
								Value: "http://test/insecure:8080",
							}),
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "test/builder:latest",
						},
						Incremental: &t,
					}},
				Output: buildapiv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "test/test-result:latest",
					},
				},
			},
		},
		Status: buildapiv1.BuildStatus{
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
	newArtifacts := []buildapiv1.ImageSourcePath{
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
			Name:  "OPENSHIFT_BUILD_COMMIT",
			Value: "1575a90c569a7cc0eea84fbd3304d9df37c9f5ee",
		}, s2iapi.EnvironmentSpec{
			Name:  "HTTPS_PROXY",
			Value: "https://test/secure:8443",
		}, s2iapi.EnvironmentSpec{
			Name:  "HTTP_PROXY",
			Value: "http://test/insecure:8080",
		},
	}
	expectedLabelMap := map[string]string{
		"io.openshift.build.commit.id": "1575a90c569a7cc0eea84fbd3304d9df37c9f5ee",
		"io.openshift.build.name":      "openshift-test-1-build",
		"io.openshift.build.namespace": "openshift-demo",
	}

	mockBuild := makeBuild()
	mockBuild.Name = "openshift-test-1-build"
	mockBuild.Namespace = "openshift-demo"
	mockBuild.Spec.Source.Git = &buildapiv1.GitBuildSource{URI: "http://localhost/123"}
	sourceInfo := &git.SourceInfo{}
	sourceInfo.CommitID = "1575a90c569a7cc0eea84fbd3304d9df37c9f5ee"
	resultedEnvList := buildEnvVars(mockBuild, sourceInfo)
	if !reflect.DeepEqual(expectedEnvList, resultedEnvList) {
		t.Errorf("Expected EnvironmentList to match:\n%#v\ngot:\n%#v", expectedEnvList, resultedEnvList)
	}

	resultedLabelList := buildLabels(mockBuild, sourceInfo)
	resultedLabelMap := map[string]string{}
	for _, label := range resultedLabelList {
		resultedLabelMap[label.Key] = label.Value
	}
	if !reflect.DeepEqual(expectedLabelMap, resultedLabelMap) {
		t.Errorf("Expected LabelList to match:\n%#v\ngot:\n%#v", expectedLabelMap, resultedLabelMap)
	}

}

func TestScriptProxyConfig(t *testing.T) {
	newBuild := &buildapiv1.Build{
		Spec: buildapiv1.BuildSpec{
			CommonSpec: buildapiv1.CommonSpec{
				Strategy: buildapiv1.BuildStrategy{
					SourceStrategy: &buildapiv1.SourceBuildStrategy{
						Env: append([]corev1.EnvVar{}, corev1.EnvVar{
							Name:  "HTTPS_PROXY",
							Value: "https://test/secure",
						}, corev1.EnvVar{
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
		t.Fatalf("An error occurred while parsing the proxy config: %v", err)
	}
	if resultedProxyConf.HTTPProxy.Path != "/insecure" {
		t.Errorf("Expected HTTP Proxy path to be /insecure, got: %v", resultedProxyConf.HTTPProxy.Path)
	}
	if resultedProxyConf.HTTPSProxy.Path != "/secure" {
		t.Errorf("Expected HTTPS Proxy path to be /secure, got: %v", resultedProxyConf.HTTPSProxy.Path)
	}
}
