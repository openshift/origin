package deploymentconfig

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"

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
	recorder       record.EventRecorder
}

// fatalError is an error which can't be retried.
type fatalError string

// transientError is an error which should always be retried (indefinitely).
type transientError string

func (e fatalError) Error() string { return "fatal error handling deploymentConfig: " + string(e) }
func (e transientError) Error() string {
	return "transient error handling deploymentConfig: " + string(e)
}

// Handle processes config and creates a new deployment if necessary.
func (c *DeploymentConfigController) Handle(config *deployapi.DeploymentConfig) error {
	// Only deploy when the version has advanced past 0.
	if config.LatestVersion == 0 {
		glog.V(5).Infof("Waiting for first version of %s", labelFor(config))
		return nil
	}

	// Check if the latest deployment already exists
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

	// Check if any previous deployment is still running (any non-terminal state).
	if deployments, err := c.deploymentClient.listDeployments(config.Namespace); err != nil {
		return fmt.Errorf("couldn't list deployments for config %s: %v", labelFor(config), err)
	} else {
		for _, deployment := range deployments.Items {
			// consider only the deployments for this deployment config
			if config.Name != deployment.Annotations[deployapi.DeploymentConfigAnnotation] {
				continue
			}
			deploymentStatus := deployapi.DeploymentStatus(deployment.Annotations[deployapi.DeploymentStatusAnnotation])
			switch deploymentStatus {
			case deployapi.DeploymentStatusFailed,
				deployapi.DeploymentStatusComplete:
				// Previous deployment in terminal state - can ignore
				// Ignoring specific deployment states so that any new 
				// deployment state will not be ignored
			default:
				glog.V(4).Infof("Found previous deployment %s/%s (status %s) - will requeue", deployment.Namespace, deployment.Name, deploymentStatus)
				return transientError(fmt.Sprintf("found previous deployment (state: %s) for %s - requeuing", deploymentStatus, labelFor(config)))
			}
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
			c.recorder.Eventf(config, "alreadyExists", "Deployment already exists for config: %s", labelFor(config))
			glog.V(4).Infof("Deployment already exists for config %s", labelFor(config))
			return nil
		}

		// log an event if the deployment could not be created that the user can discover
		c.recorder.Eventf(config, "failedCreate", "Error creating: %v", err)
		return fmt.Errorf("couldn't create deployment for config %s: %v", labelFor(config), err)
	}
}

// labelFor builds a string identifier for a DeploymentConfig.
func labelFor(config *deployapi.DeploymentConfig) string {
	return fmt.Sprintf("%s/%s:%d", config.Namespace, config.Name, config.LatestVersion)
}

// deploymentClient abstracts access to deployments.
type deploymentClient interface {
	listDeployments(namespace string) (*kapi.ReplicationControllerList, error)
	getDeployment(namespace, name string) (*kapi.ReplicationController, error)
	createDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// deploymentClientImpl is a pluggable deploymentClient.
type deploymentClientImpl struct {
	listDeploymentsFunc  func(namespace string) (*kapi.ReplicationControllerList, error)
	getDeploymentFunc    func(namespace, name string) (*kapi.ReplicationController, error)
	createDeploymentFunc func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *deploymentClientImpl) listDeployments(namespace string) (*kapi.ReplicationControllerList, error) {
	return i.listDeploymentsFunc(namespace)
}

func (i *deploymentClientImpl) getDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return i.getDeploymentFunc(namespace, name)
}

func (i *deploymentClientImpl) createDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.createDeploymentFunc(namespace, deployment)
}
