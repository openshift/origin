package util

import (
	"fmt"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	originalImage = "originalImage"
	newImage      = originalImage + ":" + newTag

	tagName          = "test"
	unmatchedTagName = "unmatched"

	// immutable imageid associated w/ test tag
	newTag = "123"

	imageRepoName          = "testRepo"
	unmatchedImageRepoName = "unmatchedRepo"
)

func TestGenerateBuildFromConfig(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	output := mockOutput()

	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:   "test-build-config",
			Labels: map[string]string{"testlabel": "testvalue"},
		},
		Parameters: buildapi.BuildParameters{
			Source: source,
			Revision: &buildapi.SourceRevision{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitSourceRevision{
					Commit: "1234",
				},
			},
			Strategy: strategy,
			Output:   output,
		},
	}
	revision := &buildapi.SourceRevision{
		Type: buildapi.BuildSourceGit,
		Git: &buildapi.GitSourceRevision{
			Commit: "abcd",
		},
	}
	build := GenerateBuildFromConfig(bc, revision, nil)
	if !reflect.DeepEqual(source, build.Parameters.Source) {
		t.Errorf("Build source does not match BuildConfig source")
	}
	if !reflect.DeepEqual(strategy, build.Parameters.Strategy) {
		t.Errorf("Build strategy does not match BuildConfig strategy")
	}
	if !reflect.DeepEqual(output, build.Parameters.Output) {
		t.Errorf("Build output does not match BuildConfig output")
	}
	if !reflect.DeepEqual(revision, build.Parameters.Revision) {
		t.Errorf("Build revision does not match passed in revision")
	}
	if build.Labels["testlabel"] != bc.Labels["testlabel"] {
		t.Errorf("Build does not contain labels from BuildConfig")
	}
	if build.Labels[buildapi.BuildConfigLabel] != bc.Name {
		t.Errorf("Build does not contain labels from BuildConfig")
	}
}

func TestGenerateBuildWithImageTag(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	strategy.DockerStrategy.Image = originalImage
	output := mockOutput()
	imageRepoGetter := &mockImageRepositoryNamespaceGetter{"", imageRepoName}

	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-config",
		},
		Parameters: buildapi.BuildParameters{
			Source: source,
			Revision: &buildapi.SourceRevision{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitSourceRevision{
					Commit: "1234",
				},
			},
			Strategy: strategy,
			Output:   output,
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					Image: originalImage,
					From: kapi.ObjectReference{
						Name: imageRepoName,
					},
					Tag: tagName,
				},
			},
		},
	}

	build, err := GenerateBuildWithImageTag(bc, nil, imageRepoGetter)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.DockerStrategy.Image != newImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Parameters.Strategy.DockerStrategy.Image, newImage)
	}
}

func TestGenerateBuildWithImageTagUnmatchedRepo(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	strategy.DockerStrategy.Image = originalImage
	output := mockOutput()
	imageRepoGetter := &mockImageRepositoryNamespaceGetter{"", imageRepoName}

	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-config",
		},
		Parameters: buildapi.BuildParameters{
			Source: source,
			Revision: &buildapi.SourceRevision{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitSourceRevision{
					Commit: "1234",
				},
			},
			Strategy: strategy,
			Output:   output,
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					Image: originalImage,
					From: kapi.ObjectReference{
						Name: unmatchedImageRepoName,
					},
					Tag: tagName,
				},
			},
		},
	}

	build, err := GenerateBuildWithImageTag(bc, nil, imageRepoGetter)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.DockerStrategy.Image != originalImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Parameters.Strategy.DockerStrategy.Image, originalImage)
	}
}

func TestGenerateBuildWithImageTagNoTrigger(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	strategy.DockerStrategy.Image = originalImage
	output := mockOutput()
	imageRepoGetter := &mockImageRepositoryNamespaceGetter{"", imageRepoName}

	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-config",
		},
		Parameters: buildapi.BuildParameters{
			Source: source,
			Revision: &buildapi.SourceRevision{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitSourceRevision{
					Commit: "1234",
				},
			},
			Strategy: strategy,
			Output:   output,
		},
	}

	build, err := GenerateBuildWithImageTag(bc, nil, imageRepoGetter)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.DockerStrategy.Image != originalImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Parameters.Strategy.DockerStrategy.Image, originalImage)
	}
}

func TestGenerateBuildWithImageTagUnmatchedTag(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	strategy.DockerStrategy.Image = originalImage
	output := mockOutput()
	imageRepoGetter := &mockImageRepositoryNamespaceGetter{"", imageRepoName}

	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-config",
		},
		Parameters: buildapi.BuildParameters{
			Source: source,
			Revision: &buildapi.SourceRevision{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitSourceRevision{
					Commit: "1234",
				},
			},
			Strategy: strategy,
			Output:   output,
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					Image: originalImage,
					From: kapi.ObjectReference{
						Name: imageRepoName,
					},
					Tag: unmatchedTagName,
				},
			},
		},
	}

	build, err := GenerateBuildWithImageTag(bc, nil, imageRepoGetter)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.DockerStrategy.Image != originalImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Parameters.Strategy.DockerStrategy.Image, originalImage)
	}
}

func TestGenerateBuildFromBuild(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	output := mockOutput()

	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build",
		},
		Parameters: buildapi.BuildParameters{
			Source: source,
			Revision: &buildapi.SourceRevision{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitSourceRevision{
					Commit: "1234",
				},
			},
			Strategy: strategy,
			Output:   output,
		},
	}
	newBuild := GenerateBuildFromBuild(build)
	if !reflect.DeepEqual(build.Parameters, newBuild.Parameters) {
		t.Errorf("Build parameters does not match the original Build parameters")
	}
	if !reflect.DeepEqual(build.ObjectMeta.Labels, newBuild.ObjectMeta.Labels) {
		t.Errorf("Build labels does not match the original Build labels")
	}
}

func TestSubstituteImageDockerNil(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	build := GenerateBuildFromConfig(bc, nil, nil)

	// Docker build with nil base image
	// base image should still be nil
	SubstituteImageReferences(build, originalImage, newImage)
	if build.Parameters.Strategy.DockerStrategy.Image != "" {
		t.Errorf("Base image name was improperly substituted in docker strategy")
	}
}

func TestSubstituteImageDockerMatch(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	build := GenerateBuildFromConfig(bc, nil, nil)

	// Docker build with a matched base image
	// base image should be replaced.
	build.Parameters.Strategy.DockerStrategy.Image = originalImage
	SubstituteImageReferences(build, originalImage, newImage)
	if build.Parameters.Strategy.DockerStrategy.Image != newImage {
		t.Errorf("Base image name was not substituted in docker strategy")
	}
	if bc.Parameters.Strategy.DockerStrategy.Image != "" {
		t.Errorf("Docker BuildConfig was updated when Build was modified")
	}
}

func TestSubstituteImageDockerMismatch(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	build := GenerateBuildFromConfig(bc, nil, nil)

	// Docker build with an unmatched base image
	// base image should not be replaced.
	SubstituteImageReferences(build, "unmatched", "dummy")
	if build.Parameters.Strategy.DockerStrategy.Image == "dummy2" {
		t.Errorf("Base image name was improperly substituted in docker strategy")
	}
}

func TestSubstituteImageSTIMatch(t *testing.T) {
	source := mockSource()
	strategy := mockSTIStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	build := GenerateBuildFromConfig(bc, nil, nil)

	// STI build with a matched base image
	// base image should be replaced
	SubstituteImageReferences(build, originalImage, newImage)
	if build.Parameters.Strategy.STIStrategy.Image != newImage {
		t.Errorf("Base image name was not substituted in sti strategy")
	}
	if bc.Parameters.Strategy.STIStrategy.Image != originalImage {
		t.Errorf("STI BuildConfig was updated when Build was modified")
	}

}

func TestSubstituteImageSTIMismatch(t *testing.T) {
	source := mockSource()
	strategy := mockSTIStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	build := GenerateBuildFromConfig(bc, nil, nil)

	// STI build with an unmatched base image
	// base image should not be replaced
	SubstituteImageReferences(build, "unmatched", "dummy")
	if build.Parameters.Strategy.STIStrategy.Image == "dummy" {
		t.Errorf("Base image name was improperly substituted in STI strategy")
	}
}

func TestSubstituteImageCustomAllMatch(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	build := GenerateBuildFromConfig(bc, nil, nil)

	// Full custom build with a Image and a well defined environment variable image value,
	// both should be replaced.  Additional environment variables should not be touched.
	build.Parameters.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 2)
	build.Parameters.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	build.Parameters.Strategy.CustomStrategy.Env[1] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: originalImage}
	SubstituteImageReferences(build, originalImage, newImage)
	if build.Parameters.Strategy.CustomStrategy.Image != newImage {
		t.Errorf("Base image name was not substituted in custom strategy")
	}
	if build.Parameters.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable %s was improperly substituted in custom strategy", build.Parameters.Strategy.CustomStrategy.Env[0].Name)
	}
	if build.Parameters.Strategy.CustomStrategy.Env[1].Value != newImage {
		t.Errorf("Image env variable was not properly substituted in custom strategy")
	}
	if c := len(build.Parameters.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
	if bc.Parameters.Strategy.CustomStrategy.Image != originalImage {
		t.Errorf("Custom BuildConfig Image was updated when Build was modified")
	}
	if len(bc.Parameters.Strategy.CustomStrategy.Env) != 0 {
		t.Errorf("Custom BuildConfig Env was updated when Build was modified")
	}

}

func TestSubstituteImageCustomAllMismatch(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	build := GenerateBuildFromConfig(bc, nil, nil)

	// Full custom build with base image that is not matched
	// Base image name should be unchanged
	SubstituteImageReferences(build, "dummy", "dummy")
	if build.Parameters.Strategy.CustomStrategy.Image != originalImage {
		t.Errorf("Base image name was improperly substituted in custom strategy")
	}
}

func TestSubstituteImageCustomBaseMatchEnvMismatch(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	build := GenerateBuildFromConfig(bc, nil, nil)

	// Full custom build with a Image and a well defined environment variable image value that does not match the new image
	// Only base image should be replaced.  Environment variables should not be touched.
	build.Parameters.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 2)
	build.Parameters.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	build.Parameters.Strategy.CustomStrategy.Env[1] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: "dummy"}
	SubstituteImageReferences(build, originalImage, newImage)
	if build.Parameters.Strategy.CustomStrategy.Image != newImage {
		t.Errorf("Base image name was not substituted in custom strategy")
	}
	if build.Parameters.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable %s was improperly substituted in custom strategy", build.Parameters.Strategy.CustomStrategy.Env[0].Name)
	}
	if build.Parameters.Strategy.CustomStrategy.Env[1].Value != "dummy" {
		t.Errorf("Image env variable was improperly substituted in custom strategy")
	}
	if c := len(build.Parameters.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
}

func TestSubstituteImageCustomBaseMatchEnvMissing(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	build := GenerateBuildFromConfig(bc, nil, nil)

	// Custom build with a base Image but no image environment variable.
	// base image should be replaced, new image environment variable should be added,
	// existing environment variable should be untouched
	build.Parameters.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 1)
	build.Parameters.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	SubstituteImageReferences(build, originalImage, newImage)
	if build.Parameters.Strategy.CustomStrategy.Image != newImage {
		t.Errorf("Base image name was not substituted in custom strategy")
	}
	if build.Parameters.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable was improperly substituted in custom strategy")
	}
	if build.Parameters.Strategy.CustomStrategy.Env[1].Name != buildapi.CustomBuildStrategyBaseImageKey || build.Parameters.Strategy.CustomStrategy.Env[1].Value != newImage {
		t.Errorf("Image env variable was not added in custom strategy %s %s |", build.Parameters.Strategy.CustomStrategy.Env[1].Name, build.Parameters.Strategy.CustomStrategy.Env[1].Value)
	}
	if c := len(build.Parameters.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
}

func TestSubstituteImageCustomBaseMatchEnvNil(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	build := GenerateBuildFromConfig(bc, nil, nil)

	// Custom build with a base Image but no environment variables
	// base image should be replaced, new image environment variable should be added
	SubstituteImageReferences(build, originalImage, newImage)
	if build.Parameters.Strategy.CustomStrategy.Image != newImage {
		t.Errorf("Base image name was not substituted in custom strategy")
	}
	if build.Parameters.Strategy.CustomStrategy.Env[0].Name != buildapi.CustomBuildStrategyBaseImageKey || build.Parameters.Strategy.CustomStrategy.Env[0].Value != newImage {
		t.Errorf("New image name variable was not added to environment list in custom strategy")
	}
	if c := len(build.Parameters.Strategy.CustomStrategy.Env); c != 1 {
		t.Errorf("Expected %d, found %d environment variables", 1, c)
	}
}

func mockSource() buildapi.BuildSource {
	return buildapi.BuildSource{
		Type: buildapi.BuildSourceGit,
		Git: &buildapi.GitBuildSource{
			URI: "http://test.repository/namespace/name",
			Ref: "test-tag",
		},
	}
}

func mockDockerStrategy() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.DockerBuildStrategyType,
		DockerStrategy: &buildapi.DockerBuildStrategy{
			NoCache: true,
		},
	}
}

func mockSTIStrategy() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.STIBuildStrategyType,
		STIStrategy: &buildapi.STIBuildStrategy{
			Image: originalImage,
		},
	}
}

func mockCustomStrategy() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.CustomBuildStrategyType,
		CustomStrategy: &buildapi.CustomBuildStrategy{
			Image: originalImage,
		},
	}
}

func mockOutput() buildapi.BuildOutput {
	return buildapi.BuildOutput{
		DockerImageReference: "http://localhost:5000/test/image-tag",
	}
}

func mockBuildConfig(source buildapi.BuildSource, strategy buildapi.BuildStrategy, output buildapi.BuildOutput) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-config",
		},
		Parameters: buildapi.BuildParameters{
			Source: source,
			Revision: &buildapi.SourceRevision{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitSourceRevision{
					Commit: "1234",
				},
			},
			Strategy: strategy,
			Output:   output,
		},
	}
}

type mockImageRepositoryNamespaceGetter struct {
	namespace string
	name      string
}

func (m *mockImageRepositoryNamespaceGetter) GetByNamespace(namespace, name string) (*imageapi.ImageRepository, error) {
	if m.namespace != namespace || m.name != name {
		return nil, nil
	}
	return &imageapi.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Name: imageRepoName},
		Status: imageapi.ImageRepositoryStatus{
			DockerImageRepository: originalImage,
			Tags: map[string]imageapi.TagEventList{
				tagName: {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: fmt.Sprintf("%s:%s", originalImage, newTag),
							Image:                newTag,
						},
					},
				},
			},
		},
		Tags: map[string]string{tagName: newTag},
	}, nil
}
