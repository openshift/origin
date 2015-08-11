package deploymentconfig

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigController is responsible for creating a new deployment when:
//
//    1. The config version is > 0 and,
//    2. No existing deployment for that version exists.
//
// The responsibility of constructing a new deployment resource from a config
// is delegated. See util.MakeDeployment for more details. The new deployment
// will have DesiredReplicasAnnotation set to the desired replica count for
// the new deployment based on the replica count of the previous/active
// deployment.
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

func (e fatalError) Error() string {
	return fmt.Sprintf("fatal error handling DeploymentConfig: %s", string(e))
}
func (e transientError) Error() string {
	return "transient error handling DeploymentConfig: " + string(e)
}

// Handle processes config and creates a new deployment if necessary.
func (c *DeploymentConfigController) Handle(config *deployapi.DeploymentConfig) error {
	// Only deploy when the version has advanced past 0.
	if config.LatestVersion == 0 {
		glog.V(5).Infof("Waiting for first version of %s", deployutil.LabelForDeploymentConfig(config))
		return nil
	}

	// Check if any existing inflight deployments (any non-terminal state).
	existingDeployments, err := c.deploymentClient.listDeploymentsForConfig(config.Namespace, config.Name)
	if err != nil {
		return fmt.Errorf("couldn't list Deployments for DeploymentConfig %s: %v", deployutil.LabelForDeploymentConfig(config), err)
	}
	var inflightDeployment *kapi.ReplicationController
	latestDeploymentExists := false
	for _, deployment := range existingDeployments.Items {
		// check if this is the latest deployment
		// we'll return after we've dealt with the multiple-active-deployments case
		if deployutil.DeploymentVersionFor(&deployment) == config.LatestVersion {
			latestDeploymentExists = true
		}

		deploymentStatus := deployutil.DeploymentStatusFor(&deployment)
		switch deploymentStatus {
		case deployapi.DeploymentStatusFailed,
			deployapi.DeploymentStatusComplete:
			// Previous deployment in terminal state - can ignore
			// Ignoring specific deployment states so that any newly introduced
			// deployment state will not be ignored
		default:
			if inflightDeployment == nil {
				inflightDeployment = &deployment
				continue
			}
			var deploymentForCancellation *kapi.ReplicationController
			if deployutil.DeploymentVersionFor(inflightDeployment) < deployutil.DeploymentVersionFor(&deployment) {
				deploymentForCancellation, inflightDeployment = inflightDeployment, &deployment
			} else {
				deploymentForCancellation = &deployment
			}

			deploymentForCancellation.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
			deploymentForCancellation.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentCancelledNewerDeploymentExists
			if _, err = c.deploymentClient.updateDeployment(deploymentForCancellation.Namespace, deploymentForCancellation); err != nil {
				util.HandleError(fmt.Errorf("couldn't cancel Deployment %s: %v", deployutil.LabelForDeployment(deploymentForCancellation), err))
			}
			glog.V(4).Infof("Cancelled Deployment %s for DeploymentConfig %s", deployutil.LabelForDeployment(deploymentForCancellation), deployutil.LabelForDeploymentConfig(config))
		}
	}

	// if the latest deployment exists then nothing else needs to be done
	if latestDeploymentExists {
		return nil
	}

	// check to see if there are inflight deployments
	if inflightDeployment != nil {
		// raise a transientError so that the deployment config can be re-queued
		glog.V(4).Infof("Found previous inflight Deployment for %s - will requeue", deployutil.LabelForDeploymentConfig(config))
		return transientError(fmt.Sprintf("found previous inflight Deployment for %s - requeuing", deployutil.LabelForDeploymentConfig(config)))
	}

	// Try and build a deployment for the config.
	deployment, err := c.makeDeployment(config)
	if err != nil {
		return fatalError(fmt.Sprintf("couldn't make Deployment from (potentially invalid) DeploymentConfig %s: %v", deployutil.LabelForDeploymentConfig(config), err))
	}

	// Compute the desired replicas for the deployment. Use the last completed
	// deployment's current replica count, or the config template if there is no
	// prior completed deployment available.
	desiredReplicas := config.Template.ControllerTemplate.Replicas
	if len(existingDeployments.Items) > 0 {
		sort.Sort(deployutil.DeploymentsByLatestVersionDesc(existingDeployments.Items))
		for _, existing := range existingDeployments.Items {
			if deployutil.DeploymentStatusFor(&existing) == deployapi.DeploymentStatusComplete {
				desiredReplicas = existing.Spec.Replicas
				glog.V(4).Infof("Desired replicas for %s set to %d based on prior completed deployment %s", deployutil.LabelForDeploymentConfig(config), desiredReplicas, existing.Name)
				break
			}
		}
	}
	deployment.Annotations[deployapi.DesiredReplicasAnnotation] = strconv.Itoa(desiredReplicas)

	// Create the deployment.
	if _, err := c.deploymentClient.createDeployment(config.Namespace, deployment); err == nil {
		glog.V(4).Infof("Created Deployment for DeploymentConfig %s", deployutil.LabelForDeploymentConfig(config))
		return nil
	} else {
		// If the deployment was already created, just move on. The cache could be stale, or another
		// process could have already handled this update.
		if errors.IsAlreadyExists(err) {
			glog.V(4).Infof("Deployment already exists for DeploymentConfig %s", deployutil.LabelForDeploymentConfig(config))
			return nil
		}

		// log an event if the deployment could not be created that the user can discover
		c.recorder.Eventf(config, "failedCreate", "Error creating: %v", err)
		return fmt.Errorf("couldn't create Deployment for DeploymentConfig %s: %v", deployutil.LabelForDeploymentConfig(config), err)
	}
}

// deploymentClient abstracts access to deployments.
type deploymentClient interface {
	createDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
	// listDeploymentsForConfig should return deployments associated with the
	// provided config.
	listDeploymentsForConfig(namespace, configName string) (*kapi.ReplicationControllerList, error)
	updateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// deploymentClientImpl is a pluggable deploymentClient.
type deploymentClientImpl struct {
	createDeploymentFunc         func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
	listDeploymentsForConfigFunc func(namespace, configName string) (*kapi.ReplicationControllerList, error)
	updateDeploymentFunc         func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *deploymentClientImpl) createDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.createDeploymentFunc(namespace, deployment)
}

func (i *deploymentClientImpl) listDeploymentsForConfig(namespace, configName string) (*kapi.ReplicationControllerList, error) {
	return i.listDeploymentsForConfigFunc(namespace, configName)
}

func (i *deploymentClientImpl) updateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.updateDeploymentFunc(namespace, deployment)
}
