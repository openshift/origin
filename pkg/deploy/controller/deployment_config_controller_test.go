package controller

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	api "github.com/openshift/origin/pkg/api/latest"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestHandleNewDeploymentConfig(t *testing.T) {
	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentConfigControllerDeploymentClientImpl{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with name %s", name)
				return nil, nil
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
		},
	}

	err := controller.HandleDeploymentConfig(deploytest.OkDeploymentConfig(0))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleUpdatedDeploymentConfigOk(t *testing.T) {
	deploymentConfig := deploytest.OkDeploymentConfig(1)
	var deployed *kapi.ReplicationController

	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentConfigControllerDeploymentClientImpl{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("ReplicationController", name)
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				deployed = deployment
				return deployment, nil
			},
		},
	}

	err := controller.HandleDeploymentConfig(deploymentConfig)

	if deployed == nil {
		t.Fatalf("expected a deployment")
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleUpdatedDeploymentConfigLookupFailure(t *testing.T) {
	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentConfigControllerDeploymentClientImpl{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("test error"))
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
		},
	}

	err := controller.HandleDeploymentConfig(deploytest.OkDeploymentConfig(1))

	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestHandleUpdatedDeploymentConfigAlreadyDeployed(t *testing.T) {
	deploymentConfig := deploytest.OkDeploymentConfig(0)

	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentConfigControllerDeploymentClientImpl{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploymentConfig, kapi.Codec)
				return deployment, nil
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to to create deployment: %v", deployment)
				return nil, nil
			},
		},
	}

	err := controller.HandleDeploymentConfig(deploymentConfig)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleUpdatedDeploymentConfigError(t *testing.T) {
	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentConfigControllerDeploymentClientImpl{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("ReplicationController", name)
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("test error"))
			},
		},
	}

	err := controller.HandleDeploymentConfig(deploytest.OkDeploymentConfig(1))

	if err == nil {
		t.Fatalf("expected error")
	}
}
