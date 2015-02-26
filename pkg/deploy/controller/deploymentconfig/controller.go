package deploymentconfig

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigController is responsible for creating a new deployment when:
//
//    1. The config version is > 0 and,
//    2. No existing deployment for that version exists.
//
// The responsibility of constructing a new deployment resource from a config
// is delegated. See util.MakeDeployment for more details.
//
// Use the DeploymentConfigControllerFactory to create this controller.
type DeploymentConfigController struct {
	// deploymentClient provides access to deployments.
	deploymentClient deploymentClient
	// makeDeployment knows how to make a deployment from a config.
	makeDeployment func(*deployapi.DeploymentConfig) (*kapi.ReplicationController, error)
}

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string { return "fatal error handling deploymentConfig: " + string(e) }

// Handle processes config and creates a new deployment if necessary.
func (c *DeploymentConfigController) Handle(config *deployapi.DeploymentConfig) error {
	// Only deploy when the version has advanced past 0.
	if config.LatestVersion == 0 {
		glog.V(5).Infof("Waiting for first version of %s", labelFor(config))
		return nil
	}

	// Find any existing deployment, and return if one already exists.
	if deployment, err := c.deploymentClient.getDeployment(config.Namespace, deployutil.LatestDeploymentNameForConfig(config)); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("couldn't get deployment for config %s: %v", labelFor(config), err)
		}
	} else {
		// If there's an existing deployment, nothing needs to be done.
		if deployment != nil {
			return nil
		}
	}

	// Try and build a deployment for the config.
	deployment, err := c.makeDeployment(config)
	if err != nil {
		return fatalError(fmt.Sprintf("couldn't make deployment from (potentially invalid) config %s: %v", labelFor(config), err))
	}

	// Create the deployment.
	if _, err := c.deploymentClient.createDeployment(config.Namespace, deployment); err == nil {
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

// deploymentClient abstracts access to deployments.
type deploymentClient interface {
	getDeployment(namespace, name string) (*kapi.ReplicationController, error)
	createDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// deploymentClientImpl is a pluggable deploymentClient.
type deploymentClientImpl struct {
	getDeploymentFunc    func(namespace, name string) (*kapi.ReplicationController, error)
	createDeploymentFunc func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *deploymentClientImpl) getDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return i.getDeploymentFunc(namespace, name)
}

func (i *deploymentClientImpl) createDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.createDeploymentFunc(namespace, deployment)
}
