package generator

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type FakeDockerCfg map[string]map[string]string

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

	dockerReference       = "dockerReference"
	latestDockerReference = "latestDockerReference"
)

var (
	encode = func(src string) []byte {
		return []byte(src)
	}
	sampleDockerConfigs = map[string][]byte{
		"hub":  encode(`{"https://index.docker.io/v1/":{"auth": "Zm9vOmJhcgo=", "email": ""}}`),
		"ipv4": encode(`{"https://1.1.1.1:5000/v1/":{"auth": "Zm9vOmJhcgo=", "email": ""}}`),
		"host": encode(`{"https://registry.host/v1/":{"auth": "Zm9vOmJhcgo=", "email": ""}}`),
	}
)

func TestInstantiate(t *testing.T) {
	generator := mockBuildGenerator()
	_, err := generator.Instantiate(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestInstantiateRetry(t *testing.T) {
	instantiationCalls := 0
	fakeSecrets := []runtime.Object{}
	for _, s := range mockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mockBuilderServiceAccount(mockBuilderSecrets()),
		Client: Client{
			GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
				return mockBuildConfig(mockSource(), mockSourceStrategyForImageRepository(), mockOutput()), nil
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
	generator := BuildGenerator{Client: Client{
		GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
			return nil, fmt.Errorf("get-error")
		},
		GetImageStreamFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
			return nil, fmt.Errorf("get-error")
		},
		GetImageStreamImageFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error) {
			return nil, fmt.Errorf("get-error")
		},
		GetImageStreamTagFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error) {
			return nil, fmt.Errorf("get-error")
		},
	}}

	_, err := generator.Instantiate(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "get-error") {
		t.Errorf("Expected get-error, got different %v", err)
	}
}

func TestInstantiateGenerateBuildError(t *testing.T) {
	fakeSecrets := []runtime.Object{}
	for _, s := range mockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mockBuilderServiceAccount(mockBuilderSecrets()),
		Client: Client{
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
	strategy := mockDockerStrategyForDockerImage(originalImage)
	output := mockOutput()
	resources := mockResources()
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
			Strategy:  strategy,
			Output:    output,
			Resources: resources,
		},
	}
	revision := &buildapi.SourceRevision{
		Type: buildapi.BuildSourceGit,
		Git: &buildapi.GitSourceRevision{
			Commit: "abcd",
		},
	}
	generator := mockBuildGenerator()

	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, revision)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if !reflect.DeepEqual(source, build.Parameters.Source) {
		t.Errorf("Build source does not match BuildConfig source")
	}
	// FIXME: This is disabled because the strategies does not match since we plug the
	//        pullSecret into the build strategy.
	/*
		if !reflect.DeepEqual(strategy, build.Parameters.Strategy) {
			t.Errorf("Build strategy does not match BuildConfig strategy %+v != %+v", strategy.DockerStrategy, build.Parameters.Strategy.DockerStrategy)
		}
	*/
	if !reflect.DeepEqual(output, build.Parameters.Output) {
		t.Errorf("Build output does not match BuildConfig output")
	}
	if !reflect.DeepEqual(revision, build.Parameters.Revision) {
		t.Errorf("Build revision does not match passed in revision")
	}
	if !reflect.DeepEqual(resources, build.Parameters.Resources) {
		t.Errorf("Build resources does not match passed in resources")
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

func TestGenerateBuildWithImageTagForSourceStrategyImageRepository(t *testing.T) {
	source := mockSource()
	strategy := mockSourceStrategyForImageRepository()
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
	fakeSecrets := []runtime.Object{}
	for _, s := range mockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mockBuilderServiceAccount(mockBuilderSecrets()),
		Client: Client{
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
			GetImageStreamTagFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error) {
				return &imageapi.ImageStreamTag{
					ImageName: name,
					Image: imageapi.Image{
						ObjectMeta:           kapi.ObjectMeta{Name: imageRepoName + ":" + newTag},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			GetImageStreamImageFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error) {
				return &imageapi.ImageStreamImage{
					Image: imageapi.Image{
						ObjectMeta:           kapi.ObjectMeta{Name: imageRepoName + ":@id"},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},

			UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
				return nil
			},
		}}

	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.SourceStrategy.From.Name != newImage {
		t.Errorf("source-to-image base image value %s does not match expected value %s", build.Parameters.Strategy.SourceStrategy.From.Name, newImage)
	}
}

func TestGenerateBuildWithImageTagForDockerStrategyImageRepository(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategyForImageRepository()
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
	fakeSecrets := []runtime.Object{}
	for _, s := range mockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mockBuilderServiceAccount(mockBuilderSecrets()),
		Client: Client{
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
			GetImageStreamTagFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error) {
				return &imageapi.ImageStreamTag{
					ImageName: name,
					Image: imageapi.Image{
						ObjectMeta:           kapi.ObjectMeta{Name: imageRepoName + ":" + newTag},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			GetImageStreamImageFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error) {
				return &imageapi.ImageStreamImage{
					Image: imageapi.Image{
						ObjectMeta:           kapi.ObjectMeta{Name: imageRepoName + ":@id"},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
				return nil
			},
		}}

	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.DockerStrategy.From.Name != newImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Parameters.Strategy.DockerStrategy.From.Name, newImage)
	}
}

func TestGenerateBuildWithImageTagForCustomStrategyImageRepository(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategyForImageRepository()
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
	fakeSecrets := []runtime.Object{}
	for _, s := range mockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mockBuilderServiceAccount(mockBuilderSecrets()),
		Client: Client{
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
			GetImageStreamTagFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error) {
				return &imageapi.ImageStreamTag{
					ImageName: name,
					Image: imageapi.Image{
						ObjectMeta:           kapi.ObjectMeta{Name: imageRepoName + ":" + newTag},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			GetImageStreamImageFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error) {
				return &imageapi.ImageStreamImage{
					Image: imageapi.Image{
						ObjectMeta:           kapi.ObjectMeta{Name: imageRepoName + ":@id"},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
				return nil
			},
		}}

	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Parameters.Strategy.CustomStrategy.From.Name != newImage {
		t.Errorf("Custom base image value %s does not match expected value %s", build.Parameters.Strategy.CustomStrategy.From.Name, newImage)
	}
}

func TestGenerateBuildFromBuild(t *testing.T) {
	source := mockSource()
	strategy := mockDockerStrategyForImageRepository()
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

func TestSubstituteImageCustomAllMatch(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategyForDockerImage(originalImage)
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with a Image and a well defined environment variable image value,
	// both should be replaced.  Additional environment variables should not be touched.
	build.Parameters.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 2)
	build.Parameters.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	build.Parameters.Strategy.CustomStrategy.Env[1] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: originalImage}
	updateCustomImageEnv(build.Parameters.Strategy.CustomStrategy, newImage)
	if build.Parameters.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable %s was improperly substituted in custom strategy", build.Parameters.Strategy.CustomStrategy.Env[0].Name)
	}
	if build.Parameters.Strategy.CustomStrategy.Env[1].Value != newImage {
		t.Errorf("Image env variable was not properly substituted in custom strategy")
	}
	if c := len(build.Parameters.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
	if bc.Parameters.Strategy.CustomStrategy.From.Name != originalImage {
		t.Errorf("Custom BuildConfig Image was updated when Build was modified %s!=%s", bc.Parameters.Strategy.CustomStrategy.From.Name, originalImage)
	}
	if len(bc.Parameters.Strategy.CustomStrategy.Env) != 0 {
		t.Errorf("Custom BuildConfig Env was updated when Build was modified")
	}
}

func TestSubstituteImageCustomAllMismatch(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategyForDockerImage(originalImage)
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with base image that is not matched
	// Base image name should be unchanged
	updateCustomImageEnv(build.Parameters.Strategy.CustomStrategy, "dummy")
	if build.Parameters.Strategy.CustomStrategy.From.Name != originalImage {
		t.Errorf("Base image name was improperly substituted in custom strategy %s %s", build.Parameters.Strategy.CustomStrategy.From.Name, originalImage)
	}
}

func TestSubstituteImageCustomBaseMatchEnvMismatch(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategyForImageRepository()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with a Image and a well defined environment variable image value that does not match the new image
	// Environment variables should not be updated.
	build.Parameters.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 2)
	build.Parameters.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someEnvVar", Value: originalImage}
	build.Parameters.Strategy.CustomStrategy.Env[1] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: "dummy"}
	updateCustomImageEnv(build.Parameters.Strategy.CustomStrategy, newImage)
	if build.Parameters.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable %s was improperly substituted in custom strategy", build.Parameters.Strategy.CustomStrategy.Env[0].Name)
	}
	if build.Parameters.Strategy.CustomStrategy.Env[1].Value != newImage {
		t.Errorf("Image env variable was not substituted in custom strategy")
	}
	if c := len(build.Parameters.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
}

func TestSubstituteImageCustomBaseMatchEnvMissing(t *testing.T) {
	source := mockSource()
	strategy := mockCustomStrategyForImageRepository()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Custom build with a base Image but no image environment variable.
	// base image should be replaced, new image environment variable should be added,
	// existing environment variable should be untouched
	build.Parameters.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 1)
	build.Parameters.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	updateCustomImageEnv(build.Parameters.Strategy.CustomStrategy, newImage)
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
	strategy := mockCustomStrategyForImageRepository()
	output := mockOutput()
	bc := mockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Custom build with a base Image but no environment variables
	// base image should be replaced, new image environment variable should be added
	updateCustomImageEnv(build.Parameters.Strategy.CustomStrategy, newImage)
	if build.Parameters.Strategy.CustomStrategy.Env[0].Name != buildapi.CustomBuildStrategyBaseImageKey || build.Parameters.Strategy.CustomStrategy.Env[0].Value != newImage {
		t.Errorf("New image name variable was not added to environment list in custom strategy")
	}
	if c := len(build.Parameters.Strategy.CustomStrategy.Env); c != 1 {
		t.Errorf("Expected %d, found %d environment variables", 1, c)
	}
}

func TestGetNextBuildName(t *testing.T) {
	bc := mockBuildConfig(mockSource(), mockSourceStrategyForImageRepository(), mockOutput())
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

func TestResolveImageStreamRef(t *testing.T) {
	type resolveTest struct {
		streamRef         *kapi.ObjectReference
		tag               string
		expectedSuccess   bool
		expectedDockerRef string
	}
	generator := mockBuildGenerator()

	tests := []resolveTest{
		{
			streamRef: &kapi.ObjectReference{
				Name: imageRepoName,
			},
			tag:               tagName,
			expectedSuccess:   false,
			expectedDockerRef: dockerReference,
		},
		{
			streamRef: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: imageRepoName + ":" + tagName,
			},
			expectedSuccess:   true,
			expectedDockerRef: latestDockerReference,
		},
		{
			streamRef: &kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: imageRepoName + "@myid",
			},
			expectedSuccess: true,
			// until we default to the "real" pull by id logic,
			// the @id is applied as a :tag when resolving the repository.
			expectedDockerRef: latestDockerReference,
		},
	}
	for i, test := range tests {
		ref, error := generator.resolveImageStreamReference(kapi.NewDefaultContext(), test.streamRef, "")
		if error != nil {
			if test.expectedSuccess {
				t.Errorf("Scenario %d: Unexpected error %v", i, error)
			}
			continue
		} else if !test.expectedSuccess {
			t.Errorf("Scenario %d: did not get expected error", i)
		}
		if ref != test.expectedDockerRef {
			t.Errorf("Scenario %d: Resolved reference %s did not match expected value %s", i, ref, test.expectedDockerRef)
		}
	}
}

func mockResources() kapi.ResourceRequirements {
	res := kapi.ResourceRequirements{}
	res.Limits = kapi.ResourceList{}
	res.Limits[kapi.ResourceCPU] = resource.MustParse("100m")
	res.Limits[kapi.ResourceMemory] = resource.MustParse("100Mi")
	return res
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

func mockDockerStrategyForNilImage() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.DockerBuildStrategyType,
		DockerStrategy: &buildapi.DockerBuildStrategy{
			NoCache: true,
		},
	}
}

func mockDockerStrategyForDockerImage(name string) buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.DockerBuildStrategyType,
		DockerStrategy: &buildapi.DockerBuildStrategy{
			NoCache: true,
			From: &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: name,
			},
		},
	}
}

func mockDockerStrategyForImageRepository() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.DockerBuildStrategyType,
		DockerStrategy: &buildapi.DockerBuildStrategy{
			NoCache: true,
			From: &kapi.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      imageRepoName + ":" + tagName,
				Namespace: imageRepoNamespace,
			},
		},
	}
}

func mockCustomStrategyForDockerImage(name string) buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.CustomBuildStrategyType,
		CustomStrategy: &buildapi.CustomBuildStrategy{
			From: &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: originalImage,
			},
		},
	}
}

func mockSourceStrategyForImageRepository() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.SourceBuildStrategyType,
		SourceStrategy: &buildapi.SourceBuildStrategy{
			From: &kapi.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      imageRepoName + ":" + tagName,
				Namespace: imageRepoNamespace,
			},
		},
	}
}

func mockCustomStrategyForImageRepository() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.CustomBuildStrategyType,
		CustomStrategy: &buildapi.CustomBuildStrategy{
			From: &kapi.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      imageRepoName + ":" + tagName,
				Namespace: imageRepoNamespace,
			},
		},
	}
}

func mockOutput() buildapi.BuildOutput {
	return buildapi.BuildOutput{
		DockerImageReference: "http://localhost:5000/test/image-tag",
	}
}

func mockOutputWithImageName(name string) buildapi.BuildOutput {
	return buildapi.BuildOutput{
		To: &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: name,
		},
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
	fakeSecrets := []runtime.Object{}
	for _, s := range mockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	return &BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mockBuilderServiceAccount(mockBuilderSecrets()),
		Client: Client{
			GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
				return mockBuildConfig(mockSource(), mockSourceStrategyForImageRepository(), mockOutput()), nil
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
				if name != imageRepoName {
					return &imageapi.ImageStream{}, nil
				}
				return &imageapi.ImageStream{
					ObjectMeta: kapi.ObjectMeta{
						Name:      imageRepoName,
						Namespace: imageRepoNamespace,
					},
					Status: imageapi.ImageStreamStatus{
						DockerImageRepository: "repo/namespace/image",
						Tags: map[string]imageapi.TagEventList{
							tagName: {
								Items: []imageapi.TagEvent{
									{DockerImageReference: dockerReference},
								},
							},
							imageapi.DefaultImageTag: {
								Items: []imageapi.TagEvent{
									{DockerImageReference: latestDockerReference},
								},
							},
						},
					},
				}, nil
			},
			GetImageStreamTagFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error) {
				return &imageapi.ImageStreamTag{
					ImageName: name,
					Image: imageapi.Image{
						ObjectMeta:           kapi.ObjectMeta{Name: imageRepoName + ":" + newTag},
						DockerImageReference: latestDockerReference,
					},
				}, nil
			},
			GetImageStreamImageFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error) {
				return &imageapi.ImageStreamImage{
					Image: imageapi.Image{
						ObjectMeta:           kapi.ObjectMeta{Name: imageRepoName + ":@id"},
						DockerImageReference: latestDockerReference,
					},
				}, nil
			},
		}}
}

func TestGenerateBuildFromConfigWithSecrets(t *testing.T) {
	source := mockSource()
	revision := &buildapi.SourceRevision{
		Type: buildapi.BuildSourceGit,
		Git: &buildapi.GitSourceRevision{
			Commit: "abcd",
		},
	}
	dockerCfgTable := map[string]map[string][]byte{
		// FIXME: This image pull spec does not return ANY registry, but it should
		// return the hub.
		//"docker.io/secret2/image":     {".dockercfg": sampleDockerConfigs["hub"]},
		"secret1/image":               {".dockercfg": sampleDockerConfigs["hub"]},
		"1.1.1.1:5000/secret3/image":  {".dockercfg": sampleDockerConfigs["ipv4"]},
		"registry.host/secret4/image": {".dockercfg": sampleDockerConfigs["host"]},
	}
	for imageName := range dockerCfgTable {
		// Setup the BuildGenerator
		strategy := mockDockerStrategyForDockerImage(imageName)
		output := mockOutputWithImageName(imageName)
		generator := mockBuildGenerator()
		bc := mockBuildConfig(source, strategy, output)
		build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, revision)

		if build.Parameters.Output.PushSecret == nil {
			t.Errorf("Expected PushSecret for image '%s' to be set, got nil", imageName)
			continue
		}
		if build.Parameters.Strategy.DockerStrategy.PullSecret == nil {
			t.Errorf("Expected PullSecret for image '%s' to be set, got nil", imageName)
			continue
		}
		if len(build.Parameters.Output.PushSecret.Name) == 0 {
			t.Errorf("Expected PushSecret for image %s to be set not empty", imageName)
		}
		if len(build.Parameters.Strategy.DockerStrategy.PullSecret.Name) == 0 {
			t.Errorf("Expected PullSecret for image %s to be set not empty", imageName)
		}
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}
	}
}

func mockBuilderSecrets() (secrets []*kapi.Secret) {
	i := 1
	for name, conf := range sampleDockerConfigs {
		secrets = append(secrets, &kapi.Secret{
			ObjectMeta: kapi.ObjectMeta{
				Name: name,
			},
			Type: kapi.SecretTypeDockercfg,
			Data: map[string][]byte{".dockercfg": conf},
		})
		i++
	}
	return secrets
}

func mockBuilderServiceAccount(secrets []*kapi.Secret) kclient.ServiceAccountsNamespacer {
	var (
		secretRefs  []kapi.ObjectReference
		fakeObjects []runtime.Object
	)
	for _, secret := range secrets {
		secretRefs = append(secretRefs, kapi.ObjectReference{Name: secret.Name, Kind: "Secret"})
		fakeObjects = append(fakeObjects, secret)
	}
	fakeObjects = append(fakeObjects, &kapi.ServiceAccount{
		ObjectMeta: kapi.ObjectMeta{Name: bootstrappolicy.BuilderServiceAccountName},
		Secrets:    secretRefs,
	})
	return testclient.NewSimpleFake(fakeObjects...)
}
