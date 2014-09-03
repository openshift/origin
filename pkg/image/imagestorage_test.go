package image

import (
	"fmt"
	"strings"
	"testing"
	"time"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/imagetest"
)

func TestListImagesError(t *testing.T) {
	mockRegistry := imagetest.NewImageRegistry()
	mockRegistry.Err = fmt.Errorf("test error")

	storage := ImageStorage{
		registry: mockRegistry,
	}

	images, err := storage.List(nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}

	if images != nil {
		t.Errorf("Unexpected non-nil images list: %#v", images)
	}
}

func TestListImagesEmptyList(t *testing.T) {
	mockRegistry := imagetest.NewImageRegistry()
	mockRegistry.Images = &api.ImageList{
		Items: []api.Image{},
	}

	storage := ImageStorage{
		registry: mockRegistry,
	}

	images, err := storage.List(labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(images.(*api.ImageList).Items) != 0 {
		t.Errorf("Unexpected non-zero images list: %#v", images)
	}
}

func TestListImagesPopulatedList(t *testing.T) {
	mockRegistry := imagetest.NewImageRegistry()
	mockRegistry.Images = &api.ImageList{
		Items: []api.Image{
			{
				JSONBase: kubeapi.JSONBase{
					ID: "foo",
				},
			},
			{
				JSONBase: kubeapi.JSONBase{
					ID: "bar",
				},
			},
		},
	}

	storage := ImageStorage{
		registry: mockRegistry,
	}

	list, err := storage.List(labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	images := list.(*api.ImageList)

	if e, a := 2, len(images.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateImageBadObject(t *testing.T) {
	storage := ImageStorage{}

	channel, err := storage.Create("hello")
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not an image:") == -1 {
		t.Errorf("Expected 'not an image' error, got %v", err)
	}
}

func TestCreateImageMissingID(t *testing.T) {
	storage := ImageStorage{}

	channel, err := storage.Create(&api.Image{})
	if channel != nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if !kubeerrors.IsInvalid(err) {
		t.Errorf("Expected 'invalid' error, got %v", err)
	}
}

func TestCreateImageRegistrySaveError(t *testing.T) {
	mockRegistry := imagetest.NewImageRegistry()
	mockRegistry.Err = fmt.Errorf("test error")
	storage := ImageStorage{registry: mockRegistry}

	channel, err := storage.Create(&api.Image{
		JSONBase:             kubeapi.JSONBase{ID: "foo"},
		DockerImageReference: "openshift/ruby-19-centos",
	})
	if channel == nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		status, ok := result.(*kubeapi.Status)
		if !ok {
			t.Errorf("Expected status type, got: %#v", result)
		}
		if status.Status != "failure" || status.Message != "foo" {
			t.Errorf("Expected failure status, got %#V", status)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	default:
	}
}

func TestCreateImageOK(t *testing.T) {
	mockRegistry := imagetest.NewImageRegistry()
	storage := ImageStorage{registry: mockRegistry}

	channel, err := storage.Create(&api.Image{
		JSONBase:             kubeapi.JSONBase{ID: "foo"},
		DockerImageReference: "openshift/ruby-19-centos",
	})
	if channel == nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		image, ok := result.(*api.Image)
		if !ok {
			t.Errorf("Expected image type, got: %#v", result)
		}
		if image.ID != "foo" {
			t.Errorf("Unexpected image: %#v", image)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	default:
	}
}

func TestGetImageError(t *testing.T) {
	mockRegistry := imagetest.NewImageRegistry()
	mockRegistry.Err = fmt.Errorf("bad")
	storage := ImageStorage{registry: mockRegistry}

	image, err := storage.Get("foo")
	if image != nil {
		t.Errorf("Unexpected non-nil image: %#v", image)
	}
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, got %#v", mockRegistry.Err, err)
	}
}

func TestGetImageOK(t *testing.T) {
	mockRegistry := imagetest.NewImageRegistry()
	mockRegistry.Image = &api.Image{
		JSONBase:             kubeapi.JSONBase{ID: "foo"},
		DockerImageReference: "openshift/ruby-19-centos",
	}
	storage := ImageStorage{registry: mockRegistry}

	image, err := storage.Get("foo")
	if image == nil {
		t.Error("Unexpected nil image")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error", err)
	}
	if image.(*api.Image).ID != "foo" {
		t.Errorf("Unexpected image: %#v", image)
	}
}

func TestUpdateImage(t *testing.T) {
	storage := ImageStorage{}
	channel, err := storage.Update(&api.Image{})
	if channel != nil {
		t.Errorf("Unexpected non-nil channel: %#v", channel)
	}
	if err == nil || strings.Index(err.Error(), "not supported") == -1 {
		t.Errorf("Expected 'not supported' error, got: %#v", err)
	}
}

func TestDeleteImage(t *testing.T) {
	mockRegistry := imagetest.NewImageRegistry()
	storage := ImageStorage{registry: mockRegistry}
	channel, err := storage.Delete("foo")
	if channel == nil {
		t.Error("Unexpected nil channel")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		status, ok := result.(*kubeapi.Status)
		if !ok {
			t.Errorf("Expected status type, got: %#v", result)
		}
		if status.Status != "success" {
			t.Errorf("Expected status=success, got: %#v", status)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	default:
	}
}
