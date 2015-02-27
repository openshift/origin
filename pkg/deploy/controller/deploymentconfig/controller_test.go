package deploymentconfig

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// TestHandle_initialOk ensures that an initial config (version 0) doesn't result
// in a new deployment.
func TestHandle_initialOk(t *testing.T) {
	controller := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with name %s", name)
				return nil, nil
			},
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
		},
	}

	err := controller.Handle(deploytest.OkDeploymentConfig(0))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_updateOk ensures that an updated config (version >0) with no
// existing deployment will result in a new deployment.
func TestHandle_updateOk(t *testing.T) {
	deploymentConfig := deploytest.OkDeploymentConfig(1)
	var deployed *kapi.ReplicationController

	controller := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("ReplicationController", name)
			},
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				deployed = deployment
				return deployment, nil
			},
		},
	}

	err := controller.Handle(deploymentConfig)

	if deployed == nil {
		t.Fatalf("expected a deployment")
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_nonfatalLookupError ensures that an API failure to look up the
// existing deployment for an updated config results in a nonfatal error.
func TestHandle_nonfatalLookupError(t *testing.T) {
	configController := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("fatal test error"))
			},
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
		},
	}

	err := configController.Handle(deploytest.OkDeploymentConfig(1))
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, isFatal := err.(fatalError); isFatal {
		t.Fatalf("expected a retryable error, got a fatal error: %v", err)
	}
}

// TestHandle_configAlreadyDeployed ensures that an attempt to create a
// deployment for an updated config for which the deployment was already
// created results in a no-op.
func TestHandle_configAlreadyDeployed(t *testing.T) {
	deploymentConfig := deploytest.OkDeploymentConfig(0)

	controller := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploymentConfig, kapi.Codec)
				return deployment, nil
			},
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to to create deployment: %v", deployment)
				return nil, nil
			},
		},
	}

	err := controller.Handle(deploymentConfig)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_nonfatalCreateError ensures that a failed API attempt to create
// a new deployment for an updated config results in a nonfatal error.
func TestHandle_nonfatalCreateError(t *testing.T) {
	configController := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("ReplicationController", name)
			},
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("test error"))
			},
		},
	}

	err := configController.Handle(deploytest.OkDeploymentConfig(1))
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, isFatal := err.(fatalError); isFatal {
		t.Fatalf("expected a nonfatal error, got a fatal error: %v", err)
	}
}

// TestHandle_fatalError ensures that in internal (not API) failure to make a
// deployment from an updated config results in a fatal error.
func TestHandle_fatalError(t *testing.T) {
	configController := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return nil, fmt.Errorf("couldn't make deployment")
		},
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("ReplicationController", name)
			},
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to create")
				return nil, kerrors.NewInternalError(fmt.Errorf("test error"))
			},
		},
	}

	err := configController.Handle(deploytest.OkDeploymentConfig(1))
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, isFatal := err.(fatalError); !isFatal {
		t.Fatalf("expected a fatal error, got: %v", err)
	}
}
