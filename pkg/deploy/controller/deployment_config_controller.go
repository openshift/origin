package controller

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigController is responsible for creating a deployment when a DeploymentConfig is
// updated with a new LatestVersion. Any deployment created is correlated to a DeploymentConfig
// by setting the DeploymentConfigLabel on the deployment.
//
// Deployments are represented by ReplicationControllers. The DeploymentConfig used to create the
// ReplicationController is encoded and stored in an annotation on the ReplicationController.
type DeploymentConfigController struct {
	// DeploymentClient provides access to Deployments.
	DeploymentClient DeploymentConfigControllerDeploymentClient
	// NextDeploymentConfig blocks until the next DeploymentConfig is available.
	NextDeploymentConfig func() *deployapi.DeploymentConfig
	// Codec is used to encode DeploymentConfigs which are stored on deployments.
	Codec runtime.Codec
	// Stop is an optional channel that controls when the controller exits.
	Stop <-chan struct{}
}

// Run processes DeploymentConfigs one at a time until the Stop channel unblocks.
func (c *DeploymentConfigController) Run() {
	go util.Until(func() {
		err := c.HandleDeploymentConfig(c.NextDeploymentConfig())
		if err != nil {
			glog.Errorf("%v", err)
		}
	}, 0, c.Stop)
}

// HandleDeploymentConfig examines the current state of a DeploymentConfig, and creates a new
// deployment for the config if the following conditions are true:
//
//   1. The config version is greater than 0
//   2. No deployment exists corresponding to  the config's version
//
// If the config can't be processed, an error is returned.
func (c *DeploymentConfigController) HandleDeploymentConfig(config *deployapi.DeploymentConfig) error {
	// Only deploy when the version has advanced past 0.
	if config.LatestVersion == 0 {
		glog.V(5).Infof("Waiting for first version of %s", labelFor(config))
		return nil
	}

	// Find any existing deployment, and return if one already exists.
	if deployment, err := c.DeploymentClient.GetDeployment(config.Namespace, deployutil.LatestDeploymentNameForConfig(config)); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error looking up existing deployment for config %s: %v", labelFor(config), err)
		}
	} else {
		// If there's an existing deployment, nothing needs to be done.
		if deployment != nil {
			return nil
		}
	}

	// Try and build a deployment for the config.
	deployment, err := deployutil.MakeDeployment(config, c.Codec)
	if err != nil {
		return fmt.Errorf("couldn't make deployment from (potentially invalid) config %s: %v", labelFor(config), err)
	}

	// Create the deployment.
	if _, err := c.DeploymentClient.CreateDeployment(config.Namespace, deployment); err == nil {
		glog.V(4).Infof("Created deployment for config %s", labelFor(config))
		return nil
	} else {
		// If the deployment was already created, just move on. The cache could be stale, or another
		// process could have already handled this update.
		if errors.IsAlreadyExists(err) {
			glog.V(4).Infof("Deployment already exists for config %s", labelFor(config))
			return nil
		}
		return fmt.Errorf("couldn't create deployment for config %s: %v", labelFor(config), err)
	}
}

// labelFor builds a string identifier for a DeploymentConfig.
func labelFor(config *deployapi.DeploymentConfig) string {
	return fmt.Sprintf("%s/%s:%d", config.Namespace, config.Name, config.LatestVersion)
}

// DeploymentConfigControllerDeploymentClient abstracts access to deployments.
type DeploymentConfigControllerDeploymentClient interface {
	GetDeployment(namespace, name string) (*kapi.ReplicationController, error)
	CreateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// DeploymentConfigControllerDeploymentClientImpl is a pluggable deploymentConfigControllerDeploymentClient.
type DeploymentConfigControllerDeploymentClientImpl struct {
	GetDeploymentFunc    func(namespace, name string) (*kapi.ReplicationController, error)
	CreateDeploymentFunc func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *DeploymentConfigControllerDeploymentClientImpl) GetDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return i.GetDeploymentFunc(namespace, name)
}

func (i *DeploymentConfigControllerDeploymentClientImpl) CreateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.CreateDeploymentFunc(namespace, deployment)
}
