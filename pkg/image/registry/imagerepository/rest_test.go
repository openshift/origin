package imagerepository

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/test"
)

func TestGetImageRepositoryError(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("test error")
	storage := REST{registry: mockRepositoryRegistry}

	image, err := storage.Get(kubeapi.NewDefaultContext(), "image1")
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
		TypeMeta:              kubeapi.TypeMeta{ID: "foo"},
		DockerImageRepository: "openshift/ruby-19-centos",
	}
	storage := REST{registry: mockRepositoryRegistry}

	repo, err := storage.Get(kubeapi.NewDefaultContext(), "foo")
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

	imageRepositories, err := storage.List(kubeapi.NewDefaultContext(), nil, nil)
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

	imageRepositories, err := storage.List(kubeapi.NewDefaultContext(), labels.Everything(), labels.Everything())
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
				TypeMeta: kubeapi.TypeMeta{
					ID: "foo",
				},
			},
			{
				TypeMeta: kubeapi.TypeMeta{
					ID: "bar",
				},
			},
		},
	}

	storage := REST{
		registry: mockRepositoryRegistry,
	}

	list, err := storage.List(kubeapi.NewDefaultContext(), labels.Everything(), labels.Everything())
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

	channel, err := storage.Create(kubeapi.NewDefaultContext(), &api.ImageList{})
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

	channel, err := storage.Create(kubeapi.NewDefaultContext(), &api.ImageRepository{})
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

	channel, err := storage.Create(kubeapi.NewDefaultContext(), &api.ImageRepository{})
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

	channel, err := storage.Update(kubeapi.NewDefaultContext(), &api.ImageList{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not an image repository:") == -1 {
		t.Errorf("Expected 'not an image repository' error, got %v", err)
	}
}

func TestUpdateImageRepositoryMissingID(t *testing.T) {
	storage := REST{}

	channel, err := storage.Update(kubeapi.NewDefaultContext(), &api.ImageRepository{})
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

	channel, err := storage.Update(kubeapi.NewDefaultContext(), &api.ImageRepository{
		TypeMeta: kubeapi.TypeMeta{ID: "bar"},
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

	channel, err := storage.Update(kubeapi.NewDefaultContext(), &api.ImageRepository{
		TypeMeta: kubeapi.TypeMeta{ID: "bar"},
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

	channel, err := storage.Delete(kubeapi.NewDefaultContext(), "foo")
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

func TestCreateImageRepositoryConflictingNamespace(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(kubeapi.WithNamespace(kubeapi.NewContext(), "legal-name"), &api.ImageRepository{
		TypeMeta: kubeapi.TypeMeta{ID: "bar", Namespace: "some-value"},
	})

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func TestUpdateImageRepositoryConflictingNamespace(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(kubeapi.WithNamespace(kubeapi.NewContext(), "legal-name"), &api.ImageRepository{
		TypeMeta: kubeapi.TypeMeta{ID: "bar", Namespace: "some-value"},
	})

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func checkExpectedNamespaceError(t *testing.T, err error) {
	expectedError := "ImageRepository.Namespace does not match the provided context"
	if err == nil {
		t.Errorf("Expected '" + expectedError + "', but we didn't get one")
	} else {
		e, ok := err.(kubeclient.APIStatus)
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
