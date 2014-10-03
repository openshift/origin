package imagerepository

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/test"
)

func TestGetImageRepositoryError(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("test error")
	storage := REST{registry: mockRepositoryRegistry}

	image, err := storage.Get(nil, "image1")
	if image != nil {
		t.Errorf("Unexpected non-nil image: %#v", image)
	}
	if err != mockRepositoryRegistry.Err {
		t.Errorf("Expected %#v, got %#v", mockRepositoryRegistry.Err, err)
	}
}

func TestGetImageRepositoryOK(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.ImageRepository = &api.ImageRepository{
		JSONBase:              kubeapi.JSONBase{ID: "foo"},
		DockerImageRepository: "openshift/ruby-19-centos",
	}
	storage := REST{registry: mockRepositoryRegistry}

	repo, err := storage.Get(nil, "foo")
	if repo == nil {
		t.Errorf("Unexpected nil repo: %#v", repo)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	if e, a := mockRepositoryRegistry.ImageRepository, repo; !reflect.DeepEqual(e, a) {
		t.Errorf("Expected %#v, got %#v", e, a)
	}
}

func TestListImageRepositoriesError(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("test error")

	storage := REST{
		registry: mockRepositoryRegistry,
	}

	imageRepositories, err := storage.List(nil, nil, nil)
	if err != mockRepositoryRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRepositoryRegistry.Err, err)
	}

	if imageRepositories != nil {
		t.Errorf("Unexpected non-nil imageRepositories list: %#v", imageRepositories)
	}
}

func TestListImageRepositoriesEmptyList(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.ImageRepositories = &api.ImageRepositoryList{
		Items: []api.ImageRepository{},
	}

	storage := REST{
		registry: mockRepositoryRegistry,
	}

	imageRepositories, err := storage.List(nil, labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(imageRepositories.(*api.ImageRepositoryList).Items) != 0 {
		t.Errorf("Unexpected non-zero imageRepositories list: %#v", imageRepositories)
	}
}

func TestListImageRepositoriesPopulatedList(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.ImageRepositories = &api.ImageRepositoryList{
		Items: []api.ImageRepository{
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
		registry: mockRepositoryRegistry,
	}

	list, err := storage.List(nil, labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	imageRepositories := list.(*api.ImageRepositoryList)

	if e, a := 2, len(imageRepositories.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateImageRepositoryBadObject(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(nil, &api.ImageList{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not an image repository:") == -1 {
		t.Errorf("Expected 'not an image repository' error, got %v", err)
	}
}

func TestCreateImageRepositoryOK(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Create(nil, &api.ImageRepository{})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	result := <-channel
	repo, ok := result.(*api.ImageRepository)
	if !ok {
		t.Errorf("Unexpected result: %#v", result)
	}
	if len(repo.ID) == 0 {
		t.Errorf("Expected repo's ID to be set: %#v", repo)
	}
	if repo.CreationTimestamp.IsZero() {
		t.Error("Unexpected zero CreationTimestamp")
	}
}

func TestCreateRegistryErrorSaving(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("foo")
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Create(nil, &api.ImageRepository{})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	result := <-channel
	status, ok := result.(*kubeapi.Status)
	if !ok {
		t.Errorf("Expected status, got %#v", result)
	}
	if status.Status != kubeapi.StatusFailure || status.Message != "foo" {
		t.Errorf("Expected status=failure, message=foo, got %#v", status)
	}
}

func TestUpdateImageRepositoryBadObject(t *testing.T) {
	storage := REST{}

	channel, err := storage.Update(nil, &api.ImageList{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not an image repository:") == -1 {
		t.Errorf("Expected 'not an image repository' error, got %v", err)
	}
}

func TestUpdateImageRepositoryMissingID(t *testing.T) {
	storage := REST{}

	channel, err := storage.Update(nil, &api.ImageRepository{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "id is unspecified:") == -1 {
		t.Errorf("Expected 'id is unspecified' error, got %v", err)
	}
}

func TestUpdateRegistryErrorSaving(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("foo")
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(nil, &api.ImageRepository{
		JSONBase: kubeapi.JSONBase{ID: "bar"},
	})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	result := <-channel
	status, ok := result.(*kubeapi.Status)
	if !ok {
		t.Errorf("Expected status, got %#v", result)
	}
	if status.Status != kubeapi.StatusFailure || status.Message != "foo" {
		t.Errorf("Expected status=failure, message=foo, got %#v", status)
	}
}

func TestUpdateImageRepositoryOK(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(nil, &api.ImageRepository{
		JSONBase: kubeapi.JSONBase{ID: "bar"},
	})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	result := <-channel
	repo, ok := result.(*api.ImageRepository)
	if !ok {
		t.Errorf("Expected image repository, got %#v", result)
	}
	if repo.ID != "bar" {
		t.Errorf("Unexpected repo returned: %#v", repo)
	}
}

func TestDeleteImageRepository(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Delete(nil, "foo")
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	result := <-channel
	status, ok := result.(*kubeapi.Status)
	if !ok {
		t.Errorf("Expected status, got %#v", result)
	}
	if status.Status != kubeapi.StatusSuccess {
		t.Errorf("Expected status=success, got %#v", status)
	}
}
