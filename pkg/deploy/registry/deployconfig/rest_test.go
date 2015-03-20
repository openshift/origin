package deployconfig

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

func TestListDeploymentConfigsError(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
	mockRegistry.Err = fmt.Errorf("test error")

	storage := REST{
		registry: mockRegistry,
	}

	deploymentConfigs, err := storage.List(kapi.NewDefaultContext(), nil, nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}

	if deploymentConfigs != nil {
		t.Errorf("Unexpected non-nil deploymentConfigs list: %#v", deploymentConfigs)
	}
}

func TestListDeploymentConfigsEmptyList(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
	mockRegistry.DeploymentConfigs = &api.DeploymentConfigList{
		Items: []api.DeploymentConfig{},
	}

	storage := REST{
		registry: mockRegistry,
	}

	deploymentConfigs, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(deploymentConfigs.(*api.DeploymentConfigList).Items) != 0 {
		t.Errorf("Unexpected non-zero deploymentConfigs list: %#v", deploymentConfigs)
	}
}

func TestListDeploymentConfigsPopulatedList(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
	mockRegistry.DeploymentConfigs = &api.DeploymentConfigList{
		Items: []api.DeploymentConfig{
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

	deploymentConfigs := list.(*api.DeploymentConfigList)

	if e, a := 2, len(deploymentConfigs.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateDeploymentConfigBadObject(t *testing.T) {
	storage := REST{}

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.DeploymentList{})
	if obj != nil {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "not a deploymentConfig") == -1 {
		t.Errorf("Expected 'not a deploymentConfig' error, got '%v'", err.Error())
	}
}

func TestCreateRegistrySaveError(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
	mockRegistry.Err = fmt.Errorf("test error")
	storage := REST{registry: mockRegistry}

	_, err := storage.Create(kapi.NewDefaultContext(), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
		Template: api.DeploymentTemplate{
			Strategy:           deploytest.OkStrategy(),
			ControllerTemplate: deploytest.OkControllerTemplate(),
		},
	})
	if err != mockRegistry.Err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateDeploymentConfigOK(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
	storage := REST{registry: mockRegistry}

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
		Template: api.DeploymentTemplate{
			Strategy:           deploytest.OkStrategy(),
			ControllerTemplate: deploytest.OkControllerTemplate(),
		},
	})
	if obj == nil {
		t.Errorf("Expected nil obj, got %v", obj)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	deploymentConfig, ok := obj.(*api.DeploymentConfig)
	if !ok {
		t.Errorf("Expected deploymentConfig type, got: %#v", obj)
	}
	if deploymentConfig.Name != "foo" {
		t.Errorf("Unexpected deploymentConfig: %#v", deploymentConfig)
	}
}

func TestGetDeploymentConfigError(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
	mockRegistry.Err = fmt.Errorf("bad")
	storage := REST{registry: mockRegistry}

	deploymentConfig, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if deploymentConfig != nil {
		t.Errorf("Unexpected non-nil deploymentConfig: %#v", deploymentConfig)
	}
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, got %#v", mockRegistry.Err, err)
	}
}

func TestGetDeploymentConfigOK(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
	mockRegistry.DeploymentConfig = &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	}
	storage := REST{registry: mockRegistry}

	deploymentConfig, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if deploymentConfig == nil {
		t.Error("Unexpected nil deploymentConfig")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error", err)
	}
	if deploymentConfig.(*api.DeploymentConfig).Name != "foo" {
		t.Errorf("Unexpected deploymentConfig: %#v", deploymentConfig)
	}
}

func TestUpdateDeploymentConfigBadObject(t *testing.T) {
	storage := REST{}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.DeploymentList{})
	if obj != nil || created {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "not a deploymentConfig:") == -1 {
		t.Errorf("Expected 'not a deploymentConfig' error, got %v", err)
	}
}

func TestUpdateDeploymentConfigMissingID(t *testing.T) {
	storage := REST{}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.DeploymentConfig{})
	if obj != nil || created {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "id is unspecified:") == -1 {
		t.Errorf("Expected 'id is unspecified' error, got %v", err)
	}
}

func TestUpdateRegistryErrorSaving(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentConfigRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("foo")
	storage := REST{registry: mockRepositoryRegistry}

	_, _, err := storage.Update(kapi.NewDefaultContext(), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "bar"},
	})
	if err != mockRepositoryRegistry.Err {
		t.Errorf("Unexpected error: %#v", err)
	}
}

func TestUpdateDeploymentConfigOK(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentConfigRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "bar"},
	})
	if err != nil || created {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	repo, ok := obj.(*api.DeploymentConfig)
	if !ok {
		t.Errorf("Expected DeploymentConfig, got %#v", obj)
	}
	if repo.Name != "bar" {
		t.Errorf("Unexpected repo returned: %#v", repo)
	}
}

func TestDeleteDeploymentConfig(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
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

func TestCreateDeploymentConfigConflictingNamespace(t *testing.T) {
	storage := REST{}

	obj, err := storage.Create(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "some-value"},
	})

	if obj != nil {
		t.Error("Expected a nil obj, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func TestUpdateDeploymentConfigConflictingNamespace(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentConfigRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	obj, created, err := storage.Update(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "some-value"},
	})

	if obj != nil || created {
		t.Error("Expected a nil obj, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func checkExpectedNamespaceError(t *testing.T, err error) {
	expectedError := "DeploymentConfig.Namespace does not match the provided context"
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
