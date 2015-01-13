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
}

func (m *mockBuildConfigUpdater) UpdateBuildConfig(buildcfg *buildapi.BuildConfig) error {
	m.buildcfg = buildcfg
	return nil
}

type mockBuildCreator struct {
	buildcfg           *buildapi.BuildConfig
	imageSubstitutions map[string]string
	err                error
}

func (m *mockBuildCreator) CreateBuild(buildcfg *buildapi.BuildConfig, imageSubstitutions map[string]string) error {
	m.buildcfg = buildcfg
	m.imageSubstitutions = imageSubstitutions
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
					ContextDir: "contextimage",
					BaseImage:  baseImage,
				},
			},
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					Image: triggerImage,
					ImageRepositoryRef: &kapi.ObjectReference{
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
			ImageRepositoryRef: &kapi.ObjectReference{
				Name: repoName,
			},
			Tag: repoTag,
		},
	})
}

func mockImageChangeController(buildcfg *buildapi.BuildConfig, repoName, dockerImageRepo string, tags map[string]string) *ImageChangeController {
	imageRepo := imageapi.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{
			Name: repoName,
		},
		DockerImageRepository: dockerImageRepo,
		Tags: tags,
	}

	return &ImageChangeController{
		NextImageRepository: func() *imageapi.ImageRepository { return &imageRepo },
		BuildConfigStore:    buildtest.NewFakeBuildConfigStore(buildcfg),
		BuildConfigUpdater:  &mockBuildConfigUpdater{},
		BuildCreator:        &mockBuildCreator{},
	}
}

func TestNewImageID(t *testing.T) {
	// valid configuration, new build should be triggered.
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	controller := mockImageChangeController(buildcfg, "testImageRepo", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	controller.HandleImageRepo()
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if buildCreator.buildcfg == nil {
		t.Error("Expected new build when new image was created!")
	}
	if buildCreator.imageSubstitutions["registry.com/namespace/imagename"] != "registry.com/namespace/imagename:newImageID123" {
		t.Errorf("Image substitutions not properly setup for new build: %s |", buildCreator.imageSubstitutions["registry.com/namespace/imagename"])
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
	controller := mockImageChangeController(buildcfg, "testImageRepo", "registry.com/namespace/imagename", map[string]string{buildapi.DefaultImageTag: "newImageID123"})
	controller.HandleImageRepo()
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if buildCreator.buildcfg == nil {
		t.Error("Expected new build when new image was created!")
	}
	if buildCreator.imageSubstitutions["registry.com/namespace/imagename"] != "registry.com/namespace/imagename:newImageID123" {
		t.Errorf("Image substitutions not properly setup for new build using default tag: %s |", buildCreator.imageSubstitutions["registry.com/namespace/imagename"])
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
	controller := mockImageChangeController(buildcfg, "otherImageRepo", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	controller.HandleImageRepo()
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if buildCreator.buildcfg != nil {
		t.Error("New build created when a different repository was updated!")
	}
	if len(buildCreator.imageSubstitutions) != 0 {
		t.Errorf("Should not have had any image substitutions since tag does not exist in imagerepo")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestNewImageDifferentTagUpdate(t *testing.T) {
	// this buildconfig references a different tag than the one that will be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	controller := mockImageChangeController(buildcfg, "testImageRepo", "registry.com/namespace/imagename", map[string]string{"otherTag": "newImageID123"})
	controller.HandleImageRepo()
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if buildCreator.buildcfg != nil {
		t.Error("New build created when a different repository was updated!")
	}
	if len(buildCreator.imageSubstitutions) != 0 {
		t.Errorf("Should not have had any image substitutions since tag does not exist in imagerepo")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestNewImageDifferentTagUpdate2(t *testing.T) {
	// this buildconfig references a different tag than the one that will be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	buildcfg.Triggers[0].ImageChange.LastTriggeredImageID = "testTagID123"
	controller := mockImageChangeController(buildcfg, "testImageRepo", "registry.com/namespace/imagename",
		map[string]string{"otherTag": "newImageID123", "testTag": "testTagID123"})
	controller.HandleImageRepo()
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if buildCreator.buildcfg != nil {
		t.Error("New build created when a different repository was updated!")
	}
	if len(buildCreator.imageSubstitutions) != 0 {
		t.Errorf("Should not have had any image substitutions since tag does not exist in imagerepo")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestNewDifferentImageUpdate(t *testing.T) {
	// this buildconfig references a different image than the one that will be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename1", "registry.com/namespace/imagename1", "testImageRepo1", "testTag1")
	controller := mockImageChangeController(buildcfg, "testImageRepo2", "registry.com/namespace/imagename2", map[string]string{"testTag2": "newImageID123"})
	controller.HandleImageRepo()
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if buildCreator.buildcfg != nil {
		t.Error("New build created when a different repository was updated!")
	}
	if len(buildCreator.imageSubstitutions) != 0 {
		t.Errorf("Should not have had any image substitutions since tag does not exist in imagerepo")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when a different repository was updated!")
	}
}

func TestMultipleTriggers(t *testing.T) {
	// this buildconfig references multiple images
	buildcfg := mockBuildConfig("registry.com/namespace/imagename1", "registry.com/namespace/imagename1", "testImageRepo1", "testTag1")
	appendTrigger(buildcfg, "registry.com/namespace/imagename2", "testImageRepo2", "testTag2")
	controller := mockImageChangeController(buildcfg, "testImageRepo2", "registry.com/namespace/imagename2", map[string]string{"testTag2": "newImageID123"})
	controller.HandleImageRepo()
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if buildCreator.buildcfg == nil {
		t.Error("Expected new build when new image was created!")
	}
	if len(buildCreator.imageSubstitutions) != 1 {
		t.Errorf("Expected exactly 1 substitution, got different count: %d", len(buildCreator.imageSubstitutions))
	}
	if buildCreator.imageSubstitutions["registry.com/namespace/imagename2"] != "registry.com/namespace/imagename2:newImageID123" {
		t.Errorf("Image substitutions not properly setup for new build using default tag: %s |", buildCreator.imageSubstitutions["registry.com/namespace/imagename2"])
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
	controller := mockImageChangeController(buildcfg, "testImageRepo2", "registry.com/namespace/imagename2", map[string]string{"testTag2": "newImageID123"})
	controller.HandleImageRepo()
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if buildCreator.buildcfg != nil {
		t.Error("New build created when a different trigger type was defined!")
	}
	if len(buildCreator.imageSubstitutions) != 0 {
		t.Errorf("Should not have had any image substitutions since different trigger type was defined!")
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
	controller := mockImageChangeController(buildcfg, "testImageRepo", "registry.com/namespace/imagename", map[string]string{"testTag": "imageID123"})
	controller.HandleImageRepo()
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if buildCreator.buildcfg != nil {
		t.Error("New build created when no change happened!")
	}
	if len(buildCreator.imageSubstitutions) != 0 {
		t.Errorf("Should not have had any image substitutions since no change happened!")
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Error("BuildConfig was updated when no change happened!")
	}
}

func TestBuildCreateError(t *testing.T) {
	// valid configuration, but build creation failes, in that situation the buildconfig should not be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageRepo", "testTag")
	controller := mockImageChangeController(buildcfg, "testImageRepo", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	buildCreator := controller.BuildCreator.(*mockBuildCreator)
	buildCreator.err = fmt.Errorf("error")
	controller.HandleImageRepo()
	buildConfigUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	if buildCreator.buildcfg == nil {
		t.Error("Expected new build when new image was created!")
	}
	if buildCreator.imageSubstitutions["registry.com/namespace/imagename"] != "registry.com/namespace/imagename:newImageID123" {
		t.Errorf("Image substitutions not properly setup for new build: %s |", buildCreator.imageSubstitutions["registry.com/namespace/imagename"])
	}
	if buildConfigUpdater.buildcfg != nil {
		t.Fatal("Expected no buildConfig update on BuildCreate error!")
	}
}
