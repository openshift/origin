package controller

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestHandleNewDeploymentConfig(t *testing.T) {
	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with name %s", name)
				return nil, nil
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return deploytest.OkDeploymentConfig(0)
		},
	}

	controller.HandleDeploymentConfig()
}

func TestHandleInitialDeployment(t *testing.T) {
	deploymentConfig := deploytest.OkDeploymentConfig(1)
	var deployed *kapi.ReplicationController

	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("replicationController", name)
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				deployed = deployment
				return deployment, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return deploymentConfig
		},
	}

	controller.HandleDeploymentConfig()

	if deployed == nil {
		t.Fatalf("expected a deployment")
	}
}

func TestHandleConfigChangeLatestAlreadyDeployed(t *testing.T) {
	deploymentConfig := deploytest.OkDeploymentConfig(0)

	controller := &DeploymentConfigController{
		Codec: api.Codec,
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploymentConfig, kapi.Codec)
				return deployment, nil
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to to create deployment: %v", deployment)
				return nil, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return deploymentConfig
		},
	}

	controller.HandleDeploymentConfig()
}

type testDeploymentInterface struct {
	GetDeploymentFunc    func(namespace, name string) (*kapi.ReplicationController, error)
	CreateDeploymentFunc func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *testDeploymentInterface) GetDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return i.GetDeploymentFunc(namespace, name)
}

func (i *testDeploymentInterface) CreateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.CreateDeploymentFunc(namespace, deployment)
}
