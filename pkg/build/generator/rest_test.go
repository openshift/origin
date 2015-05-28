package generator

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestCreateClone(t *testing.T) {
	rest := CloneREST{&BuildGenerator{Client: Client{
		CreateBuildFunc: func(ctx kapi.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx kapi.Context, name string) (*buildapi.Build, error) {
			return &buildapi.Build{}, nil
		},
	}}}

	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.BuildRequest{ObjectMeta: kapi.ObjectMeta{Name: "name"}})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCreateCloneObjectError(t *testing.T) {
	rest := CloneREST{&BuildGenerator{}}
	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.Build{})
	if err == nil {
		t.Error("Expected object got none!")
	}
}

func TestCreateCloneValidationError(t *testing.T) {
	rest := CloneREST{&BuildGenerator{}}
	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil {
		t.Error("Expected object got none!")
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

func TestCreateInstantiate(t *testing.T) {
	imageStream := mockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"test": "newImageID123"})
	image := mockImage("testImage@id", "registry.com/namespace/imagename@id")
	fakeSecrets := []runtime.Object{}
	for _, s := range mockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	rest := InstantiateREST{&BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mockBuilderServiceAccount(mockBuilderSecrets()),
		Client: Client{
			GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
				return mockBuildConfig(mockSource(), mockSourceStrategyForImageStreamTag(), mockOutput()), nil
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
				return imageStream, nil
			},
			GetImageStreamTagFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error) {
				return &imageapi.ImageStreamTag{*image, name}, nil
			},
			GetImageStreamImageFunc: func(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error) {
				return &imageapi.ImageStreamImage{Image: *image}, nil
			},
		}}}

	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.BuildRequest{ObjectMeta: kapi.ObjectMeta{Name: "name"}})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCreateInstantiateObjectError(t *testing.T) {
	rest := InstantiateREST{&BuildGenerator{}}
	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.Build{})
	if err == nil {
		t.Error("Expected object got none!")
	}
}

func TestCreateInstantiateValidationError(t *testing.T) {
	rest := InstantiateREST{&BuildGenerator{}}
	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil {
		t.Error("Expected object got none!")
	}
}
