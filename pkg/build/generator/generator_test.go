package generator

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
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
	imageRepoNamespace     = "testns"
)

func TestInstantiate(t *testing.T) {
	generator := BuildGenerator{Client: Client{
		GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
			return mockBuildConfig(mockSource(), mockSTIStrategyForImage(), mockOutput()), nil
		},
		UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
			return nil
		},
		CreateBuildFunc: func(ctx kapi.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx kapi.Context, name string) (*buildapi.Build, error) {
			return &buildapi.Build{}, nil
		},
	}}

	_, err := generator.Instantiate(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestInstantiateRetry(t *testing.T) {
	instantiationCalls := 0
	generator := BuildGenerator{Client: Client{
		GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
			return mockBuildConfig(mockSource(), mockSTIStrategyForImage(), mockOutput()), nil
		},
		UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
			instantiationCalls++
			return fmt.Errorf("update-error")
		},
	}}

	_, err := generator.Instantiate(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "update-error") {
		t.Errorf("Expected update-error, got different %v", err)
	}

}

func TestInstantiateGetBuildConfigError(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	strategy.DockerStrategy.Image = originalImage
	output := mockOutput()
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
	generator := BuildGenerator{Client: Client{
		GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
			return bc, nil
		},
		GetImageStreamFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
			return nil, fmt.Errorf("get-error")
		},
	}}

	_, err := generator.Instantiate(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "get-error") {
		t.Errorf("Expected get-error, got different %v", err)
	}
}

func TestInstantiateGenerateBuildError(t *testing.T) {
	generator := BuildGenerator{Client: Client{
		GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
			return nil, fmt.Errorf("get-error")
		},
	}}

	_, err := generator.Instantiate(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "get-error") {
		t.Errorf("Expected get-error, got different %v", err)
	}
}

func TestClone(t *testing.T) {
	generator := BuildGenerator{Client: Client{
		CreateBuildFunc: func(ctx kapi.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx kapi.Context, name string) (*buildapi.Build, error) {
			return &buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "test-build-1",
					Namespace: kapi.NamespaceDefault,
				},
			}, nil
		},
	}}

	_, err := generator.Clone(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCloneError(t *testing.T) {
	generator := BuildGenerator{Client: Client{
		GetBuildFunc: func(ctx kapi.Context, name string) (*buildapi.Build, error) {
			return nil, fmt.Errorf("get-error")
		},
	}}

	_, err := generator.Clone(kapi.NewContext(), &buildapi.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "get-error") {
		t.Errorf("Expected get-error, got different %v", err)
	}
}

func TestCreateBuild(t *testing.T) {
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test-build",
			Namespace: kapi.NamespaceDefault,
		},
	}
	generator := BuildGenerator{Client: Client{
		CreateBuildFunc: func(ctx kapi.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx kapi.Context, name string) (*buildapi.Build, error) {
			return build, nil
		},
	}}

	build, err := generator.createBuild(kapi.NewDefaultContext(), build)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.CreationTimestamp.IsZero() || len(build.UID) == 0 {
		t.Error("Expected meta fields being filled in!")
	}
}

func TestCreateBuildNamespaceError(t *testing.T) {
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build",
		},
	}
	generator := mockBuildGenerator()

	_, err := generator.createBuild(kapi.NewContext(), build)
	if err == nil || !strings.Contains(err.Error(), "Build.Namespace") {
		t.Errorf("Expected namespace error, got different %v", err)
	}
}

func TestCreateBuildCreateError(t *testing.T) {
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test-build",
			Namespace: kapi.NamespaceDefault,
		},
	}
	generator := BuildGenerator{Client: Client{
		CreateBuildFunc: func(ctx kapi.Context, build *buildapi.Build) error {
			return fmt.Errorf("create-error")
		},
	}}

	_, err := generator.createBuild(kapi.NewDefaultContext(), build)
	if err == nil || !strings.Contains(err.Error(), "create-error") {
		t.Errorf("Expected create-error, got different %v", err)
	}
}

func TestGenerateBuildFromConfig(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	output := mockOutput()
	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test-build-config",
			Namespace: "test-namespace",
			Labels:    map[string]string{"testlabel": "testvalue"},
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
	generator := mockBuildGenerator()

	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, revision, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
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
	if build.Config.Name != bc.Name || build.Config.Namespace != bc.Namespace || build.Config.Kind != "BuildConfig" {
		t.Errorf("Build does not contain correct BuildConfig reference: %v", build.Config)
	}
}

func TestGenerateBuildWithImageTagForDockerStrategy(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	strategy.DockerStrategy.Image = originalImage
	output := mockOutput()
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
	generator := BuildGenerator{Client: Client{
		GetImageStreamFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
			return &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: imageRepoName},
				Status: imageapi.ImageStreamStatus{
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
			}, nil
		},
		UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
			return nil
		},
	}}

	build, err := generator.generateBuild(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.DockerStrategy.Image != newImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Parameters.Strategy.DockerStrategy.Image, newImage)
	}
}

func TestGenerateBuildFromConfigWithImageTagUnmatchedRepo(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	strategy.DockerStrategy.Image = originalImage
	output := mockOutput()
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
	generator := mockBuildGenerator()

	build, err := generator.generateBuild(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.DockerStrategy.Image != originalImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Parameters.Strategy.DockerStrategy.Image, originalImage)
	}
}

func TestGenerateBuildFromConfigWithImageTagNoTrigger(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	strategy.DockerStrategy.Image = originalImage
	output := mockOutput()
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
	generator := mockBuildGenerator()

	build, err := generator.generateBuild(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.DockerStrategy.Image != originalImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Parameters.Strategy.DockerStrategy.Image, originalImage)
	}
}

func TestGenerateBuildFromConfigWithImageTagUnmatchedTag(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	strategy.DockerStrategy.Image = originalImage
	output := mockOutput()
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
	generator := mockBuildGenerator()

	build, err := generator.generateBuild(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.DockerStrategy.Image != originalImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Parameters.Strategy.DockerStrategy.Image, originalImage)
	}
}

func TestGenerateBuildWithImageTagForSTIStrategyImage(t *testing.T) {
	source := mockSource()
	strategy := mockSTIStrategyForImage()
	output := mockOutput()

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
	generator := BuildGenerator{Client: Client{
		GetImageStreamFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
			return &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: imageRepoName},
				Status: imageapi.ImageStreamStatus{
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
			}, nil
		},
		UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
			return nil
		},
	}}

	build, err := generator.generateBuild(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.STIStrategy.Image != newImage {
		t.Errorf("STI base image value %s does not match expected value %s", build.Parameters.Strategy.STIStrategy.Image, newImage)
	}
}

func TestGenerateBuildWithImageTagForSTIStrategyImageRepository(t *testing.T) {
	source := mockSource()
	strategy := mockSTIStrategyForImageRepository()
	output := mockOutput()
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
	generator := BuildGenerator{Client: Client{
		GetImageStreamFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
			return &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: imageRepoName},
				Status: imageapi.ImageStreamStatus{
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
			}, nil
		},
		UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
			return nil
		},
	}}

	build, err := generator.generateBuild(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatal("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.STIStrategy.Image != newImage {
		t.Errorf("STI base image value %s does not match expected value %s", build.Parameters.Strategy.STIStrategy.Image, newImage)
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

	newBuild := generateBuildFromBuild(build)
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
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Docker build with nil base image
	// base image should still be nil
	substituteImageReferences(build, originalImage, newImage)
	if build.Parameters.Strategy.DockerStrategy.Image != "" {
		t.Errorf("Base image name was improperly substituted in docker strategy")
	}
}

func TestSubstituteImageDockerMatch(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Docker build with a matched base image
	// base image should be replaced.
	build.Parameters.Strategy.DockerStrategy.Image = originalImage
	substituteImageReferences(build, originalImage, newImage)
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
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Docker build with an unmatched base image
	// base image should not be replaced.
	substituteImageReferences(build, "unmatched", "dummy")
	if build.Parameters.Strategy.DockerStrategy.Image == "dummy2" {
		t.Errorf("Base image name was improperly substituted in docker strategy")
	}
}

func TestSubstituteImageSTIMatch(t *testing.T) {
	source := mockSource()
	strategy := mockSTIStrategyForImage()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// STI build with a matched base image
	// base image should be replaced
	substituteImageReferences(build, originalImage, newImage)
	if build.Parameters.Strategy.STIStrategy.Image != newImage {
		t.Errorf("Base image name was not substituted in sti strategy")
	}
	if bc.Parameters.Strategy.STIStrategy.Image != originalImage {
		t.Errorf("STI BuildConfig was updated when Build was modified")
	}
}

func TestSubstituteImageRepositorySTIMatch(t *testing.T) {
	source := mockSource()
	strategy := mockSTIStrategyForImageRepository()
	repoRef := *strategy.STIStrategy.From
	output := mockOutput()

	// this test just uses a build rather than generating a build from a config
	// because generating a build from a config will try to resolve the From reference
	// in the buildconfig and without significantly more infrastructure setup, that
	// resolution will fail.
	build := mockBuild(source, strategy, output)
	// STI build with a matched base image
	// base image should be replaced
	substituteImageRepoReferences(build, repoRef, newImage)
	if build.Parameters.Strategy.STIStrategy.Image != newImage {
		t.Errorf("Base image name was not substituted in sti strategy")
	}
}

func TestSubstituteImageSTIMismatch(t *testing.T) {
	source := mockSource()
	strategy := mockSTIStrategyForImage()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// STI build with an unmatched base image
	// base image should not be replaced
	substituteImageReferences(build, "unmatched", "dummy")
	if build.Parameters.Strategy.STIStrategy.Image == "dummy" {
		t.Errorf("Base image name was improperly substituted in STI strategy")
	}
}

func TestSubstituteImageRepositorySTIMismatch(t *testing.T) {
	source := mockSource()
	strategy := mockSTIStrategyForImage()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	imageRepoRef := kapi.ObjectReference{
		Name: "unmatched",
	}
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// STI build with an unmatched image repository
	// base image should not be set
	substituteImageRepoReferences(build, imageRepoRef, "dummy")
	if build.Parameters.Strategy.STIStrategy.Image == "dummy" {
		t.Errorf("Base image name was improperly set in STI strategy")
	}
}

func TestSubstituteImageCustomAllMatch(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with a Image and a well defined environment variable image value,
	// both should be replaced.  Additional environment variables should not be touched.
	build.Parameters.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 2)
	build.Parameters.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	build.Parameters.Strategy.CustomStrategy.Env[1] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: originalImage}
	substituteImageReferences(build, originalImage, newImage)
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
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with base image that is not matched
	// Base image name should be unchanged
	substituteImageReferences(build, "dummy", "dummy")
	if build.Parameters.Strategy.CustomStrategy.Image != originalImage {
		t.Errorf("Base image name was improperly substituted in custom strategy")
	}
}

func TestSubstituteImageCustomBaseMatchEnvMismatch(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategy()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with a Image and a well defined environment variable image value that does not match the new image
	// Only base image should be replaced.  Environment variables should not be touched.
	build.Parameters.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 2)
	build.Parameters.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	build.Parameters.Strategy.CustomStrategy.Env[1] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: "dummy"}
	substituteImageReferences(build, originalImage, newImage)
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
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Custom build with a base Image but no image environment variable.
	// base image should be replaced, new image environment variable should be added,
	// existing environment variable should be untouched
	build.Parameters.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 1)
	build.Parameters.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	substituteImageReferences(build, originalImage, newImage)
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
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Custom build with a base Image but no environment variables
	// base image should be replaced, new image environment variable should be added
	substituteImageReferences(build, originalImage, newImage)
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

func TestGetNextBuildName(t *testing.T) {
	bc := mockBuildConfig(mockSource(), mockSTIStrategyForImage(), mockOutput())
	if expected, actual := bc.Name+"-1", getNextBuildName(bc); expected != actual {
		t.Errorf("Wrong buildName, expected %s, got %s", expected, actual)
	}
	if expected, actual := 1, bc.LastVersion; expected != actual {
		t.Errorf("Wrong version, expected %d, got %d", expected, actual)
	}
}

func TestGetNextBuildNameFromBuild(t *testing.T) {
	testCases := []struct {
		value    string
		expected string
	}{
		// 0
		{"mybuild-1", `^mybuild-1-\d+$`},
		// 1
		{"mybuild-1-1426794070", `^mybuild-1-\d+$`},
		// 2
		{"mybuild-1-1426794070-1-1426794070", `^mybuild-1-1426794070-1-\d+$`},
		// 3
		{"my-build-1", `^my-build-1-\d+$`},
	}

	for i, tc := range testCases {
		buildName := getNextBuildNameFromBuild(&buildapi.Build{ObjectMeta: kapi.ObjectMeta{Name: tc.value}})
		if matched, err := regexp.MatchString(tc.expected, buildName); !matched || err != nil {
			t.Errorf("(%d) Unexpected build name, got %s", i, buildName)
		}
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

func mockSTIStrategyForImage() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.STIBuildStrategyType,
		STIStrategy: &buildapi.STIBuildStrategy{
			Image: originalImage,
		},
	}
}

func mockSTIStrategyForImageRepository() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.STIBuildStrategyType,
		STIStrategy: &buildapi.STIBuildStrategy{
			From: &kapi.ObjectReference{
				Name:      imageRepoName,
				Namespace: imageRepoNamespace,
			},
			Tag: tagName,
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

func mockBuild(source buildapi.BuildSource, strategy buildapi.BuildStrategy, output buildapi.BuildOutput) *buildapi.Build {
	return &buildapi.Build{
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
}

func mockBuildGenerator() *BuildGenerator {
	return &BuildGenerator{Client: Client{
		GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
			return mockBuildConfig(mockSource(), mockSTIStrategyForImage(), mockOutput()), nil
		},
		UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
			return nil
		},
		CreateBuildFunc: func(ctx kapi.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx kapi.Context, name string) (*buildapi.Build, error) {
			return &buildapi.Build{}, nil
		},
		GetImageStreamFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
			return &imageapi.ImageStream{}, nil
		},
	}}
}
