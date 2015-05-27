package deploymentcancellation

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentCancellationController monitors for deployments that have been
// cancelled by the user and sets the ActiveDeadlineSeconds to 0 on the
// corresponding deployer pods.
//
// Use the DeploymentCancellationControllerFactory to create this controller.
type DeploymentCancellationController struct {
	// podClient provides access to pods.
	podClient podClient
	recorder  record.EventRecorder
}

// Handle processes deployment and either creates a deployer pod or responds
// to a terminal deployment status.
func (c *DeploymentCancellationController) Handle(deployment *kapi.ReplicationController) error {
	currentStatus := deployutil.DeploymentStatusFor(deployment)

	switch currentStatus {
	case deployapi.DeploymentStatusNew,
		deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning:

		if !deployutil.IsDeploymentCancelled(deployment) {
			return nil
		}

		deployerPod, err := c.podClient.getPod(deployment.Namespace, deployutil.DeployerPodNameFor(deployment))
		if err != nil {
			return fmt.Errorf("couldn't fetch deployer pod for %s/%s: %v", deployment.Namespace, deployutil.LabelForDeployment(deployment), err)
		}

		// set the ActiveDeadlineSeconds on the deployer pod to 0, if not already set
		zeroDelay := int64(0)
		if deployerPod.Spec.ActiveDeadlineSeconds == nil || *deployerPod.Spec.ActiveDeadlineSeconds != zeroDelay {
			deployerPod.Spec.ActiveDeadlineSeconds = &zeroDelay
			if _, err := c.podClient.updatePod(deployerPod.Namespace, deployerPod); err != nil {
				c.recorder.Eventf(deployment, "failedCancellation", "Error updating ActiveDeadlineSeconds to 0 on deployer pod for Deployment %s/%s: %v", deployment.Namespace, deployutil.LabelForDeployment(deployment), err)
				return fmt.Errorf("couldn't update ActiveDeadlineSeconds to 0 on deployer pod for Deployment %s/%s: %v", deployment.Namespace, deployutil.LabelForDeployment(deployment), err)
			}
			glog.V(4).Infof("Updated ActiveDeadlineSeconds to 0 on deployer pod for Deployment %s/%s", deployment.Namespace, deployutil.LabelForDeployment(deployment))
		}
	}

	return nil
}

// podClient abstracts access to pods.
type podClient interface {
	getPod(namespace, name string) (*kapi.Pod, error)
	updatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
}

// podClientImpl is a pluggable podClient.
type podClientImpl struct {
	getPodFunc    func(namespace, name string) (*kapi.Pod, error)
	updatePodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
}

func (i *podClientImpl) getPod(namespace, name string) (*kapi.Pod, error) {
	return i.getPodFunc(namespace, name)
}

func (i *podClientImpl) updatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.updatePodFunc(namespace, pod)
}
