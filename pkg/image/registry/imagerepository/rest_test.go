package imagerepository

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/test"
)

func TestGetImageRepositoryError(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("test error")
	storage := REST{
		registry: mockRepositoryRegistry,
	}

	image, err := storage.Get(kapi.NewDefaultContext(), "image1")
	if image != (*api.ImageRepository)(nil) {
		t.Errorf("Unexpected non-nil image: %#v", image)
	}
	if err != mockRepositoryRegistry.Err {
		t.Errorf("Expected %#v, got %#v", mockRepositoryRegistry.Err, err)
	}
}

func TestGetImageRepositoryOK(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.ImageRepository = &api.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "foo"},
		DockerImageRepository: "openshift/ruby-19-centos",
	}
	storage := REST{
		registry: mockRepositoryRegistry,
	}

	repo, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if repo == nil {
		t.Fatalf("Unexpected nil repo: %#v", repo)
	}
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
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

	imageRepositories, err := storage.List(kapi.NewDefaultContext(), nil, nil)
	if err != mockRepositoryRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRepositoryRegistry.Err, err)
	}

	if imageRepositories != (*api.ImageRepositoryList)(nil) {
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

	imageRepositories, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
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
				ObjectMeta: kapi.ObjectMeta{
					Name: "foo",
				},
			},
			{
				ObjectMeta: kapi.ObjectMeta{
					Name: "bar",
				},
			},
		},
	}

	storage := REST{
		registry: mockRepositoryRegistry,
	}

	list, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}

	imageRepositories := list.(*api.ImageRepositoryList)

	if e, a := 2, len(imageRepositories.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateImageRepositoryOK(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	storage := REST{
		registry: mockRepositoryRegistry,
	}

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}

	repo, ok := obj.(*api.ImageRepository)
	if !ok {
		t.Fatalf("Unexpected obj: %#v", obj)
	}
	if len(repo.Name) == 0 {
		t.Errorf("Expected repo's ID to be set: %#v", repo)
	}
	if repo.CreationTimestamp.IsZero() {
		t.Error("Unexpected zero CreationTimestamp")
	}
	if repo.DockerImageRepository != "" {
		t.Errorf("unexpected repository: %#v", repo)
	}
}

func TestCreateRegistryErrorSaving(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("foo")
	storage := REST{
		registry: mockRepositoryRegistry,
	}

	_, err := storage.Create(kapi.NewDefaultContext(), &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})
	if err != mockRepositoryRegistry.Err {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
}

func TestUpdateImageRepositoryMissingID(t *testing.T) {
	storage := REST{}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.ImageRepository{})
	if obj != nil || created {
		t.Fatalf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "name: required value") == -1 {
		t.Errorf("Expected 'id is unspecified' error, got %v", err)
	}
}

func TestUpdateRegistryErrorSaving(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("foo")
	storage := REST{
		registry: mockRepositoryRegistry,
	}

	_, created, err := storage.Update(kapi.NewDefaultContext(), &api.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Name: "bar"},
	})
	if err != mockRepositoryRegistry.Err || created {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
}

func TestUpdateImageRepositoryOK(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	storage := REST{
		registry: mockRepositoryRegistry,
	}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Name: "bar"},
	})
	if err != nil || created {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	repo, ok := obj.(*api.ImageRepository)
	if !ok {
		t.Errorf("Expected image repository, got %#v", obj)
	}
	if repo.Name != "bar" {
		t.Errorf("Unexpected repo returned: %#v", repo)
	}
}

func TestDeleteImageRepository(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	storage := REST{
		registry: mockRepositoryRegistry,
	}

	obj, err := storage.Delete(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	status, ok := obj.(*kapi.Status)
	if !ok {
		t.Errorf("Expected status, got %#v", obj)
	}
	if status.Status != kapi.StatusSuccess {
		t.Errorf("Expected status=success, got %#v", status)
	}
}

func TestCreateImageRepositoryConflictingNamespace(t *testing.T) {
	storage := REST{}

	obj, err := storage.Create(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "some-value"},
	})

	if obj != nil {
		t.Error("Expected a nil obj, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func TestUpdateImageRepositoryConflictingNamespace(t *testing.T) {
	mockRepositoryRegistry := test.NewImageRepositoryRegistry()
	storage := REST{
		registry: mockRepositoryRegistry,
	}

	obj, created, err := storage.Update(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "some-value"},
	})

	if obj != nil || created {
		t.Error("Expected a nil obj, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func checkExpectedNamespaceError(t *testing.T, err error) {
	expectedError := "ImageRepository.Namespace does not match the provided context"
	if err == nil {
		t.Fatalf("Expected '" + expectedError + "', but we didn't get one")
	}
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
