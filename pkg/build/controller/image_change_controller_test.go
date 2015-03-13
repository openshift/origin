package controller

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildtest "github.com/openshift/origin/pkg/build/controller/test"
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

type mockBuildCreator struct {
	build *buildapi.Build
	err   error
}

func (m *mockBuildCreator) Create(namespace string, build *buildapi.Build) error {
	m.build = build
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

func mockImageRepo(repoName, dockerImageRepo string, tags map[string]string) *imageapi.ImageRepository {
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

	return &imageapi.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{
			Name: repoName,
		},
		Status: imageapi.ImageRepositoryStatus{
			DockerImageRepository: dockerImageRepo,
			Tags: tagHistory,
		},
		Tags: tags,
	}
}

func mockImageChangeController(buildcfg *buildapi.BuildConfig) *ImageChangeController {
	return &ImageChangeController{
		BuildConfigStore:   buildtest.NewFakeBuildConfigStore(buildcfg),
		BuildCreator:       &mockBuildCreator{},
		BuildConfigUpdater: &mockBuildConfigUpdater{},
	}
}

func TestNewImageID(t *testing.T) {
	// valid configuration, new build should be triggered.
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	imagerepo := mockImageRepo("testImageRepo", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg)
	err := controller.HandleImageRepo(imagerepo)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if buildCreator.build == nil {
		t.Fatal("Expected new build when new image was created!")
	}
	if buildCreator.build.Parameters.Strategy.DockerStrategy.Image != "registry.com/namespace/imagename:newImageID123" {
		t.Errorf("Image substitutions not properly setup for new build.  Expected %s, got %s |", "registry.com/namespace/imagename:newImageID123", buildCreator.build.Parameters.Strategy.DockerStrategy.Image)
	}
	if buildConfigUpdater.buildcfg == nil {
		t.Fatal("Expected buildConfig update when new image was created!")
	}
	if buildConfigUpdater.buildcfg.Triggers[0].ImageChange.LastTriggeredImageID != "newImageID123" {
		t.Errorf("Expected imageID newImageID123, got %s", buildConfigUpdater.buildcfg.Triggers[0].ImageChange.LastTriggeredImageID)
	}
}

func TestNewImageIDDefaultTag(t *testing.T) {
	// valid configuration using default tag, new build should be triggered.
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "")
	imagerepo := mockImageRepo("testImageRepo", "registry.com/namespace/imagename", map[string]string{buildapi.DefaultImageTag: "newImageID123"})
	controller := mockImageChangeController(buildcfg)
	err := controller.HandleImageRepo(imagerepo)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if buildCreator.build == nil {
		t.Fatal("Expected new build when new image was created!")
	}
	if buildCreator.build.Parameters.Strategy.DockerStrategy.Image != "registry.com/namespace/imagename:newImageID123" {
		t.Errorf("Image substitutions not properly setup for new build.  Expected %s, got %s |", "registry.com/namespace/imagename:newImageID123", buildCreator.build.Parameters.Strategy.DockerStrategy.Image)
	}
	if buildConfigUpdater.buildcfg == nil {
		t.Fatal("Expected buildConfig update when new image was created!")
	}
	if buildConfigUpdater.buildcfg.Triggers[0].ImageChange.LastTriggeredImageID != "newImageID123" {
		t.Errorf("Expected imageID newImageID123, got %s", buildConfigUpdater.buildcfg.Triggers[0].ImageChange.LastTriggeredImageID)
	}
}

func TestNonExistentImageRepository(t *testing.T) {
	// this buildconfig references a non-existent imagerepo, so an update to the real imagerepo should not
	// trigger a build here.
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	imagerepo := mockImageRepo("otherImageRepo", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg)
	err := controller.HandleImageRepo(imagerepo)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if buildCreator.build != nil {
		t.Error("New build created when a different repository was updated!")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestNewImageDifferentTagUpdate(t *testing.T) {
	// this buildconfig references a different tag than the one that will be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	imagerepo := mockImageRepo("testImageRepo", "registry.com/namespace/imagename", map[string]string{"otherTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg)
	err := controller.HandleImageRepo(imagerepo)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if buildCreator.build != nil {
		t.Error("New build created when a different repository was updated!")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestNewImageDifferentTagUpdate2(t *testing.T) {
	// this buildconfig references a different tag than the one that will be updated
	// it has previously run a build for the testTagID123 tag.
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	buildcfg.Triggers[0].ImageChange.LastTriggeredImageID = "testTagID123"
	imagerepo := mockImageRepo("testImageRepo", "registry.com/namespace/imagename", map[string]string{"otherTag": "newImageID123", "testTag": "testTagID123"})
	controller := mockImageChangeController(buildcfg)
	err := controller.HandleImageRepo(imagerepo)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if buildCreator.build != nil {
		t.Error("New build created when a different repository was updated!")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestNewDifferentImageUpdate(t *testing.T) {
	// this buildconfig references a different image than the one that will be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename1", "registry.com/namespace/imagename1", "testImageRepo1", "testTag1")
	imagerepo := mockImageRepo("testImageRepo2", "registry.com/namespace/imagename2", map[string]string{"testTag2": "newImageID123"})
	controller := mockImageChangeController(buildcfg)
	err := controller.HandleImageRepo(imagerepo)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if buildCreator.build != nil {
		t.Error("New build created when a different repository was updated!")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestMultipleTriggers(t *testing.T) {
	// this buildconfig references multiple images
	buildcfg := mockBuildConfig("registry.com/namespace/imagename1", "registry.com/namespace/imagename1", "testImageRepo1", "testTag1")
	appendTrigger(buildcfg, "registry.com/namespace/imagename2", "testImageRepo2", "testTag2")
	imagerepo := mockImageRepo("testImageRepo2", "registry.com/namespace/imagename2", map[string]string{"testTag2": "newImageID123"})
	controller := mockImageChangeController(buildcfg)
	err := controller.HandleImageRepo(imagerepo)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if buildCreator.build == nil {
		t.Fatal("Expected new build when new image was created!")
	}
	if buildCreator.build.Parameters.Strategy.DockerStrategy.Image != "registry.com/namespace/imagename1" {
		t.Errorf("Image substitutions not properly setup for new build.  Expected %s, got %s |", "registry.com/namespace/imagename1", buildCreator.build.Parameters.Strategy.DockerStrategy.Image)
	}

	if buildConfigUpdater.buildcfg == nil {
		t.Fatal("Expected buildConfig update when new image was created!")
	}
	if buildConfigUpdater.buildcfg.Triggers[1].ImageChange.LastTriggeredImageID != "newImageID123" {
		t.Errorf("Expected imageID newImageID123, got %s", buildConfigUpdater.buildcfg.Triggers[1].ImageChange.LastTriggeredImageID)
	}
}

func TestBuildConfigWithDifferentTriggerType(t *testing.T) {
	// this buildconfig has different (than ImageChangeTrigger) trigger defined
	buildcfg := mockBuildConfig("registry.com/namespace/imagename1", "", "", "")
	buildcfg.Triggers[0].Type = buildapi.GenericWebHookBuildTriggerType
	imagerepo := mockImageRepo("testImageRepo2", "registry.com/namespace/imagename2", map[string]string{"testTag2": "newImageID123"})
	controller := mockImageChangeController(buildcfg)
	err := controller.HandleImageRepo(imagerepo)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if buildCreator.build != nil {
		t.Error("New build created when a different trigger type was defined!")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different trigger was defined!")
	}
}

func TestNoImageIDChange(t *testing.T) {
	// this buildConfig has up to date configuration, but is checked eg. during
	// startup when we're checking all the imageRepos
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	buildcfg.Triggers[0].ImageChange.LastTriggeredImageID = "imageID123"
	imagerepo := mockImageRepo("testImageRepo", "registry.com/namespace/imagename", map[string]string{"testTag": "imageID123"})
	controller := mockImageChangeController(buildcfg)
	err := controller.HandleImageRepo(imagerepo)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if buildCreator.build != nil {
		t.Error("New build created when no change happened!")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when no change happened!")
	}
}

func TestBuildCreateError(t *testing.T) {
	// valid configuration, but build creation fails, in that situation the buildconfig should not be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	imagerepo := mockImageRepo("testImageRepo", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildCreator.err = fmt.Errorf("error")
	err := controller.HandleImageRepo(imagerepo)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err == nil {
		t.Error("Expected error from HandleImageRepo")
	}
	if _, ok := err.(ImageChangeControllerFatalError); ok {
		t.Error("Expected retryable error from HandleImageRepo")
	}
	if buildCreator.build == nil {
		t.Fatal("Expected new build when new image was created!")
	}
	if buildCreator.build.Parameters.Strategy.DockerStrategy.Image != "registry.com/namespace/imagename:newImageID123" {
		t.Errorf("Image substitutions not properly setup for new build.  Expected %s, got %s |", "registry.com/namespace/imagename:newImageID123", buildCreator.build.Parameters.Strategy.DockerStrategy.Image)
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Fatal("Expected no buildConfig update on BuildCreate error!")
	}
}

func TestBuildUpdateError(t *testing.T) {
	// valid configuration, but build creation fails, in that situation the buildconfig should not be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	imagerepo := mockImageRepo("testImageRepo", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)
	buildConfigUpdater.err = fmt.Errorf("error")
	err := controller.HandleImageRepo(imagerepo)

	if _, ok := err.(ImageChangeControllerFatalError); !ok {
		t.Error("Expected fatal error from HandleImageRepo")
	}
	if buildCreator.build == nil {
		t.Fatal("Expected new build when new image was created!")
	}
	if buildCreator.build.Parameters.Strategy.DockerStrategy.Image != "registry.com/namespace/imagename:newImageID123" {
		t.Errorf("Image substitutions not properly setup for new build.  Expected %s, got %s |", "registry.com/namespace/imagename:newImageID123", buildCreator.build.Parameters.Strategy.DockerStrategy.Image)
	}
}

func TestNewImageIDNoDockerRepo(t *testing.T) {
	// No docker repository associated with the imagerepo, so no build can be created
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	imagerepo := mockImageRepo("testImageRepo", "", map[string]string{"testTag": "newImageID123"})
	controller := mockImageChangeController(buildcfg)
	err := controller.HandleImageRepo(imagerepo)
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if err != nil {
		t.Errorf("Unexpected error %v from HandleImageRepo", err)
	}
	if buildCreator.build != nil {
		t.Error("New build created when no change happened!")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when no change happened!")
	}
}
