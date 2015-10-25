package generator

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"

	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	mocks "github.com/openshift/origin/pkg/build/generator/test"
	buildutil "github.com/openshift/origin/pkg/build/util"
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

func TestInstantiate(t *testing.T) {
	generator := mockBuildGenerator()
	_, err := generator.Instantiate(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

// TODO(agoldste): I'm not sure the intent of this test. Using the previous logic for
// the generator, which would try to update the build config before creating
// the build, I can see why the UpdateBuildConfigFunc is set up to return an
// error, but nothing is checking the value of instantiationCalls. We could
// update this test to fail sooner, when the build is created, but that's
// already handled by TestCreateBuildCreateError. We may just want to delete
// this test.
/*
func TestInstantiateRetry(t *testing.T) {
	instantiationCalls := 0
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: Client{
			GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput()), nil
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
*/

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
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
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

func TestInstantiateWithImageTrigger(t *testing.T) {
	imageID := "the-image-id-12345"
	defaultTriggers := func() []buildapi.BuildTriggerPolicy {
		return []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.GenericWebHookBuildTriggerType,
			},
			{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					From: &kapi.ObjectReference{
						Name: "image1:tag1",
						Kind: "ImageStreamTag",
					},
				},
			},
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					From: &kapi.ObjectReference{
						Name:      "image2:tag2",
						Namespace: "image2ns",
						Kind:      "ImageStreamTag",
					},
				},
			},
		}
	}
	triggersWithImageID := func() []buildapi.BuildTriggerPolicy {
		triggers := defaultTriggers()
		triggers[2].ImageChange.LastTriggeredImageID = imageID
		return triggers
	}
	tests := []struct {
		name          string
		reqFrom       *kapi.ObjectReference
		triggerIndex  int // index of trigger that will be updated with the image id, if -1, no update expected
		triggers      []buildapi.BuildTriggerPolicy
		errorExpected bool
	}{
		{
			name: "default trigger",
			reqFrom: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "image3:tag3",
			},
			triggerIndex: 1,
			triggers:     defaultTriggers(),
		},
		{
			name: "trigger with from",
			reqFrom: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "image1:tag1",
			},
			triggerIndex: 2,
			triggers:     defaultTriggers(),
		},
		{
			name: "trigger with from and namespace",
			reqFrom: &kapi.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      "image2:tag2",
				Namespace: "image2ns",
			},
			triggerIndex: 3,
			triggers:     defaultTriggers(),
		},
		{
			name: "existing image id",
			reqFrom: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "image1:tag1",
			},
			triggers:      triggersWithImageID(),
			errorExpected: true,
		},
	}

	for _, tc := range tests {
		bc := &buildapi.BuildConfig{
			Spec: buildapi.BuildConfigSpec{
				BuildSpec: buildapi.BuildSpec{
					Strategy: buildapi.BuildStrategy{
						Type: buildapi.SourceBuildStrategyType,
						SourceStrategy: &buildapi.SourceBuildStrategy{
							From: kapi.ObjectReference{
								Name: "image3:tag3",
								Kind: "ImageStreamTag",
							},
						},
					},
				},
				Triggers: tc.triggers,
			},
		}
		generator := mockBuildGeneratorForInstantiate()
		client := generator.Client.(Client)
		client.GetBuildConfigFunc =
			func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
				return bc, nil
			}
		client.UpdateBuildConfigFunc =
			func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
				bc = buildConfig
				return nil
			}
		generator.Client = client

		req := &buildapi.BuildRequest{
			TriggeredByImage: &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: imageID,
			},
			From: tc.reqFrom,
		}
		_, err := generator.Instantiate(kapi.NewDefaultContext(), req)
		if err != nil && !tc.errorExpected {
			t.Errorf("%s: unexpected error %v", tc.name, err)
			continue
		}
		if err == nil && tc.errorExpected {
			t.Errorf("%s: expected error but didn't get one", tc.name)
			continue
		}
		if tc.errorExpected {
			continue
		}
		for i := range bc.Spec.Triggers {
			if i == tc.triggerIndex {
				// Verify that the trigger got updated
				if bc.Spec.Triggers[i].ImageChange.LastTriggeredImageID != imageID {
					t.Errorf("%s: expeccted trigger at index %d to contain imageID %s", tc.name, i, imageID)
				}
				continue
			}
			// Ensure that other triggers are updated with the latest docker image ref
			if bc.Spec.Triggers[i].Type == buildapi.ImageChangeBuildTriggerType {
				from := bc.Spec.Triggers[i].ImageChange.From
				if from == nil {
					from = buildutil.GetImageStreamForStrategy(bc.Spec.Strategy)
				}
				if bc.Spec.Triggers[i].ImageChange.LastTriggeredImageID != ("ref@" + from.Name) {
					t.Errorf("%s: expected LastTriggeredImageID for trigger at %d to be %s. Got: %s", tc.name, i, "ref@"+from.Name, bc.Spec.Triggers[i].ImageChange.LastTriggeredImageID)
				}
			}
		}
	}
}

func TestInstantiateWithLastVersion(t *testing.T) {
	g := mockBuildGenerator()
	c := g.Client.(Client)
	c.GetBuildConfigFunc = func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
		bc := mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput())
		bc.Status.LastVersion = 1
		return bc, nil
	}
	g.Client = c

	// Version not specified
	_, err := g.Instantiate(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	// Version specified and it matches
	lastVersion := 1
	_, err = g.Instantiate(kapi.NewDefaultContext(), &buildapi.BuildRequest{LastVersion: &lastVersion})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	// Version specified, but doesn't match
	lastVersion = 0
	_, err = g.Instantiate(kapi.NewDefaultContext(), &buildapi.BuildRequest{LastVersion: &lastVersion})
	if err == nil {
		t.Errorf("Expected an error and did not get one")
	}
}

func TestFindImageTrigger(t *testing.T) {
	defaultTrigger := &buildapi.ImageChangeTrigger{}
	image1Trigger := &buildapi.ImageChangeTrigger{
		From: &kapi.ObjectReference{
			Name: "image1:tag1",
		},
	}
	image2Trigger := &buildapi.ImageChangeTrigger{
		From: &kapi.ObjectReference{
			Name:      "image2:tag2",
			Namespace: "image2ns",
		},
	}
	image4Trigger := &buildapi.ImageChangeTrigger{
		From: &kapi.ObjectReference{
			Name: "image4:tag4",
		},
	}
	image5Trigger := &buildapi.ImageChangeTrigger{
		From: &kapi.ObjectReference{
			Name:      "image5:tag5",
			Namespace: "bcnamespace",
		},
	}
	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "testbc",
			Namespace: "bcnamespace",
		},
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.SourceBuildStrategyType,
					SourceStrategy: &buildapi.SourceBuildStrategy{
						From: kapi.ObjectReference{
							Name: "image3:tag3",
							Kind: "ImageStreamTag",
						},
					},
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
				},
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: defaultTrigger,
				},
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: image1Trigger,
				},
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: image2Trigger,
				},
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: image4Trigger,
				},
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: image5Trigger,
				},
			},
		},
	}

	tests := []struct {
		name   string
		input  *kapi.ObjectReference
		expect *buildapi.ImageChangeTrigger
	}{
		{
			name:   "nil reference",
			input:  nil,
			expect: nil,
		},
		{
			name: "match name",
			input: &kapi.ObjectReference{
				Name: "image1:tag1",
			},
			expect: image1Trigger,
		},
		{
			name: "mismatched namespace",
			input: &kapi.ObjectReference{
				Name:      "image1:tag1",
				Namespace: "otherns",
			},
			expect: nil,
		},
		{
			name: "match name and namespace",
			input: &kapi.ObjectReference{
				Name:      "image2:tag2",
				Namespace: "image2ns",
			},
			expect: image2Trigger,
		},
		{
			name: "match default trigger",
			input: &kapi.ObjectReference{
				Name: "image3:tag3",
			},
			expect: defaultTrigger,
		},
		{
			name: "input includes bc namespace",
			input: &kapi.ObjectReference{
				Name:      "image4:tag4",
				Namespace: "bcnamespace",
			},
			expect: image4Trigger,
		},
		{
			name: "implied namespace in trigger input",
			input: &kapi.ObjectReference{
				Name: "image5:tag5",
			},
			expect: image5Trigger,
		},
	}

	for _, tc := range tests {
		result := findImageChangeTrigger(bc, tc.input)
		if result != tc.expect {
			t.Errorf("%s: unexpected trigger for %#v: %#v", tc.name, tc.input, result)
		}
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
	source := mocks.MockSource()
	strategy := mockDockerStrategyForDockerImage(originalImage)
	output := mocks.MockOutput()
	resources := mockResources()
	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test-build-config",
			Namespace: "test-namespace",
			Labels:    map[string]string{"testlabel": "testvalue"},
		},
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
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
		},
		Status: buildapi.BuildConfigStatus{
			LastVersion: 12,
		},
	}
	revision := &buildapi.SourceRevision{
		Type: buildapi.BuildSourceGit,
		Git: &buildapi.GitSourceRevision{
			Commit: "abcd",
		},
	}
	generator := mockBuildGenerator()

	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, revision, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if !reflect.DeepEqual(source, build.Spec.Source) {
		t.Errorf("Build source does not match BuildConfig source")
	}
	// FIXME: This is disabled because the strategies does not match since we plug the
	//        pullSecret into the build strategy.
	/*
		if !reflect.DeepEqual(strategy, build.Spec.Strategy) {
			t.Errorf("Build strategy does not match BuildConfig strategy %+v != %+v", strategy.DockerStrategy, build.Spec.Strategy.DockerStrategy)
		}
	*/
	if !reflect.DeepEqual(output, build.Spec.Output) {
		t.Errorf("Build output does not match BuildConfig output")
	}
	if !reflect.DeepEqual(revision, build.Spec.Revision) {
		t.Errorf("Build revision does not match passed in revision")
	}
	if !reflect.DeepEqual(resources, build.Spec.Resources) {
		t.Errorf("Build resources does not match passed in resources")
	}
	if build.Labels["testlabel"] != bc.Labels["testlabel"] {
		t.Errorf("Build does not contain labels from BuildConfig")
	}
	if build.Labels[buildapi.BuildConfigLabel] != bc.Name {
		t.Errorf("Build does not contain labels from BuildConfig")
	}
	if build.Status.Config.Name != bc.Name || build.Status.Config.Namespace != bc.Namespace || build.Status.Config.Kind != "BuildConfig" {
		t.Errorf("Build does not contain correct BuildConfig reference: %v", build.Status.Config)
	}
	if build.Annotations[buildapi.BuildNumberAnnotation] != "13" {
		t.Errorf("Build number annotation value %s does not match expected value 13", build.Annotations[buildapi.BuildNumberAnnotation])
	}
}

func TestGenerateBuildWithImageTagForSourceStrategyImageRepository(t *testing.T) {
	source := mocks.MockSource()
	strategy := mocks.MockSourceStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-config",
		},
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
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
		},
	}
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
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

	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Spec.Strategy.SourceStrategy.From.Name != newImage {
		t.Errorf("source-to-image base image value %s does not match expected value %s", build.Spec.Strategy.SourceStrategy.From.Name, newImage)
	}
}

func TestGenerateBuildWithImageTagForDockerStrategyImageRepository(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockDockerStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-config",
		},
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
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
		},
	}
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
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

	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Spec.Strategy.DockerStrategy.From.Name != newImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Spec.Strategy.DockerStrategy.From.Name, newImage)
	}
}

func TestGenerateBuildWithImageTagForCustomStrategyImageRepository(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-config",
		},
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
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
		},
	}
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
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

	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Spec.Strategy.CustomStrategy.From.Name != newImage {
		t.Errorf("Custom base image value %s does not match expected value %s", build.Spec.Strategy.CustomStrategy.From.Name, newImage)
	}
}

func TestGenerateBuildFromBuild(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockDockerStrategyForImageRepository()
	output := mocks.MockOutput()
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build",
		},
		Spec: buildapi.BuildSpec{
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

	newBuild := generateBuildFromBuild(build, nil)
	if !reflect.DeepEqual(build.Spec, newBuild.Spec) {
		t.Errorf("Build parameters does not match the original Build parameters")
	}
	if !reflect.DeepEqual(build.ObjectMeta.Labels, newBuild.ObjectMeta.Labels) {
		t.Errorf("Build labels does not match the original Build labels")
	}
}

func TestGenerateBuildFromBuildWithBuildConfig(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockDockerStrategyForImageRepository()
	output := mocks.MockOutput()
	annotatedBuild := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "annotatedBuild",
			Annotations: map[string]string{
				buildapi.BuildCloneAnnotation: "sourceOfBuild",
			},
		},
		Spec: buildapi.BuildSpec{
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
	nonAnnotatedBuild := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "nonAnnotatedBuild",
		},
		Spec: buildapi.BuildSpec{
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

	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "buildConfigName",
		},
		Status: buildapi.BuildConfigStatus{
			LastVersion: 5,
		},
	}

	newBuild := generateBuildFromBuild(annotatedBuild, buildConfig)
	if !reflect.DeepEqual(annotatedBuild.Spec, newBuild.Spec) {
		t.Errorf("Build parameters does not match the original Build parameters")
	}
	if !reflect.DeepEqual(annotatedBuild.ObjectMeta.Labels, newBuild.ObjectMeta.Labels) {
		t.Errorf("Build labels does not match the original Build labels")
	}
	if newBuild.Annotations[buildapi.BuildNumberAnnotation] != "6" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildapi.BuildNumberAnnotation], "6")
	}
	if newBuild.Annotations[buildapi.BuildCloneAnnotation] != "annotatedBuild" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildapi.BuildCloneAnnotation], "annotatedBuild")
	}

	newBuild = generateBuildFromBuild(nonAnnotatedBuild, buildConfig)
	if !reflect.DeepEqual(nonAnnotatedBuild.Spec, newBuild.Spec) {
		t.Errorf("Build parameters does not match the original Build parameters")
	}
	if !reflect.DeepEqual(nonAnnotatedBuild.ObjectMeta.Labels, newBuild.ObjectMeta.Labels) {
		t.Errorf("Build labels does not match the original Build labels")
	}
	// was incremented by previous test, so expect 7 now.
	if newBuild.Annotations[buildapi.BuildNumberAnnotation] != "7" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildapi.BuildNumberAnnotation], "7")
	}
	if newBuild.Annotations[buildapi.BuildCloneAnnotation] != "nonAnnotatedBuild" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildapi.BuildCloneAnnotation], "nonAnnotatedBuild")
	}

}

func TestSubstituteImageCustomAllMatch(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForDockerImage(originalImage)
	output := mocks.MockOutput()
	bc := mocks.MockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with a Image and a well defined environment variable image value,
	// both should be replaced.  Additional environment variables should not be touched.
	build.Spec.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 2)
	build.Spec.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	build.Spec.Strategy.CustomStrategy.Env[1] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: originalImage}
	updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
	if build.Spec.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable %s was improperly substituted in custom strategy", build.Spec.Strategy.CustomStrategy.Env[0].Name)
	}
	if build.Spec.Strategy.CustomStrategy.Env[1].Value != newImage {
		t.Errorf("Image env variable was not properly substituted in custom strategy")
	}
	if c := len(build.Spec.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
	if bc.Spec.Strategy.CustomStrategy.From.Name != originalImage {
		t.Errorf("Custom BuildConfig Image was updated when Build was modified %s!=%s", bc.Spec.Strategy.CustomStrategy.From.Name, originalImage)
	}
	if len(bc.Spec.Strategy.CustomStrategy.Env) != 0 {
		t.Errorf("Custom BuildConfig Env was updated when Build was modified")
	}
}

func TestSubstituteImageCustomAllMismatch(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForDockerImage(originalImage)
	output := mocks.MockOutput()
	bc := mocks.MockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with base image that is not matched
	// Base image name should be unchanged
	updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, "dummy")
	if build.Spec.Strategy.CustomStrategy.From.Name != originalImage {
		t.Errorf("Base image name was improperly substituted in custom strategy %s %s", build.Spec.Strategy.CustomStrategy.From.Name, originalImage)
	}
}

func TestSubstituteImageCustomBaseMatchEnvMismatch(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := mocks.MockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with a Image and a well defined environment variable image value that does not match the new image
	// Environment variables should not be updated.
	build.Spec.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 2)
	build.Spec.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someEnvVar", Value: originalImage}
	build.Spec.Strategy.CustomStrategy.Env[1] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: "dummy"}
	updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
	if build.Spec.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable %s was improperly substituted in custom strategy", build.Spec.Strategy.CustomStrategy.Env[0].Name)
	}
	if build.Spec.Strategy.CustomStrategy.Env[1].Value != newImage {
		t.Errorf("Image env variable was not substituted in custom strategy")
	}
	if c := len(build.Spec.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
}

func TestSubstituteImageCustomBaseMatchEnvMissing(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := mocks.MockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Custom build with a base Image but no image environment variable.
	// base image should be replaced, new image environment variable should be added,
	// existing environment variable should be untouched
	build.Spec.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 1)
	build.Spec.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
	if build.Spec.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable was improperly substituted in custom strategy")
	}
	if build.Spec.Strategy.CustomStrategy.Env[1].Name != buildapi.CustomBuildStrategyBaseImageKey || build.Spec.Strategy.CustomStrategy.Env[1].Value != newImage {
		t.Errorf("Image env variable was not added in custom strategy %s %s |", build.Spec.Strategy.CustomStrategy.Env[1].Name, build.Spec.Strategy.CustomStrategy.Env[1].Value)
	}
	if c := len(build.Spec.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
}

func TestSubstituteImageCustomBaseMatchEnvNil(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := mocks.MockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator()
	build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Custom build with a base Image but no environment variables
	// base image should be replaced, new image environment variable should be added
	updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
	if build.Spec.Strategy.CustomStrategy.Env[0].Name != buildapi.CustomBuildStrategyBaseImageKey || build.Spec.Strategy.CustomStrategy.Env[0].Value != newImage {
		t.Errorf("New image name variable was not added to environment list in custom strategy")
	}
	if c := len(build.Spec.Strategy.CustomStrategy.Env); c != 1 {
		t.Errorf("Expected %d, found %d environment variables", 1, c)
	}
}

func TestGetNextBuildName(t *testing.T) {
	bc := mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput())
	if expected, actual := bc.Name+"-1", getNextBuildName(bc); expected != actual {
		t.Errorf("Wrong buildName, expected %s, got %s", expected, actual)
	}
	if expected, actual := 1, bc.Status.LastVersion; expected != actual {
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
		// 4
		{"mybuild-10-1426794070", `^mybuild-10-\d+$`},
	}

	for i, tc := range testCases {
		buildName := getNextBuildNameFromBuild(&buildapi.Build{ObjectMeta: kapi.ObjectMeta{Name: tc.value}}, nil)
		if matched, err := regexp.MatchString(tc.expected, buildName); !matched || err != nil {
			t.Errorf("(%d) Unexpected build name, got %s expected %s", i, buildName, tc.expected)
		}
	}
}

func TestGetNextBuildNameFromBuildWithBuildConfig(t *testing.T) {
	testCases := []struct {
		value       string
		buildConfig *buildapi.BuildConfig
		expected    string
	}{
		// 0
		{
			"mybuild-1",
			&buildapi.BuildConfig{
				ObjectMeta: kapi.ObjectMeta{
					Name: "buildConfigName",
				},
				Status: buildapi.BuildConfigStatus{
					LastVersion: 5,
				},
			},
			`^buildConfigName-6$`,
		},
		// 1
		{
			"mybuild-1-1426794070",
			&buildapi.BuildConfig{
				ObjectMeta: kapi.ObjectMeta{
					Name: "buildConfigName",
				},
				Status: buildapi.BuildConfigStatus{
					LastVersion: 5,
				},
			},
			`^buildConfigName-6$`,
		},
	}

	for i, tc := range testCases {
		buildName := getNextBuildNameFromBuild(&buildapi.Build{ObjectMeta: kapi.ObjectMeta{Name: tc.value}}, tc.buildConfig)
		if matched, err := regexp.MatchString(tc.expected, buildName); !matched || err != nil {
			t.Errorf("(%d) Unexpected build name, got %s expected %s", i, buildName, tc.expected)
		}
	}
}

func TestResolveImageStreamRef(t *testing.T) {
	type resolveTest struct {
		streamRef         kapi.ObjectReference
		tag               string
		expectedSuccess   bool
		expectedDockerRef string
	}
	generator := mockBuildGenerator()

	tests := []resolveTest{
		{
			streamRef: kapi.ObjectReference{
				Name: imageRepoName,
			},
			tag:               tagName,
			expectedSuccess:   false,
			expectedDockerRef: dockerReference,
		},
		{
			streamRef: kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: imageRepoName + ":" + tagName,
			},
			expectedSuccess:   true,
			expectedDockerRef: latestDockerReference,
		},
		{
			streamRef: kapi.ObjectReference{
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
			From: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: originalImage,
			},
		},
	}
}

func mockCustomStrategyForImageRepository() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.CustomBuildStrategyType,
		CustomStrategy: &buildapi.CustomBuildStrategy{
			From: kapi.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      imageRepoName + ":" + tagName,
				Namespace: imageRepoNamespace,
			},
		},
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

func mockBuild(source buildapi.BuildSource, strategy buildapi.BuildStrategy, output buildapi.BuildOutput) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build",
		},
		Spec: buildapi.BuildSpec{
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

func mockBuildGeneratorForInstantiate() *BuildGenerator {
	g := mockBuildGenerator()
	c := g.Client.(Client)
	c.GetImageStreamTagFunc = func(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error) {
		return &imageapi.ImageStreamTag{
			Image: imageapi.Image{
				ObjectMeta:           kapi.ObjectMeta{Name: imageRepoName + ":" + newTag},
				DockerImageReference: "ref@" + name,
			},
		}, nil
	}
	g.Client = c
	return g
}

func mockBuildGenerator() *BuildGenerator {
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	return &BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: Client{
			GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput()), nil
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
	source := mocks.MockSource()
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
		"secret1/image":               {".dockercfg": mocks.SampleDockerConfigs["hub"]},
		"1.1.1.1:5000/secret3/image":  {".dockercfg": mocks.SampleDockerConfigs["ipv4"]},
		"registry.host/secret4/image": {".dockercfg": mocks.SampleDockerConfigs["host"]},
	}
	for imageName := range dockerCfgTable {
		// Setup the BuildGenerator
		strategy := mockDockerStrategyForDockerImage(imageName)
		output := mockOutputWithImageName(imageName)
		generator := mockBuildGenerator()
		bc := mocks.MockBuildConfig(source, strategy, output)
		build, err := generator.generateBuildFromConfig(kapi.NewContext(), bc, revision, nil)

		if build.Spec.Output.PushSecret == nil {
			t.Errorf("Expected PushSecret for image '%s' to be set, got nil", imageName)
			continue
		}
		if build.Spec.Strategy.DockerStrategy.PullSecret == nil {
			t.Errorf("Expected PullSecret for image '%s' to be set, got nil", imageName)
			continue
		}
		if len(build.Spec.Output.PushSecret.Name) == 0 {
			t.Errorf("Expected PushSecret for image %s to be set not empty", imageName)
		}
		if len(build.Spec.Strategy.DockerStrategy.PullSecret.Name) == 0 {
			t.Errorf("Expected PullSecret for image %s to be set not empty", imageName)
		}
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}
	}
}
