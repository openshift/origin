package deployerpod

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

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
}

// Handle syncs pod's status with any associated deployment.
func (c *DeployerPodController) Handle(pod *kapi.Pod) error {
	// Verify the assumption that we'll be given only pods correlated to a deployment
	deploymentName := deployutil.DeploymentNameFor(pod)
	if len(deploymentName) == 0 {
		glog.V(2).Infof("Ignoring pod %s/%s; no Deployment annotation found", pod.Namespace, pod.Name)
		return nil
	}

	deployment, deploymentErr := c.deploymentClient.getDeployment(pod.Namespace, deploymentName)
	if deploymentErr != nil {
		return fmt.Errorf("couldn't get Deployment %s/%s associated with pod %s/%s", pod.Namespace, deploymentName, pod.Name, pod.Namespace)
	}

	currentStatus := deployutil.DeploymentStatusFor(deployment)
	nextStatus := currentStatus

	switch pod.Status.Phase {
	case kapi.PodRunning:
		nextStatus = deployapi.DeploymentStatusRunning
	case kapi.PodSucceeded, kapi.PodFailed:
		nextStatus = deployapi.DeploymentStatusComplete
		// Detect failure based on the container state
		for _, info := range pod.Status.ContainerStatuses {
			if info.State.Termination != nil && info.State.Termination.ExitCode != 0 {
				nextStatus = deployapi.DeploymentStatusFailed
			}
		}
	}

	if currentStatus != nextStatus {
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
		if _, err := c.deploymentClient.updateDeployment(deployment.Namespace, deployment); err != nil {
			return fmt.Errorf("couldn't update Deployment %s/%s to status %s: %v", deployment.Namespace, deployutil.LabelForDeployment(deployment), nextStatus, err)
		}
		glog.V(4).Infof("Updated Deployment %s/%s status from %s to %s", deployment.Namespace, deployutil.LabelForDeployment(deployment), currentStatus, nextStatus)
	}

	return nil
}

// deploymentClient abstracts access to deployments.
type deploymentClient interface {
	getDeployment(namespace, name string) (*kapi.ReplicationController, error)
	updateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// deploymentClientImpl is a pluggable deploymentControllerDeploymentClient.
type deploymentClientImpl struct {
	getDeploymentFunc    func(namespace, name string) (*kapi.ReplicationController, error)
	updateDeploymentFunc func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *deploymentClientImpl) getDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return i.getDeploymentFunc(namespace, name)
}

func (i *deploymentClientImpl) updateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.updateDeploymentFunc(namespace, deployment)
}
