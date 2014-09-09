package image

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/fsouza/go-dockerclient"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/imagetest"
)

func TestGetImageRepositoryMapping(t *testing.T) {
	imageRegistry := imagetest.NewImageRegistry()
	imageRepositoryRegistry := imagetest.NewImageRepositoryRegistry()
	storage := &ImageRepositoryMappingStorage{imageRegistry, imageRepositoryRegistry}

	obj, err := storage.Get("foo")
	if obj != nil {
		t.Errorf("Unexpected non-nil object %#v", obj)
	}
	if err == nil || strings.Index(err.Error(), "not supported") == -1 {
		t.Errorf("Expected 'not supported' error, got %#v", err)
	}
}

func TestListImageRepositoryMappings(t *testing.T) {
	imageRegistry := imagetest.NewImageRegistry()
	imageRepositoryRegistry := imagetest.NewImageRepositoryRegistry()
	storage := &ImageRepositoryMappingStorage{imageRegistry, imageRepositoryRegistry}

	list, err := storage.List(labels.Everything())
	if list != nil {
		t.Errorf("Unexpected non-nil list %#v", list)
	}
	if err == nil || strings.Index(err.Error(), "not supported") == -1 {
		t.Errorf("Expected 'not supported' error, got %#v", err)
	}
}

func TestDeleteImageRepositoryMapping(t *testing.T) {
	imageRegistry := imagetest.NewImageRegistry()
	imageRepositoryRegistry := imagetest.NewImageRepositoryRegistry()
	storage := &ImageRepositoryMappingStorage{imageRegistry, imageRepositoryRegistry}

	channel, err := storage.Delete("repo1")
	if channel != nil {
		t.Errorf("Unexpected non-nil channel %#v", channel)
	}
	if err == nil || strings.Index(err.Error(), "not supported") == -1 {
		t.Errorf("Expected 'not supported' error, got %#v", err)
	}
}

func TestUpdateImageRepositoryMapping(t *testing.T) {
	imageRegistry := imagetest.NewImageRegistry()
	imageRepositoryRegistry := imagetest.NewImageRepositoryRegistry()
	storage := &ImageRepositoryMappingStorage{imageRegistry, imageRepositoryRegistry}

	channel, err := storage.Update("repo1")
	if channel != nil {
		t.Errorf("Unexpected non-nil channel %#v", channel)
	}
	if err == nil || strings.Index(err.Error(), "not supported") == -1 {
		t.Errorf("Expected 'not supported' error, got %#v", err)
	}
}

func TestCreateImageRepositoryMappingBadObject(t *testing.T) {
	imageRegistry := imagetest.NewImageRegistry()
	imageRepositoryRegistry := imagetest.NewImageRepositoryRegistry()
	storage := &ImageRepositoryMappingStorage{imageRegistry, imageRepositoryRegistry}

	channel, err := storage.Create("bad object")
	if channel != nil {
		t.Errorf("Unexpected non-nil channel %#v", channel)
	}
	if err == nil || strings.Index(err.Error(), "not an image repository mapping") == -1 {
		t.Errorf("Expected 'not an image repository mapping' error, got %#v", err)
	}
}

func TestCreateImageRepositoryMappingFindError(t *testing.T) {
	imageRegistry := imagetest.NewImageRegistry()
	imageRepositoryRegistry := imagetest.NewImageRepositoryRegistry()
	imageRepositoryRegistry.Err = fmt.Errorf("123")
	storage := &ImageRepositoryMappingStorage{imageRegistry, imageRepositoryRegistry}

	mapping := api.ImageRepositoryMapping{
		DockerImageRepository: "localhost:5000/someproject/somerepo",
		Image: api.Image{
			JSONBase: kubeapi.JSONBase{
				ID: "imageID1",
			},
			DockerImageReference: "localhost:5000/someproject/somerepo:imageID1",
		},
		Tag: "latest",
	}

	channel, err := storage.Create(&mapping)
	if channel != nil {
		t.Errorf("Unexpected non-nil channel %#v", channel)
	}
	if err == nil || err.Error() != "123" {
		t.Errorf("Expected 'unable to locate' error, got %#v", err)
	}
}

func TestCreateImageRepositoryMappingNotFound(t *testing.T) {
	imageRegistry := imagetest.NewImageRegistry()
	imageRepositoryRegistry := imagetest.NewImageRepositoryRegistry()
	imageRepositoryRegistry.ImageRepositories = &api.ImageRepositoryList{
		Items: []api.ImageRepository{
			{
				JSONBase: kubeapi.JSONBase{
					ID: "repo1",
				},
				DockerImageRepository: "localhost:5000/test/repo",
			},
		},
	}
	storage := &ImageRepositoryMappingStorage{imageRegistry, imageRepositoryRegistry}

	mapping := api.ImageRepositoryMapping{
		DockerImageRepository: "localhost:5000/someproject/somerepo",
		Image: api.Image{
			JSONBase: kubeapi.JSONBase{
				ID: "imageID1",
			},
			DockerImageReference: "localhost:5000/someproject/somerepo:imageID1",
		},
		Tag: "latest",
	}

	channel, err := storage.Create(&mapping)
	if channel != nil {
		t.Errorf("Unexpected non-nil channel %#v", channel)
	}
	if err == nil || strings.Index(err.Error(), "Unable to locate an image repository") == -1 {
		t.Errorf("Expected 'unable to locate' error, got %#v", err)
	}
}

func TestCreateImageRepositoryMapping(t *testing.T) {
	imageRegistry := imagetest.NewImageRegistry()
	imageRepositoryRegistry := imagetest.NewImageRepositoryRegistry()
	imageRepositoryRegistry.ImageRepositories = &api.ImageRepositoryList{
		Items: []api.ImageRepository{
			{
				JSONBase: kubeapi.JSONBase{
					ID: "repo1",
				},
				DockerImageRepository: "localhost:5000/someproject/somerepo",
			},
		},
	}
	storage := &ImageRepositoryMappingStorage{imageRegistry, imageRepositoryRegistry}

	mapping := api.ImageRepositoryMapping{
		DockerImageRepository: "localhost:5000/someproject/somerepo",
		Image: api.Image{
			JSONBase: kubeapi.JSONBase{
				ID: "imageID1",
			},
			DockerImageReference: "localhost:5000/someproject/somerepo:imageID1",
			Metadata: docker.Image{
				Config: &docker.Config{
					Cmd:          []string{"ls", "/"},
					Env:          []string{"a=1"},
					ExposedPorts: map[docker.Port]struct{}{"1234/tcp": {}},
					Memory:       1234,
					CpuShares:    99,
					WorkingDir:   "/workingDir",
				},
			},
		},
		Tag: "latest",
	}
	ch, err := storage.Create(&mapping)
	if err != nil {
		t.Errorf("Unexpected error creating mapping: %#v", err)
	}

	out := <-ch
	t.Logf("out = '%#v'", out)

	image, err := imageRegistry.GetImage("imageID1")
	if err != nil {
		t.Errorf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.Metadata, image.Metadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}
}
