package execnewpod

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// ExecNewPodController keeps an ExecNewPodAction pod's status in sync with
// its associated deployment.
//
// Use ExecNewPodControllerControllerFactory to create this controller.
type ExecNewPodController struct {
	// deploymentClient provides access to deployments.
	deploymentClient deploymentClient
}

// Handle syncs the action pod's status with its associated deployment.
func (c *ExecNewPodController) Handle(pod *kapi.Pod) error {
	// Verify the assumption that we'll be given only pods correlated to a deployment
	deploymentName, hasDeploymentName := pod.Annotations[deployapi.DeploymentAnnotation]
	if !hasDeploymentName {
		glog.V(2).Infof("Ignoring pod %s; no deployment annotation found", pod.Name)
		return nil
	}

	deployment, deploymentErr := c.deploymentClient.getDeployment(pod.Namespace, deploymentName)
	if deploymentErr != nil {
		return fmt.Errorf("couldn't get deployment %s/%s associated with pod %s", pod.Namespace, deploymentName, pod.Name)
	}

	// Decide if this pod actually represents a lifecycle action
	prePodName, hasPrePodName := deployment.Annotations[deployapi.PreExecNewPodActionPodAnnotation]
	postPodName, hasPostPodName := deployment.Annotations[deployapi.PostExecNewPodActionPodAnnotation]

	if !hasPrePodName || !hasPostPodName {
		glog.V(4).Infof("Ignoring pod %s; no ExecNewPod annotations found on associated deployment %s", pod.Name, labelForDeployment(deployment))
		return nil
	}

	if pod.Name != prePodName && pod.Name != postPodName {
		glog.V(4).Infof("Ignoring pod %s; name doesn't match lifeycle annotations on associated deployment %s", pod.Name, labelForDeployment(deployment))
		return nil
	}

	// Determine whether this is a pre or post action pod so we can update the
	// right annotation on the deployment.
	var phaseAnnotation string
	if hasPrePodName && prePodName == pod.Name {
		phaseAnnotation = deployapi.PreExecNewPodActionPodPhaseAnnotation
	} else if hasPostPodName && postPodName == pod.Name {
		phaseAnnotation = deployapi.PostExecNewPodActionPodPhaseAnnotation
	} else {
		glog.V(2).Infof("Ignoring pod %s; name doesn't match lifeycle annotations on associated deployment %s", pod.Name, labelForDeployment(deployment))
		return nil
	}

	// Update the deployment to hold the latest status of the action pod.
	currentPhase := deployment.Annotations[phaseAnnotation]
	nextPhase := string(pod.Status.Phase)

	if currentPhase != nextPhase {
		deployment.Annotations[phaseAnnotation] = nextPhase
		if _, err := c.deploymentClient.updateDeployment(deployment.Namespace, deployment); err != nil {
			return fmt.Errorf("couldn't update deployment %s annotation %s from %s to %S: %v", labelForDeployment(deployment), phaseAnnotation, currentPhase, nextPhase, err)
		}
		glog.V(2).Infof("Updated deployment %s annotation %s from %s to %s", labelForDeployment(deployment), phaseAnnotation, currentPhase, nextPhase)
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
