package deployerpod

import (
	"fmt"
	"sort"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeployerPodController keeps a deployment's status in sync with the deployer pod
// handling the deployment.
//
// Use the DeployerPodControllerFactory to create this controller.
type DeployerPodController struct {
	// deploymentClient provides access to deployments.
	deploymentClient deploymentClient
	// deployerPodsFor returns all deployer pods for the named deployment.
	deployerPodsFor func(namespace, name string) (*kapi.PodList, error)
	// deletePod deletes a pod.
	deletePod func(namespace, name string) error
}

// transientError is an error which will be retried indefinitely.
type transientError string

func (e transientError) Error() string { return "transient error handling deployer pod: " + string(e) }

// Handle syncs pod's status with any associated deployment.
func (c *DeployerPodController) Handle(pod *kapi.Pod) error {
	// Find the deployment associated with the deployer pod.
	deploymentName := deployutil.DeploymentNameFor(pod)
	if len(deploymentName) == 0 {
		return nil
	}
	// Reject updates to anything but the main deployer pod
	// TODO: Find a way to filter this on the watch side.
	if pod.Name != deployutil.DeployerPodNameForDeployment(deploymentName) {
		return nil
	}

	deployment, err := c.deploymentClient.getDeployment(pod.Namespace, deploymentName)
	// If the deployment for this pod has disappeared, we should clean up this
	// and any other deployer pods, then bail out.
	if err != nil {
		// Some retrieval error occured. Retry.
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("couldn't get deployment %s/%s which owns deployer pod %s/%s", pod.Namespace, deploymentName, pod.Name, pod.Namespace)
		}
		// Find all the deployer pods for the deployment (including this one).
		deployers, err := c.deployerPodsFor(pod.Namespace, deploymentName)
		if err != nil {
			// Retry.
			return fmt.Errorf("couldn't get deployer pods for %s: %v", deployutil.LabelForDeployment(deployment), err)
		}
		// Delete all deployers.
		for _, deployer := range deployers.Items {
			err := c.deletePod(deployer.Namespace, deployer.Name)
			if err != nil {
				if !kerrors.IsNotFound(err) {
					// TODO: Should this fire an event?
					glog.V(2).Infof("Couldn't delete orphaned deployer pod %s/%s: %v", deployer.Namespace, deployer.Name, err)
				}
			} else {
				// TODO: Should this fire an event?
				glog.V(2).Infof("Deleted orphaned deployer pod %s/%s", deployer.Namespace, deployer.Name)
			}
		}
		return nil
	}

	currentStatus := deployutil.DeploymentStatusFor(deployment)
	nextStatus := currentStatus

	switch pod.Status.Phase {
	case kapi.PodRunning:
		nextStatus = deployapi.DeploymentStatusRunning
	case kapi.PodSucceeded:
		// Detect failure based on the container state
		nextStatus = deployapi.DeploymentStatusComplete
		for _, info := range pod.Status.ContainerStatuses {
			if info.State.Termination != nil && info.State.Termination.ExitCode != 0 {
				nextStatus = deployapi.DeploymentStatusFailed
			}
		}
		if nextStatus == deployapi.DeploymentStatusComplete {
			delete(deployment.Annotations, deployapi.DesiredReplicasAnnotation)
		}
	case kapi.PodFailed:
		// if the deployment is already marked Failed, do not attempt clean up again
		if currentStatus != deployapi.DeploymentStatusFailed {
			// clean up will also update the deployment status to Failed
			// failure to clean up will result in retries and
			// the deployment will not be marked Failed
			// Note: this will prevent new deployments from being created for this config
			err := c.cleanupFailedDeployment(deployment)
			if err != nil {
				return transientError(fmt.Sprintf("couldn't clean up failed deployment: %v", err))
			}
		}
	}

	if currentStatus != nextStatus {
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
		if _, err := c.deploymentClient.updateDeployment(deployment.Namespace, deployment); err != nil {
			if kerrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("couldn't update Deployment %s to status %s: %v", deployutil.LabelForDeployment(deployment), nextStatus, err)
		}
		glog.V(4).Infof("Updated Deployment %s status from %s to %s", deployutil.LabelForDeployment(deployment), currentStatus, nextStatus)
	}

	return nil
}

func (c *DeployerPodController) cleanupFailedDeployment(deployment *kapi.ReplicationController) error {
	// Scale down the current failed deployment
	configName := deployutil.DeploymentConfigNameFor(deployment)
	existingDeployments, err := c.deploymentClient.listDeploymentsForConfig(deployment.Namespace, configName)
	if err != nil {
		return fmt.Errorf("couldn't list Deployments for DeploymentConfig %s: %v", configName, err)
	}

	desiredReplicas, ok := deployutil.DeploymentDesiredReplicas(deployment)
	if !ok {
		// if desired replicas could not be found, then log the error
		// and update the failed deployment
		// this cannot be treated as a transient error
		kutil.HandleError(fmt.Errorf("Could not determine desired replicas from %s to reset replicas for last completed deployment", deployutil.LabelForDeployment(deployment)))
	}

	if ok && len(existingDeployments.Items) > 0 {
		sort.Sort(deployutil.DeploymentsByLatestVersionDesc(existingDeployments.Items))
		for index, existing := range existingDeployments.Items {
			// if a newer deployment exists:
			// - set the replicas for the current failed deployment to 0
			// - there is no point in scaling up the last completed deployment
			// since that will be scaled down by the later deployment
			if index == 0 && existing.Name != deployment.Name {
				break
			}

			// the latest completed deployment is the one that needs to be scaled back up
			if deployutil.DeploymentStatusFor(&existing) == deployapi.DeploymentStatusComplete {
				if existing.Spec.Replicas == desiredReplicas {
					break
				}

				// scale back the completed deployment to the target of the failed deployment
				existing.Spec.Replicas = desiredReplicas
				if _, err := c.deploymentClient.updateDeployment(existing.Namespace, &existing); err != nil {
					if kerrors.IsNotFound(err) {
						return nil
					}
					return fmt.Errorf("couldn't update replicas to %d for deployment %s: %v", desiredReplicas, deployutil.LabelForDeployment(&existing), err)
				}
				glog.V(4).Infof("Updated replicas to %d for deployment %s", desiredReplicas, deployutil.LabelForDeployment(&existing))

				break
			}
		}
	}
	// set the replicas for the failed deployment to 0
	// and set the status to Failed
	deployment.Spec.Replicas = 0
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusFailed)
	if _, err := c.deploymentClient.updateDeployment(deployment.Namespace, deployment); err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("couldn't scale down the deployment %s and mark it as failed: %v", deployutil.LabelForDeployment(deployment), err)
	}
	glog.V(4).Infof("Scaled down the deployment %s and marked it as failed", deployutil.LabelForDeployment(deployment))

	return nil
}

// deploymentClient abstracts access to deployments.
type deploymentClient interface {
	getDeployment(namespace, name string) (*kapi.ReplicationController, error)
	updateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
	// listDeploymentsForConfig should return deployments associated with the
	// provided config.
	listDeploymentsForConfig(namespace, configName string) (*kapi.ReplicationControllerList, error)
}

// deploymentClientImpl is a pluggable deploymentControllerDeploymentClient.
type deploymentClientImpl struct {
	getDeploymentFunc            func(namespace, name string) (*kapi.ReplicationController, error)
	updateDeploymentFunc         func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
	listDeploymentsForConfigFunc func(namespace, configName string) (*kapi.ReplicationControllerList, error)
}

func (i *deploymentClientImpl) getDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return i.getDeploymentFunc(namespace, name)
}

func (i *deploymentClientImpl) updateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.updateDeploymentFunc(namespace, deployment)
}

func (i *deploymentClientImpl) listDeploymentsForConfig(namespace, configName string) (*kapi.ReplicationControllerList, error) {
	return i.listDeploymentsForConfigFunc(namespace, configName)
}
