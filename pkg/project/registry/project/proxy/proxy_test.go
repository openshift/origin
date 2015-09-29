package proxy

import (
	//  "fmt"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

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
	mockClient := testclient.NewSimpleFake(&namespaceList)
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

	obj, err := storage.Create(kapi.NewContext(), &api.ProjectList{})
	if obj != nil {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "not a project:") == -1 {
		t.Errorf("Expected 'not an project' error, got %v", err)
	}
}

func TestCreateInvalidProject(t *testing.T) {
	mockClient := &testclient.Fake{}
	storage := NewREST(mockClient.Namespaces(), &mockLister{})
	_, err := storage.Create(nil, &api.Project{
		ObjectMeta: kapi.ObjectMeta{
			Annotations: map[string]string{"openshift.io/display-name": "h\t\ni"},
		},
	})
	if !errors.IsInvalid(err) {
		t.Errorf("Expected 'invalid' error, got %v", err)
	}
}

func TestCreateProjectOK(t *testing.T) {
	mockClient := &testclient.Fake{}
	storage := NewREST(mockClient.Namespaces(), &mockLister{})
	_, err := storage.Create(kapi.NewContext(), &api.Project{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	if len(mockClient.Actions()) != 1 {
		t.Errorf("Expected client action for create")
	}
	if !mockClient.Actions()[0].Matches("create", "namespaces") {
		t.Errorf("Expected call to create-namespace")
	}
}

func TestGetProjectOK(t *testing.T) {
	mockClient := testclient.NewSimpleFake(&kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})
	storage := NewREST(mockClient.Namespaces(), &mockLister{})
	project, err := storage.Get(kapi.NewContext(), "foo")
	if project == nil {
		t.Error("Unexpected nil project")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %v", err)
	}
	if project.(*api.Project).Name != "foo" {
		t.Errorf("Unexpected project: %#v", project)
	}
}

func TestDeleteProject(t *testing.T) {
	mockClient := &testclient.Fake{}
	storage := REST{
		client: mockClient.Namespaces(),
	}
	obj, err := storage.Delete(kapi.NewContext(), "foo")
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
	if len(mockClient.Actions()) != 1 {
		t.Errorf("Expected client action for delete")
	}
	if !mockClient.Actions()[0].Matches("delete", "namespaces") {
		t.Errorf("Expected call to delete-namespace")
	}
}
