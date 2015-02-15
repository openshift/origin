package imagerepositorymapping

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/test"
)

func TestCreateImageRepositoryMappingFindError(t *testing.T) {
	imageRegistry := test.NewImageRegistry()
	imageRepositoryRegistry := test.NewImageRepositoryRegistry()
	imageRepositoryRegistry.Err = fmt.Errorf("123")
	storage := &REST{imageRegistry, imageRepositoryRegistry}

	mapping := api.ImageRepositoryMapping{
		DockerImageRepository: "localhost:5000/someproject/somerepo",
		Image: api.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "imageID1",
			},
			DockerImageReference: "localhost:5000/someproject/somerepo:imageID1",
		},
		Tag: "latest",
	}

	obj, err := storage.Create(kapi.NewDefaultContext(), &mapping)
	if obj != nil {
		t.Fatalf("Unexpected non-nil obj %#v", obj)
	}
	if err == nil {
		t.Fatal("Unexpected nil err")
	}
	if err.Error() != "123" {
		t.Errorf("Expected 'unable to locate' error, got %#v", err)
	}
}

func TestCreateImageRepositoryMappingNotFound(t *testing.T) {
	imageRegistry := test.NewImageRegistry()
	imageRepositoryRegistry := test.NewImageRepositoryRegistry()
	imageRepositoryRegistry.ImageRepositories = &api.ImageRepositoryList{
		Items: []api.ImageRepository{
			{
				ObjectMeta: kapi.ObjectMeta{
					Name: "repo1",
				},
				DockerImageRepository: "localhost:5000/test/repo",
			},
		},
	}
	storage := &REST{imageRegistry, imageRepositoryRegistry}

	mapping := api.ImageRepositoryMapping{
		DockerImageRepository: "localhost:5000/someproject/somerepo",
		Image: api.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "imageID1",
			},
			DockerImageReference: "localhost:5000/someproject/somerepo:imageID1",
		},
		Tag: "latest",
	}

	obj, err := storage.Create(kapi.NewDefaultContext(), &mapping)
	if obj != nil {
		t.Errorf("Unexpected non-nil obj %#v", obj)
	}
	if err == nil {
		t.Fatal("Unexpected nil err")
	}
	if !errors.IsInvalid(err) {
		t.Fatalf("Expected 'invalid' err, got: %#v", err)
	}
}

func TestCreateImageRepositoryMapping(t *testing.T) {
	imageRegistry := test.NewImageRegistry()
	imageRepositoryRegistry := test.NewImageRepositoryRegistry()
	imageRepositoryRegistry.ImageRepositories = &api.ImageRepositoryList{
		Items: []api.ImageRepository{
			{
				ObjectMeta: kapi.ObjectMeta{
					Name: "repo1",
				},
				DockerImageRepository: "localhost:5000/someproject/somerepo",
			},
		},
	}
	storage := &REST{imageRegistry, imageRepositoryRegistry}

	mapping := api.ImageRepositoryMapping{
		DockerImageRepository: "localhost:5000/someproject/somerepo",
		Image: api.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "imageID1",
			},
			DockerImageReference: "localhost:5000/someproject/somerepo:imageID1",
			DockerImageMetadata: api.DockerImage{
				Config: api.DockerConfig{
					Cmd:          []string{"ls", "/"},
					Env:          []string{"a=1"},
					ExposedPorts: map[string]struct{}{"1234/tcp": {}},
					Memory:       1234,
					CPUShares:    99,
					WorkingDir:   "/workingDir",
				},
			},
		},
		Tag: "latest",
	}
	_, err := storage.Create(kapi.NewDefaultContext(), &mapping)
	if err != nil {
		t.Errorf("Unexpected error creating mapping: %#v", err)
	}

	image, err := imageRegistry.GetImage(kapi.NewDefaultContext(), "imageID1")
	if err != nil {
		t.Errorf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo, err := imageRepositoryRegistry.GetImageRepository(kapi.NewDefaultContext(), "repo1")
	if err != nil {
		t.Errorf("Unexpected non-nil err: %#v", err)
	}
	if e, a := "imageID1", repo.Tags["latest"]; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
}

func TestCreateImageRepositoryConflictingNamespace(t *testing.T) {
	imageRegistry := test.NewImageRegistry()
	imageRepositoryRegistry := test.NewImageRepositoryRegistry()
	imageRepositoryRegistry.ImageRepositories = &api.ImageRepositoryList{
		Items: []api.ImageRepository{
			{
				ObjectMeta: kapi.ObjectMeta{
					Name: "repo1",
				},
				DockerImageRepository: "localhost:5000/someproject/somerepo",
			},
		},
	}
	storage := &REST{imageRegistry, imageRepositoryRegistry}

	mapping := api.ImageRepositoryMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "some-value",
		},
		DockerImageRepository: "localhost:5000/someproject/somerepo",
		Image: api.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "imageID1",
			},
			DockerImageReference: "localhost:5000/someproject/somerepo:imageID1",
			DockerImageMetadata: api.DockerImage{
				Config: api.DockerConfig{
					Cmd:          []string{"ls", "/"},
					Env:          []string{"a=1"},
					ExposedPorts: map[string]struct{}{"1234/tcp": {}},
					Memory:       1234,
					CPUShares:    99,
					WorkingDir:   "/workingDir",
				},
			},
		},
		Tag: "latest",
	}

	ch, err := storage.Create(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &mapping)
	if ch != nil {
		t.Error("Expected a nil obj, but we got a value")
	}
	checkExpectedNamespaceError(t, err)
}

func checkExpectedNamespaceError(t *testing.T, err error) {
	expectedError := "ImageRepositoryMapping.Namespace does not match the provided context"
	if err == nil {
		t.Errorf("Expected '" + expectedError + "', but we didn't get one")
	} else {
		e, ok := err.(kclient.APIStatus)
		if !ok {
			t.Errorf("error was not a statusError: %v", err)
		}
		if e.Status().Code != http.StatusConflict {
			t.Errorf("Unexpected failure status: %v", e.Status())
		}
		if strings.Index(err.Error(), expectedError) == -1 {
			t.Errorf("Expected '"+expectedError+"' error, got '%v'", err.Error())
		}
	}

}
