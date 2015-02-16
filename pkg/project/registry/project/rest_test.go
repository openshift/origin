package project

import (
	"fmt"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/project/api"
	"github.com/openshift/origin/pkg/project/registry/test"
)

func TestListProjectsError(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
	mockRegistry.Err = fmt.Errorf("test error")

	storage := REST{
		registry: mockRegistry,
	}

	projects, err := storage.List(nil, nil, nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}

	if projects != nil {
		t.Errorf("Unexpected non-nil projects list: %#v", projects)
	}
}

func TestListProjectsEmptyList(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
	mockRegistry.Projects = &api.ProjectList{
		Items: []api.Project{},
	}

	storage := REST{
		registry: mockRegistry,
	}

	projects, err := storage.List(nil, labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(projects.(*api.ProjectList).Items) != 0 {
		t.Errorf("Unexpected non-zero projects list: %#v", projects)
	}
}

func TestListProjectsPopulatedList(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
	mockRegistry.Projects = &api.ProjectList{
		Items: []api.Project{
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
		registry: mockRegistry,
	}

	list, err := storage.List(nil, labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	projects := list.(*api.ProjectList)

	if e, a := 2, len(projects.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateProjectBadObject(t *testing.T) {
	storage := REST{}

	obj, err := storage.Create(nil, &api.ProjectList{})
	if obj != nil {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "not a project:") == -1 {
		t.Errorf("Expected 'not an project' error, got %v", err)
	}
}

func TestCreateProjectMissingID(t *testing.T) {
	storage := REST{}

	obj, err := storage.Create(nil, &api.Project{})
	if obj != nil {
		t.Errorf("Expected nil obj, got %v", obj)
	}
	if !errors.IsInvalid(err) {
		t.Errorf("Expected 'invalid' error, got %v", err)
	}
}

func TestCreateRegistrySaveError(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
	mockRegistry.Err = fmt.Errorf("test error")
	storage := REST{registry: mockRegistry}

	_, err := storage.Create(nil, &api.Project{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	})
	if err != mockRegistry.Err {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
}

func TestCreateProjectOK(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
	storage := REST{registry: mockRegistry}

	obj, err := storage.Create(nil, &api.Project{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	})
	if obj == nil {
		t.Errorf("Expected nil obj, got %v", obj)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	project, ok := obj.(*api.Project)
	if !ok {
		t.Errorf("Expected project type, got: %#v", obj)
	}
	if project.Name != "foo" {
		t.Errorf("Unexpected project: %#v", project)
	}
}

func TestGetProjectError(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
	mockRegistry.Err = fmt.Errorf("bad")
	storage := REST{registry: mockRegistry}

	project, err := storage.Get(nil, "foo")
	if project != nil {
		t.Errorf("Unexpected non-nil project: %#v", project)
	}
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, got %#v", mockRegistry.Err, err)
	}
}

func TestGetProjectOK(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
	mockRegistry.Project = &api.Project{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	}
	storage := REST{registry: mockRegistry}

	project, err := storage.Get(nil, "foo")
	if project == nil {
		t.Error("Unexpected nil project")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error", err)
	}
	if project.(*api.Project).Name != "foo" {
		t.Errorf("Unexpected project: %#v", project)
	}
}

func TestDeleteProject(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
	storage := REST{registry: mockRegistry}
	obj, err := storage.Delete(nil, "foo")
	if obj == nil {
		t.Error("Unexpected nil obj")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	status, ok := obj.(*kapi.Status)
	if !ok {
		t.Errorf("Expected status type, got: %#v", obj)
	}
	if status.Status != kapi.StatusSuccess {
		t.Errorf("Expected status=success, got: %#v", status)
	}
}
