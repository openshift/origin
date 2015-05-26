package controller

import (
	"fmt"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildtest "github.com/openshift/origin/pkg/build/controller/test"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestNewImageID(t *testing.T) {
	// valid configuration, new build should be triggered.
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "testTag")
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	image := mockImage("testImage@id", "registry.com/namespace/imagename:newImageID123")
	controller := mockImageChangeController(buildcfg, imageStream, image)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Fatalf("Unexpected error %v from HandleImageRepo", err)
	}

	if len(bcInstantiator.name) == 0 {
		t.Error("Expected build generation when new image was created!")
	}
	if actual, expected := bcInstantiator.newBuild.Parameters.Strategy.DockerStrategy.From.Name, "registry.com/namespace/imagename:newImageID123"; actual != expected {
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
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{imageapi.DefaultImageTag: "newImageID123"})
	image := mockImage("testImage@id", "registry.com/namespace/imagename:newImageID123")
	controller := mockImageChangeController(buildcfg, imageStream, image)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Fatalf("Unexpected error %v from HandleImageRepo", err)
	}
	if len(bcInstantiator.name) == 0 {
		t.Error("Expected build generation when new image was created!")
	}
	if actual, expected := bcInstantiator.newBuild.Parameters.Strategy.DockerStrategy.From.Name, "registry.com/namespace/imagename:newImageID123"; actual != expected {
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
	image := mockImage("testImage@id", "registry.com/namespace/imagename@id")
	controller := mockImageChangeController(buildcfg, imageStream, image)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err != nil {
		t.Fatalf("Unexpected error %v from HandleImageRepo", err)
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
	image := mockImage("testImage@id", "registry.com/namespace/imagename@id")
	controller := mockImageChangeController(buildcfg, imageStream, image)
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
	image := mockImage("testImage@id", "registry.com/namespace/imagename@id")
	controller := mockImageChangeController(buildcfg, imageStream, image)
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
	image := mockImage("testImage@id", "registry.com/namespace/imagename@id")
	controller := mockImageChangeController(buildcfg, imageStream, image)
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

func TestSameStreamNameDifferentNamespaces(t *testing.T) {
	// this buildconfig references an image stream with the same name as the one that was just updated,
	// but the namespaces differ
	buildcfg := mockBuildConfig("registry.com/namespace/imagename1", "registry.com/namespace/imagename1", "testImageRepo1", "testTag1")
	imageStream := mockImageStream("testImageRepo1", "registry.com/namespace/imagename2", map[string]string{"testTag1": "newImageID123"})
	imageStream.Namespace = "othernamespace"
	image := mockImage("testImage@id", "registry.com/namespace/imagename@id")
	controller := mockImageChangeController(buildcfg, imageStream, image)
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

func TestBuildConfigWithDifferentTriggerType(t *testing.T) {
	// this buildconfig has different (than ImageChangeTrigger) trigger defined
	buildcfg := mockBuildConfig("registry.com/namespace/imagename1", "", "", "")
	buildcfg.Triggers[0].Type = buildapi.GenericWebHookBuildTriggerType
	imageStream := mockImageStream("testImageRepo2", "registry.com/namespace/imagename2", map[string]string{"testTag2": "newImageID123"})
	image := mockImage("testImage@id", "registry.com/namespace/imagename@id")
	controller := mockImageChangeController(buildcfg, imageStream, image)
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
	image := mockImage("testImage@id", "registry.com/namespace/imagename@id")
	controller := mockImageChangeController(buildcfg, imageStream, image)
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
	image := mockImage("testImage@id", "registry.com/namespace/imagename:newImageID123")
	controller := mockImageChangeController(buildcfg, imageStream, image)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcInstantiator.err = fmt.Errorf("instantiating error")
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)

	err := controller.HandleImageRepo(imageStream)
	if err == nil || !strings.Contains(err.Error(), "instantiating error") {
		t.Fatalf("Expected error from HandleImageRepo")
	}
	if actual, expected := bcInstantiator.newBuild.Parameters.Strategy.DockerStrategy.From.Name, "registry.com/namespace/imagename:newImageID123"; actual != expected {
		t.Errorf("Image substitutions not properly setup for new build. Expected %s, got %s |", expected, actual)
	}
	if bcUpdater.updateCount > 1 {
		t.Fatal("Expected no buildConfig update on BuildCreate error!")
	}
}

func TestBuildConfigUpdateError(t *testing.T) {
	// valid configuration, but build creation fails, in that situation the buildconfig should not be updated
	buildcfg := mockBuildConfig("registry.com/namespace/imagename", "registry.com/namespace/imagename", "testImageStream", "testTag")
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"testTag": "newImageID123"})
	image := mockImage("testImage@id", "registry.com/namespace/imagename@id")
	controller := mockImageChangeController(buildcfg, imageStream, image)
	bcInstantiator := controller.BuildConfigInstantiator.(*buildConfigInstantiator)
	bcUpdater := controller.BuildConfigUpdater.(*mockBuildConfigUpdater)
	bcUpdater.err = fmt.Errorf("error")
	bcUpdater.errUpdateCount = 2

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
	image := mockImage("testImage@id", "registry.com/namespace/imagename@id")
	controller := mockImageChangeController(buildcfg, imageStream, image)
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

type mockBuildConfigUpdater struct {
	updateCount    int
	buildcfg       *buildapi.BuildConfig
	err            error
	errUpdateCount int
}

func (m *mockBuildConfigUpdater) Update(buildcfg *buildapi.BuildConfig) error {
	m.buildcfg = buildcfg
	m.updateCount++
	if m.errUpdateCount > 0 {
		if m.updateCount == m.errUpdateCount {
			return m.err
		}
		return nil
	}
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
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: repoName + ":" + repoTag,
					},
				},
			},
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
		},
	}
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

func mockImage(name, dockerSpec string) *imageapi.Image {
	return &imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		DockerImageReference: dockerSpec,
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

func mockBuildConfigInstantiator(buildcfg *buildapi.BuildConfig, imageStream *imageapi.ImageStream, image *imageapi.Image) *buildConfigInstantiator {
	builderAccount := kapi.ServiceAccount{
		ObjectMeta: kapi.ObjectMeta{Name: bootstrappolicy.BuilderServiceAccountName},
		Secrets:    []kapi.ObjectReference{},
	}
	instantiator := &buildConfigInstantiator{}
	generator := buildgenerator.BuildGenerator{
		Secrets:         testclient.NewSimpleFake(),
		ServiceAccounts: testclient.NewSimpleFake(&builderAccount),
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
			GetImageStreamTagFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error) {
				return &imageapi.ImageStreamTag{Image: *image, ImageName: name}, nil
			},
			GetImageStreamImageFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error) {
				return &imageapi.ImageStreamImage{Image: *image}, nil
			},
		}}
	instantiator.generator = generator
	return instantiator
}

func mockImageChangeController(buildcfg *buildapi.BuildConfig, imageStream *imageapi.ImageStream, image *imageapi.Image) *ImageChangeController {
	return &ImageChangeController{
		BuildConfigStore:        buildtest.NewFakeBuildConfigStore(buildcfg),
		BuildConfigInstantiator: mockBuildConfigInstantiator(buildcfg, imageStream, image),
		BuildConfigUpdater:      &mockBuildConfigUpdater{},
	}
}
