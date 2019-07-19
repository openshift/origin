package proxy

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes/fake"

	oapi "github.com/openshift/openshift-apiserver/pkg/api"
	projectapi "github.com/openshift/openshift-apiserver/pkg/project/apis/project"
)

// mockLister returns the namespaces in the list
type mockLister struct {
	namespaceList *corev1.NamespaceList
}

func (ml *mockLister) List(user user.Info, selector labels.Selector) (*corev1.NamespaceList, error) {
	return ml.namespaceList, nil
}

func TestListProjects(t *testing.T) {
	namespaceList := corev1.NamespaceList{
		Items: []corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			},
		},
	}
	mockClient := fake.NewSimpleClientset(&namespaceList)
	storage := REST{
		client: mockClient.CoreV1().Namespaces(),
		lister: &mockLister{&namespaceList},
	}
	user := &user.DefaultInfo{
		Name:   "test-user",
		UID:    "test-uid",
		Groups: []string{"test-groups"},
	}
	ctx := apirequest.WithUser(apirequest.NewContext(), user)
	response, err := storage.List(ctx, nil)
	if err != nil {
		t.Errorf("%#v should be nil.", err)
	}
	projects := response.(*projectapi.ProjectList)
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

	obj, err := storage.Create(apirequest.NewContext(), &projectapi.ProjectList{}, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
	if obj != nil {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "not a project:") == -1 {
		t.Errorf("Expected 'not an project' error, got %v", err)
	}
}

func TestCreateInvalidProject(t *testing.T) {
	mockClient := &fake.Clientset{}
	storage := NewREST(mockClient.CoreV1().Namespaces(), &mockLister{}, nil, nil)
	_, err := storage.Create(apirequest.NewContext(), &projectapi.Project{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{oapi.OpenShiftDisplayName: "h\t\ni"},
		},
	}, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
	if !errors.IsInvalid(err) {
		t.Errorf("Expected 'invalid' error, got %v", err)
	}
}

func TestCreateProjectOK(t *testing.T) {
	mockClient := &fake.Clientset{}
	storage := NewREST(mockClient.CoreV1().Namespaces(), &mockLister{}, nil, nil)
	_, err := storage.Create(apirequest.NewContext(), &projectapi.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
	}, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
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
	mockClient := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}})
	storage := NewREST(mockClient.CoreV1().Namespaces(), &mockLister{}, nil, nil)
	project, err := storage.Get(apirequest.NewContext(), "foo", &metav1.GetOptions{})
	if project == nil {
		t.Error("Unexpected nil project")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %v", err)
	}
	if project.(*projectapi.Project).Name != "foo" {
		t.Errorf("Unexpected project: %#v", project)
	}
}

func TestDeleteProject(t *testing.T) {
	mockClient := &fake.Clientset{}
	storage := REST{
		client: mockClient.CoreV1().Namespaces(),
	}
	obj, _, err := storage.Delete(apirequest.NewContext(), "foo", nil)
	if obj == nil {
		t.Error("Unexpected nil obj")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	status, ok := obj.(*metav1.Status)
	if !ok {
		t.Errorf("Expected status type, got: %#v", obj)
	}
	if status.Status != metav1.StatusSuccess {
		t.Errorf("Expected status=success, got: %#v", status)
	}
	if len(mockClient.Actions()) != 1 {
		t.Errorf("Expected client action for delete")
	}
	if !mockClient.Actions()[0].Matches("delete", "namespaces") {
		t.Errorf("Expected call to delete-namespace")
	}
}
