package controller

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"

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
	// DeploymentInterface provides access to Deployments.
	DeploymentInterface dccDeploymentInterface
	// NextDeploymentConfig blocks until the next DeploymentConfig is available.
	NextDeploymentConfig func() *deployapi.DeploymentConfig
	// Codec is used to encode DeploymentConfigs which are stored on deployments.
	Codec runtime.Codec
	// Stop is an optional channel that controls when the controller exits.
	Stop <-chan struct{}
}

// dccDeploymentInterface is a small private interface for dealing with Deployments.
type dccDeploymentInterface interface {
	GetDeployment(namespace, name string) (*kapi.ReplicationController, error)
	CreateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// Process DeploymentConfig events one at a time.
func (c *DeploymentConfigController) Run() {
	go util.Until(c.HandleDeploymentConfig, 0, c.Stop)
}

// Process a single DeploymentConfig event.
func (c *DeploymentConfigController) HandleDeploymentConfig() {
	config := c.NextDeploymentConfig()
	deploy, err := c.shouldDeploy(config)
	if err != nil {
		util.HandleError(fmt.Errorf("unable to decide whether to redeploy %s: %v", labelFor(config), err))
		return
	}
	if !deploy {
		return
	}

	deployment, err := deployutil.MakeDeployment(config, c.Codec)
	if err != nil {
		util.HandleError(fmt.Errorf("unable to create deployment for %s: %v", labelFor(config), err))
		return
	}

	glog.V(4).Infof("Deploying %s", labelFor(config))
	if _, deployErr := c.DeploymentInterface.CreateDeployment(config.Namespace, deployment); deployErr != nil {
		util.HandleError(fmt.Errorf("unable to create deployment %s: %v", labelFor(config), err))
		return
	}
}

// shouldDeploy returns true if the DeploymentConfig should have a new Deployment created.
func (c *DeploymentConfigController) shouldDeploy(config *deployapi.DeploymentConfig) (bool, error) {
	if config.LatestVersion == 0 {
		glog.V(5).Infof("Waiting for first version of %s", labelFor(config))
		return false, nil
	}

	latestDeploymentID := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := c.DeploymentInterface.GetDeployment(config.Namespace, latestDeploymentID)

	if err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	glog.V(5).Infof("Found deployment for %s - %s:%s", labelFor(config), deployment.UID, deployment.ResourceVersion)
	return false, nil
}

func labelFor(config *deployapi.DeploymentConfig) string {
	return fmt.Sprintf("%s/%s:%d", config.Namespace, config.Name, config.LatestVersion)
}
