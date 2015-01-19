package imagerepositorytag

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/test"
)

type statusError interface {
	Status() kapi.Status
}

func TestGetImageRepositoryTag(t *testing.T) {
	images := test.NewImageRegistry()
	images.Image = &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"}
	repositories := test.NewImageRepositoryRegistry()
	repositories.ImageRepository = &api.ImageRepository{Tags: map[string]string{"latest": "10"}}

	storage := &REST{images, repositories}

	obj, err := storage.Get(kapi.NewDefaultContext(), "test:latest")
	if err != nil {
		t.Fatalf("Unexpected err: %v", err)
	}
	actual := obj.(*api.Image)
	if actual != images.Image {
		t.Errorf("unexpected image: %#v", actual)
	}
}

func TestGetImageRepositoryTagMissingImage(t *testing.T) {
	images := test.NewImageRegistry()
	images.Err = errors.NewNotFound("image", "10")
	//images.Image = &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"}
	repositories := test.NewImageRepositoryRegistry()
	repositories.ImageRepository = &api.ImageRepository{Tags: map[string]string{"latest": "10"}}

	storage := &REST{images, repositories}

	_, err := storage.Get(kapi.NewDefaultContext(), "test:latest")
	if err == nil {
		t.Fatal("unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("unexpected error type: %v", err)
	}
	status := err.(statusError).Status()
	if status.Details.Kind != "image" || status.Details.ID != "10" {
		t.Errorf("unexpected status: %#v", status)
	}
}

func TestGetImageRepositoryTagMissingRepository(t *testing.T) {
	images := test.NewImageRegistry()
	images.Image = &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"}
	repositories := test.NewImageRepositoryRegistry()
	repositories.Err = errors.NewNotFound("imageRepository", "test")
	//repositories.ImageRepository = &api.ImageRepository{Tags: map[string]string{"latest": "10"}}

	storage := &REST{images, repositories}

	_, err := storage.Get(kapi.NewDefaultContext(), "test:latest")
	if err == nil {
		t.Fatal("unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("unexpected error type: %v", err)
	}
	status := err.(statusError).Status()
	if status.Details.Kind != "imageRepository" || status.Details.ID != "test" {
		t.Errorf("unexpected status: %#v", status)
	}
}

func TestGetImageRepositoryTagMissingTag(t *testing.T) {
	images := test.NewImageRegistry()
	images.Image = &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"}
	repositories := test.NewImageRepositoryRegistry()
	repositories.ImageRepository = &api.ImageRepository{Tags: map[string]string{"other": "10"}}

	storage := &REST{images, repositories}

	_, err := storage.Get(kapi.NewDefaultContext(), "test:latest")
	if err == nil {
		t.Fatal("unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("unexpected error type: %v", err)
	}
	status := err.(statusError).Status()
	if status.Details.Kind != "imageRepositoryTag" || status.Details.ID != "latest" {
		t.Errorf("unexpected status: %#v", status)
	}
}
