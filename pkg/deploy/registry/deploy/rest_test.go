package deploy

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	"github.com/openshift/origin/pkg/deploy/registry/test"
)

func TestListDeploymentsError(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Err = fmt.Errorf("test error")

	storage := REST{
		registry: mockRegistry,
	}

	deployments, err := storage.List(kapi.NewDefaultContext(), nil, nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}

	if deployments != nil {
		t.Errorf("Unexpected non-nil deployments list: %#v", deployments)
	}
}

func TestListDeploymentsEmptyList(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Deployments = &api.DeploymentList{
		Items: []api.Deployment{},
	}

	storage := REST{
		registry: mockRegistry,
	}

	deployments, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(deployments.(*api.DeploymentList).Items) != 0 {
		t.Errorf("Unexpected non-zero deployments list: %#v", deployments)
	}
}

func TestListDeploymentsPopulatedList(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Deployments = &api.DeploymentList{
		Items: []api.Deployment{
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

	list, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	deployments := list.(*api.DeploymentList)

	if e, a := 2, len(deployments.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateDeploymentBadObject(t *testing.T) {
	storage := REST{}

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.DeploymentList{})
	if obj != nil {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "not a deployment") == -1 {
		t.Errorf("Expected 'not a deployment' error, got '%v'", err.Error())
	}
}

func TestCreateRegistrySaveError(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Err = fmt.Errorf("test error")
	storage := REST{registry: mockRegistry}

	_, err := storage.Create(kapi.NewDefaultContext(), &api.Deployment{
		ObjectMeta:         kapi.ObjectMeta{Name: "foo"},
		Strategy:           deploytest.OkStrategy(),
		ControllerTemplate: deploytest.OkControllerTemplate(),
	})
	if err != mockRegistry.Err {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
}

func TestCreateDeploymentOk(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	storage := REST{registry: mockRegistry}

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.Deployment{
		ObjectMeta:         kapi.ObjectMeta{Name: "foo"},
		Strategy:           deploytest.OkStrategy(),
		ControllerTemplate: deploytest.OkControllerTemplate(),
	})
	if obj == nil {
		t.Errorf("Expected nil obj, got %v", obj)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	deployment, ok := obj.(*api.Deployment)
	if !ok {
		t.Errorf("Expected deployment type, got: %#v", obj)
	}
	if deployment.Name != "foo" {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestGetDeploymentError(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Err = fmt.Errorf("bad")
	storage := REST{registry: mockRegistry}

	deployment, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if deployment != nil {
		t.Errorf("Unexpected non-nil deployment: %#v", deployment)
	}
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, got %#v", mockRegistry.Err, err)
	}
}

func TestGetDeploymentOk(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Deployment = &api.Deployment{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	}
	storage := REST{registry: mockRegistry}

	deployment, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if deployment == nil {
		t.Error("Unexpected nil deployment")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error", err)
	}
	if deployment.(*api.Deployment).Name != "foo" {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestUpdateDeploymentBadObject(t *testing.T) {
	storage := REST{}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.DeploymentConfig{})
	if obj != nil || created {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "not a deployment:") == -1 {
		t.Errorf("Expected 'not a deployment' error, got %v", err)
	}
}

func TestUpdateDeploymentMissingID(t *testing.T) {
	storage := REST{}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.Deployment{})
	if obj != nil || created {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "name is unspecified:") == -1 {
		t.Errorf("Expected 'name is unspecified' error, got %v", err)
	}
}

func TestUpdateRegistryErrorSaving(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("foo")
	storage := REST{registry: mockRepositoryRegistry}

	_, created, err := storage.Update(kapi.NewDefaultContext(), &api.Deployment{
		ObjectMeta: kapi.ObjectMeta{Name: "bar"},
	})
	if err != mockRepositoryRegistry.Err || created {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
}

func TestUpdateDeploymentOk(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.Deployment{
		ObjectMeta: kapi.ObjectMeta{Name: "bar"},
	})
	if err != nil || created {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	repo, ok := obj.(*api.Deployment)
	if !ok {
		t.Errorf("Expected Deployment, got %#v", obj)
	}
	if repo.Name != "bar" {
		t.Errorf("Unexpected repo returned: %#v", repo)
	}
}

func TestDeleteDeployment(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	storage := REST{registry: mockRegistry}
	obj, err := storage.Delete(kapi.NewDefaultContext(), "foo")
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

func TestCreateDeploymentConflictingNamespace(t *testing.T) {
	storage := REST{}

	obj, err := storage.Create(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.Deployment{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "some-value"},
		Strategy:   deploytest.OkStrategy(),
	})

	if obj != nil {
		t.Error("Expected a nil obj, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func TestUpdateDeploymentConflictingNamespace(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	obj, created, err := storage.Update(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.Deployment{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "some-value"},
		Strategy:   deploytest.OkStrategy(),
	})

	if obj != nil || created {
		t.Error("Expected a nil obj, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func checkExpectedNamespaceError(t *testing.T, err error) {
	expectedError := "Deployment.Namespace does not match the provided context"
	if err == nil {
		t.Errorf("Expected '" + expectedError + "', but we didn't get one")
	} else {
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

}
