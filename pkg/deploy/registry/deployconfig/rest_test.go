package deployconfig

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
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

	deploymentConfigs, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), labels.Everything())
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

	list, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), labels.Everything())
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

	channel, err := storage.Create(kapi.NewDefaultContext(), &api.DeploymentList{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not a deploymentConfig") == -1 {
		t.Errorf("Expected 'not a deploymentConfig' error, got '%v'", err.Error())
	}
}

func TestCreateRegistrySaveError(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
	mockRegistry.Err = fmt.Errorf("test error")
	storage := REST{registry: mockRegistry}

	channel, err := storage.Create(kapi.NewDefaultContext(), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
		Template: api.DeploymentTemplate{
			Strategy:           deploytest.OkStrategy(),
			ControllerTemplate: deploytest.OkControllerTemplate(),
		},
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

func TestCreateDeploymentConfigOK(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
	storage := REST{registry: mockRegistry}

	channel, err := storage.Create(kapi.NewDefaultContext(), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
		Template: api.DeploymentTemplate{
			Strategy:           deploytest.OkStrategy(),
			ControllerTemplate: deploytest.OkControllerTemplate(),
		},
	})
	if channel == nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		deploymentConfig, ok := result.Object.(*api.DeploymentConfig)
		if !ok {
			t.Errorf("Expected deploymentConfig type, got: %#v", result)
		}
		if deploymentConfig.Name != "foo" {
			t.Errorf("Unexpected deploymentConfig: %#v", deploymentConfig)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
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

	channel, err := storage.Update(kapi.NewDefaultContext(), &api.DeploymentList{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not a deploymentConfig:") == -1 {
		t.Errorf("Expected 'not a deploymentConfig' error, got %v", err)
	}
}

func TestUpdateDeploymentConfigMissingID(t *testing.T) {
	storage := REST{}

	channel, err := storage.Update(kapi.NewDefaultContext(), &api.DeploymentConfig{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "id is unspecified:") == -1 {
		t.Errorf("Expected 'id is unspecified' error, got %v", err)
	}
}

func TestUpdateRegistryErrorSaving(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentConfigRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("foo")
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(kapi.NewDefaultContext(), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "bar"},
	})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	result := <-channel
	status, ok := result.Object.(*kapi.Status)
	if !ok {
		t.Errorf("Expected status, got %#v", result)
	}
	if status.Status != kapi.StatusFailure || status.Message != "foo" {
		t.Errorf("Expected status=failure, message=foo, got %#v", status)
	}
}

func TestUpdateDeploymentConfigOK(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentConfigRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(kapi.NewDefaultContext(), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "bar"},
	})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	result := <-channel
	repo, ok := result.Object.(*api.DeploymentConfig)
	if !ok {
		t.Errorf("Expected DeploymentConfig, got %#v", result)
	}
	if repo.Name != "bar" {
		t.Errorf("Unexpected repo returned: %#v", repo)
	}
}

func TestDeleteDeploymentConfig(t *testing.T) {
	mockRegistry := test.NewDeploymentConfigRegistry()
	storage := REST{registry: mockRegistry}
	channel, err := storage.Delete(kapi.NewDefaultContext(), "foo")
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

func TestCreateDeploymentConfigConflictingNamespace(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "some-value"},
	})

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func TestUpdateDeploymentConfigConflictingNamespace(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentConfigRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "some-value"},
	})

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
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
