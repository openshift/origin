package project

import (
	"fmt"
	"strings"
	"testing"
	"time"

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

	channel, err := storage.Create(nil, &api.ProjectList{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not a project:") == -1 {
		t.Errorf("Expected 'not an project' error, got %v", err)
	}
}

func TestCreateProjectMissingID(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(nil, &api.Project{})
	if channel != nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if !errors.IsInvalid(err) {
		t.Errorf("Expected 'invalid' error, got %v", err)
	}
}

func TestCreateRegistrySaveError(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
	mockRegistry.Err = fmt.Errorf("test error")
	storage := REST{registry: mockRegistry}

	channel, err := storage.Create(nil, &api.Project{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	})
	if channel == nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		status, ok := result.Object.(*kapi.Status)
		if !ok {
			t.Errorf("Expected status type, got: %#v", result)
		}
		if status.Status != kapi.StatusFailure || status.Message != "test error" {
			t.Errorf("Expected failure status, got %#v", status)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	}
}

func TestCreateProjectOK(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
	storage := REST{registry: mockRegistry}

	channel, err := storage.Create(nil, &api.Project{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	})
	if channel == nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		project, ok := result.Object.(*api.Project)
		if !ok {
			t.Errorf("Expected project type, got: %#v", result)
		}
		if project.Name != "foo" {
			t.Errorf("Unexpected project: %#v", project)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
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

func TestUpdateProject(t *testing.T) {
	storage := REST{}
	channel, err := storage.Update(nil, &api.Project{})
	if channel != nil {
		t.Errorf("Unexpected non-nil channel: %#v", channel)
	}
	if err == nil {
		t.Fatal("Unexpected nil err")
	}
	if strings.Index(err.Error(), "Projects may not be changed.") == -1 {
		t.Errorf("Expected 'may not be changed' error, got: %#v", err)
	}
}

func TestDeleteProject(t *testing.T) {
	mockRegistry := test.NewProjectRegistry()
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
		status, ok := result.Object.(*kapi.Status)
		if !ok {
			t.Errorf("Expected status type, got: %#v", result)
		}
		if status.Status != kapi.StatusSuccess {
			t.Errorf("Expected status=success, got: %#v", status)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	}
}
