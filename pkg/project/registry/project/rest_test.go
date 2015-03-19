package project

import (
	//	"fmt"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/project/api"
)

// mockLister returns the namespaces in the list
type mockLister struct {
	namespaceList *kapi.NamespaceList
}

func (ml *mockLister) List(user user.Info) (*kapi.NamespaceList, error) {
	return ml.namespaceList, nil
}

func TestListProjects(t *testing.T) {
	namespaceList := kapi.NamespaceList{
		Items: []kapi.Namespace{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
			},
		},
	}
	mockClient := &kclient.Fake{
		NamespacesList: namespaceList,
	}
	storage := REST{
		client: mockClient.Namespaces(),
		lister: &mockLister{&namespaceList},
	}
	user := &user.DefaultInfo{
		Name:   "test-user",
		UID:    "test-uid",
		Groups: []string{"test-groups"},
	}
	ctx := kapi.WithUser(kapi.NewContext(), user)
	response, err := storage.List(ctx, labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("%#v should be nil.", err)
	}
	projects := response.(*api.ProjectList)
	if len(projects.Items) != 1 {
		t.Errorf("%#v projects.Items should have len 1.", projects.Items)
	}
	responseProject := projects.Items[0]
	if e, r := responseProject.Name, "foo"; e != r {
		t.Errorf("%#v != %#v.", e, r)
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

func TestCreateProjectOK(t *testing.T) {
	mockClient := &kclient.Fake{}
	storage := REST{
		client: mockClient.Namespaces(),
	}
	_, err := storage.Create(nil, &api.Project{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	if len(mockClient.Actions) != 1 {
		t.Errorf("Expected client action for create")
	}
	if mockClient.Actions[0].Action != "create-namespace" {
		t.Errorf("Expected call to create-namespace")
	}

	/*
		TODO: Need upstream change to fake_namespaces so create returns the object passed in on client and not nil object
		project, ok := obj.(*api.Project)
		if !ok {
			t.Errorf("Expected project type, got: %#v", obj)
		}
		if project.Name != "foo" {
			t.Errorf("Unexpected project: %#v", project)
		}*/
}

func TestGetProjectError(t *testing.T) {
	// TODO: Need upstream change to fake_namespaces so get returns the error on Fake
	/*
		mockRegistry := test.NewProjectRegistry()
		mockRegistry.Err = fmt.Errorf("bad")
		storage := REST{registry: mockRegistry}

		project, err := storage.Get(nil, "foo")
		if project != nil {
			t.Errorf("Unexpected non-nil project: %#v", project)
		}
		if err != mockRegistry.Err {
			t.Errorf("Expected %#v, got %#v", mockRegistry.Err, err)
		}*/
}

func TestGetProjectOK(t *testing.T) {
	mockClient := &kclient.Fake{}
	storage := REST{client: mockClient.Namespaces()}
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
	mockClient := &kclient.Fake{}
	storage := REST{
		client: mockClient.Namespaces(),
	}
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
	if len(mockClient.Actions) != 1 {
		t.Errorf("Expected client action for delete")
	}
	if mockClient.Actions[0].Action != "delete-namespace" {
		t.Errorf("Expected call to delete-namespace")
	}
}
