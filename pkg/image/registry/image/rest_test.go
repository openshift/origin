package image

import (
	"fmt"
	"strings"
	"testing"
	"time"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/test"
)

func TestListImagesError(t *testing.T) {
	mockRegistry := test.NewImageRegistry()
	mockRegistry.Err = fmt.Errorf("test error")

	storage := REST{
		registry: mockRegistry,
	}

	images, err := storage.List(nil, nil, nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}

	if images != nil {
		t.Errorf("Unexpected non-nil images list: %#v", images)
	}
}

func TestListImagesEmptyList(t *testing.T) {
	mockRegistry := test.NewImageRegistry()
	mockRegistry.Images = &api.ImageList{
		Items: []api.Image{},
	}

	storage := REST{
		registry: mockRegistry,
	}

	images, err := storage.List(nil, labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(images.(*api.ImageList).Items) != 0 {
		t.Errorf("Unexpected non-zero images list: %#v", images)
	}
}

func TestListImagesPopulatedList(t *testing.T) {
	mockRegistry := test.NewImageRegistry()
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

	storage := REST{
		registry: mockRegistry,
	}

	list, err := storage.List(nil, labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	images := list.(*api.ImageList)

	if e, a := 2, len(images.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateImageBadObject(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(nil, &api.ImageList{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not an image:") == -1 {
		t.Errorf("Expected 'not an image' error, got %v", err)
	}
}

func TestCreateImageMissingID(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(nil, &api.Image{})
	if channel != nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if !errors.IsInvalid(err) {
		t.Errorf("Expected 'invalid' error, got %v", err)
	}
}

func TestCreateRegistrySaveError(t *testing.T) {
	mockRegistry := test.NewImageRegistry()
	mockRegistry.Err = fmt.Errorf("test error")
	storage := REST{registry: mockRegistry}

	channel, err := storage.Create(nil, &api.Image{
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
		if status.Status != kubeapi.StatusFailure || status.Message != "test error" {
			t.Errorf("Expected failure status, got %#v", status)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	}
}

func TestCreateImageOK(t *testing.T) {
	mockRegistry := test.NewImageRegistry()
	storage := REST{registry: mockRegistry}

	channel, err := storage.Create(nil, &api.Image{
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
	}
}

func TestGetImageError(t *testing.T) {
	mockRegistry := test.NewImageRegistry()
	mockRegistry.Err = fmt.Errorf("bad")
	storage := REST{registry: mockRegistry}

	image, err := storage.Get(nil, "foo")
	if image != nil {
		t.Errorf("Unexpected non-nil image: %#v", image)
	}
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, got %#v", mockRegistry.Err, err)
	}
}

func TestGetImageOK(t *testing.T) {
	mockRegistry := test.NewImageRegistry()
	mockRegistry.Image = &api.Image{
		JSONBase:             kubeapi.JSONBase{ID: "foo"},
		DockerImageReference: "openshift/ruby-19-centos",
	}
	storage := REST{registry: mockRegistry}

	image, err := storage.Get(nil, "foo")
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
	storage := REST{}
	channel, err := storage.Update(nil, &api.Image{})
	if channel != nil {
		t.Errorf("Unexpected non-nil channel: %#v", channel)
	}
	if err == nil {
		t.Fatal("Unexpected nil err")
	}
	if strings.Index(err.Error(), "Images may not be changed.") == -1 {
		t.Errorf("Expected 'may not be changed' error, got: %#v", err)
	}
}

func TestDeleteImage(t *testing.T) {
	mockRegistry := test.NewImageRegistry()
	storage := REST{registry: mockRegistry}
	channel, err := storage.Delete(nil, "foo")
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
		if status.Status != kubeapi.StatusSuccess {
			t.Errorf("Expected status=success, got: %#v", status)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	}
}
