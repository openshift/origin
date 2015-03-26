package controller

import (
	"fmt"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildtest "github.com/openshift/origin/pkg/build/controller/test"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type mockBuildConfigUpdater struct {
	buildcfg *buildapi.BuildConfig
	err      error
}

func (m *mockBuildConfigUpdater) Update(buildcfg *buildapi.BuildConfig) error {
	m.buildcfg = buildcfg
	return m.err
}

func mockBuildConfig(baseImage, triggerImage, repoName, repoTag string) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "testBuildCfg",
		},
		Parameters: buildapi.BuildParameters{
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					Image: baseImage,
				},
			},
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					Image: triggerImage,
					From: kapi.ObjectReference{
						Name: repoName,
					},
					Tag: repoTag,
				},
			},
		},
	}
}

func appendTrigger(buildcfg *buildapi.BuildConfig, triggerImage, repoName, repoTag string) {
	buildcfg.Triggers = append(buildcfg.Triggers, buildapi.BuildTriggerPolicy{
		Type: buildapi.ImageChangeBuildTriggerType,
		ImageChange: &buildapi.ImageChangeTrigger{
			Image: triggerImage,
			From: kapi.ObjectReference{
				Name: repoName,
			},
			Tag: repoTag,
		},
	})
}

func mockImageStream(repoName, dockerImageRepo string, tags map[string]string) *imageapi.ImageStream {
	tagHistory := make(map[string]imageapi.TagEventList)
	for tag, imageID := range tags {
		tagHistory[tag] = imageapi.TagEventList{
			Items: []imageapi.TagEvent{
				{
					Image:                imageID,
					DockerImageReference: fmt.Sprintf("%s:%s", dockerImageRepo, imageID),
				},
			},
		}
	}

	return &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: repoName,
		},
		Status: imageapi.ImageStreamStatus{
			DockerImageRepository: dockerImageRepo,
			Tags: tagHistory,
		},
	}
}

type buildConfigInstantiator struct {
	generator buildgenerator.BuildGenerator
	name      string
	newBuild  *buildapi.Build
	err       error
}

func (i *buildConfigInstantiator) Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	i.name = request.Name
	return i.generator.Instantiate(kapi.WithNamespace(kapi.NewContext(), namespace), request)
}

func mockBuildConfigInstantiator(buildcfg *buildapi.BuildConfig, imageStream *imageapi.ImageStream) *buildConfigInstantiator {
	instantiator := &buildConfigInstantiator{}
	generator := buildgenerator.BuildGenerator{
		Client: buildgenerator.Client{
			GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
				return buildcfg, nil
			},
			UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
				return nil
			},
			CreateBuildFunc: func(ctx kapi.Context, build *buildapi.Build) error {
				instantiator.newBuild = build
				return instantiator.err
			},
			GetBuildFunc: func(ctx kapi.Context, name string) (*buildapi.Build, error) {
				return instantiator.newBuild, nil
			},
			GetImageStreamFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
				return imageStream, nil
			},
		}}
	instantiator.generator = generator
	return instantiator
}

func mockImageChangeController(buildcfg *buildapi.BuildConfig, imageStream *imageapi.ImageStream) *ImageChangeController {
	return &ImageChangeController{
		BuildConfigStore:        buildtest.NewFakeBuildConfigStore(buildcfg),
		BuildConfigInstantiator: mockBuildConfigInstantiator(buildcfg, imageStream),
		BuildConfigUpdater:      &mockBuildConfigUpdater{},
	}
}

func TestNewImageID(t *testing.T) {
	// valid configuration, new build should be triggered.
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "testTag")
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}

	if len(bcInstantiator.name) == 0 {
		t.Error("Expected build generation when new image was created!")
	}
	if actual, expected := bcInstantiator.newBuild.Parameters.Strategy.DockerStrategy.Image, "registry.com/namespace/imagename:newImageID123"; actual != expected {
		t.Errorf("Image substitutions not properly setup for new build. Expected %s, got %s |", expected, actual)
	}
	if bcUpdater.buildcfg == nil {
		t.Fatalf("Expected buildConfig update when new image was created!")
	}
	if actual, expected := bcUpdater.buildcfg.Triggers[0].ImageChange.LastTriggeredImageID, "registry.com/namespace/imagename:newImageID123"; actual != expected {
		t.Errorf("Expected last triggered image %q, got %q", expected, actual)
	}
}

func TestNewImageIDDefaultTag(t *testing.T) {
	// valid configuration using default tag, new build should be triggered.
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "")
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{buildapi.DefaultImageTag: "newImageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if len(bcInstantiator.name) == 0 {
		t.Error("Expected build generation when new image was created!")
	}
	if actual, expected := bcInstantiator.newBuild.Parameters.Strategy.DockerStrategy.Image, "registry.com/namespace/imagename:newImageID123"; actual != expected {
		t.Errorf("Image substitutions not properly setup for new build. Expected %s, got %s |", expected, actual)
	}
	if bcUpdater.buildcfg == nil {
		t.Fatal("Expected buildConfig update when new image was created!")
	}
	if actual, expected := bcUpdater.buildcfg.Triggers[0].ImageChange.LastTriggeredImageID, "registry.com/namespace/imagename:newImageID123"; actual != expected {
		t.Errorf("Expected last triggered image %q, got %q", expected, actual)
	}
}

func TestNonExistentImageStream(t *testing.T) {
	// this buildconfig references a non-existent image stream, so an update to the real image stream should not
	// trigger a build here.
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "testTag")
	imageStream := mockImageStream("otherImageRepo", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if len(bcInstantiator.name) != 0 {
		t.Error("New build generated when a different repository was updated!")
	}
	if bcUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestNewImageDifferentTagUpdate(t *testing.T) {
	// this buildconfig references a different tag than the one that will be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "testTag")
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"otherTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if len(bcInstantiator.name) != 0 {
		t.Error("New build generated when a different repository was updated!")
	}
	if bcUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestNewImageDifferentTagUpdate2(t *testing.T) {
	// this buildconfig references a different tag than the one that will be updated
	// it has previously run a build for the testTagID123 tag.
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "testTag")
	buildcfg.Triggers[0].ImageChange.LastTriggeredImageID = "registry.com/namespace/imagename:testTagID123"
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"otherTag": "newImageID123", "testTag": "testTagID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if len(bcInstantiator.name) != 0 {
		t.Error("New build generated when a different repository was updated!")
	}
	if bcUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestNewDifferentImageUpdate(t *testing.T) {
	// this buildconfig references a different image than the one that will be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename1", "registry.com/namespace/imagename1", "testImageRepo1", "testTag1")
	imageStream := mockImageStream("testImageRepo2", "registry.com/namespace/imagename2", map[string]string{"testTag2": "newImageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if len(bcInstantiator.name) != 0 {
		t.Error("New build generated when a different repository was updated!")
	}
	if bcUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestMultipleTriggers(t *testing.T) {
	// this buildconfig references multiple images
	buildcfg := mockBuildConfig("registry.com/namespace/imagename1", "registry.com/namespace/imagename1", "testImageRepo1", "testTag1")
	appendTrigger(buildcfg, "registry.com/namespace/imagename2", "testImageRepo2", "testTag2")
	imageStream := mockImageStream("testImageRepo2", "registry.com/namespace/imagename2", map[string]string{"testTag2": "newImageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if len(bcInstantiator.name) == 0 {
		t.Error("Expected build generation when new image was created!")
	}
	if bcUpdater.buildcfg == nil {
		t.Fatal("Expected buildConfig update when new image was created!")
	}
	if actual, expected := bcUpdater.buildcfg.Triggers[1].ImageChange.LastTriggeredImageID, "registry.com/namespace/imagename2:newImageID123"; actual != expected {
		t.Errorf("Expected last triggered image %q, got %q", expected, actual)
	}
}

func TestBuildConfigWithDifferentTriggerType(t *testing.T) {
	// this buildconfig has different (than ImageChangeTrigger) trigger defined
	buildcfg := mockBuildConfig("registry.com/namespace/imagename1", "", "", "")
	buildcfg.Triggers[0].Type = buildapi.GenericWebHookBuildTriggerType
	imageStream := mockImageStream("testImageRepo2", "registry.com/namespace/imagename2", map[string]string{"testTag2": "newImageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if len(bcInstantiator.name) != 0 {
		t.Error("New build generated when a different repository was updated!")
	}
	if bcUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different trigger was defined!")
	}
}

func TestNoImageIDChange(t *testing.T) {
	// this buildConfig has up to date configuration, but is checked eg. during
	// startup when we're checking all the imageRepos
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "testTag")
	buildcfg.Triggers[0].ImageChange.LastTriggeredImageID = "registry.com/namespace/imagename:imageID123"
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"testTag": "imageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if len(bcInstantiator.name) != 0 {
		t.Error("New build generated when no change happened!")
	}
	if bcUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when no change happened!")
	}
}

func TestBuildConfigInstantiatorError(t *testing.T) {
	// valid configuration, but build creation fails, in that situation the buildconfig should not be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "testTag")
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcInstantiator.err = fmt.Errorf("instantiating error")
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err == nil || !strings.Contains(err.Error(), "instantiating error") {
		t.Error("Expected error from HandleImageRepo")
	}
	if actual, expected := bcInstantiator.newBuild.Parameters.Strategy.DockerStrategy.Image, "registry.com/namespace/imagename:newImageID123"; actual != expected {
		t.Errorf("Image substitutions not properly setup for new build. Expected %s, got %s |", expected, actual)
	}
	if bcUpdater.buildcfg != nil {
		t.Fatal("Expected no buildConfig update on BuildCreate error!")
	}
}

func TestBuildConfigUpdateError(t *testing.T) {
	// valid configuration, but build creation fails, in that situation the buildconfig should not be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "testTag")
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)
	bcUpdater.err = fmt.Errorf("error")

	err := controller.HandleImageRepo(imageStream)
	if len(bcInstantiator.name) == 0 {
		t.Error("Expected build generation when new image was created!")
	}
	if _, ok := err.(ImageChangeControllerFatalError); !ok {
		t.Error("Expected fatal error from HandleImageRepo")
	}
}

func TestNewImageIDNoDockerRepo(t *testing.T) {
	// No docker repository associated with the imageStream, so no build can be created
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "testTag")
	imageStream := mockImageStream("testImageStream", "", map[string]string{"testTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg, imageStream)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if len(bcInstantiator.name) != 0 {
		t.Error("New build generated when no change happened!")
	}
	if bcUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when no change happened!")
	}
}
