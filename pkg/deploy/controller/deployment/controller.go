package deployment

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentController starts a deployment by creating a deployer pod which
// implements a deployment strategy. The status of the deployment will follow
// the status of the deployer pod. The deployer pod is correlated to the
// deployment with annotations.
//
// When the deployment enters a terminal status:
//
//   1. If the deployment finished normally, the deployer pod is deleted.
//   2. If the deployment failed, the deployer pod is not deleted.
//
// Use the DeploymentControllerFactory to create this controller.
type DeploymentController struct {
	// serviceAccount to create deployment pods with
	serviceAccount string
	// deploymentClient provides access to deployments.
	deploymentClient deploymentClient
	// podClient provides access to pods.
	podClient podClient
	// makeContainer knows how to make a container appropriate to execute a deployment strategy.
	makeContainer func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error)
	// decodeConfig knows how to decode the deploymentConfig from a deployment's annotations.
	decodeConfig func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error)
	recorder     record.EventRecorder
}

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string { return "fatal error handling deployment: " + string(e) }

// Handle processes deployment and either creates a deployer pod or responds
// to a terminal deployment status.
func (c *DeploymentController) Handle(deployment *kapi.ReplicationController) error {
	currentStatus := deployutil.DeploymentStatusFor(deployment)
	nextStatus := currentStatus

	switch currentStatus {
	case deployapi.DeploymentStatusNew:
		// If the deployment has been cancelled, don't create a deployer pod, and
		// transition to failed immediately.
		if deployutil.IsDeploymentCancelled(deployment) {
			nextStatus = deployapi.DeploymentStatusFailed
			break
		}

		// Generate a deployer pod spec.
		podTemplate, err := c.makeDeployerPod(deployment)
		if err != nil {
			return fatalError(fmt.Sprintf("couldn't make deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), err))
		}

		// Create the deployer pod.
		deploymentPod, err := c.podClient.createPod(deployment.Namespace, podTemplate)
		if err == nil {
			deployment.Annotations[deployapi.DeploymentPodAnnotation] = deploymentPod.Name
			nextStatus = deployapi.DeploymentStatusPending
			glog.V(4).Infof("Created pod %s for deployment %s", deploymentPod.Name, deployutil.LabelForDeployment(deployment))
			break
		}

		// Retry on error.
		if !kerrors.IsAlreadyExists(err) {
			c.recorder.Eventf(deployment, "failedCreate", "Error creating deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), err)
			return fmt.Errorf("couldn't create deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), err)
		}

		// If the pod already exists, it's possible that a previous CreatePod
		// succeeded but the deployment state update failed and now we're re-
		// entering. Ensure that the pod is the one we created by verifying the
		// annotation on it, and throw a retryable error.
		existingPod, err := c.podClient.getPod(deployment.Namespace, deployutil.DeployerPodNameForDeployment(deployment.Name))
		if err != nil {
			c.recorder.Eventf(deployment, "failedCreate", "Error getting existing deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), err)
			return fmt.Errorf("couldn't fetch existing deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), err)
		}

		// Do a stronger check to validate that the existing deployer pod is
		// actually for this deployment, and if not, fail this deployment.
		//
		// TODO: Investigate checking the container image of the running pod and
		// comparing with the intended deployer pod image. If we do so, we'll need
		// to ensure that changes to 'unrelated' pods don't result in updates to
		// the deployment. So, the image check will have to be done in other areas
		// of the code as well.
		if deployutil.DeploymentNameFor(existingPod) != deployment.Name {
			nextStatus = deployapi.DeploymentStatusFailed
			deployment.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentFailedUnrelatedDeploymentExists
			c.recorder.Eventf(deployment, "failedCreate", "Error creating deployer pod for %s since another pod with the same name (%q) exists", deployutil.LabelForDeployment(deployment), existingPod.Name)
			glog.V(2).Infof("Couldn't create deployer pod for %s since an unrelated pod with the same name (%q) exists", deployutil.LabelForDeployment(deployment), existingPod.Name)
			break
		}

		// Update to pending relative to the existing validated deployer pod.
		deployment.Annotations[deployapi.DeploymentPodAnnotation] = existingPod.Name
		nextStatus = deployapi.DeploymentStatusPending
		glog.V(4).Infof("Detected existing deployer pod %s for deployment %s", existingPod.Name, deployutil.LabelForDeployment(deployment))
	case deployapi.DeploymentStatusPending, deployapi.DeploymentStatusRunning:
		// If the deployer pod has vanished, consider the deployment a failure.
		deployerPodName := deployutil.DeployerPodNameForDeployment(deployment.Name)
		if _, err := c.podClient.getPod(deployment.Namespace, deployerPodName); err != nil {
			if kerrors.IsNotFound(err) {
				nextStatus = deployapi.DeploymentStatusFailed
				deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
				deployment.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentFailedDeployerPodNoLongerExists
				c.recorder.Eventf(deployment, "failed", "Deployer pod %q has gone missing", deployerPodName)
				glog.V(4).Infof("Failing deployment %q because its deployer pod %q disappeared", deployutil.LabelForDeployment(deployment), deployerPodName)
				break
			} else {
				// We'll try again later on resync. Continue to process cancellations.
				glog.V(2).Infof("Error getting deployer pod %s for deployment %s: %#v", deployerPodName, deployutil.LabelForDeployment(deployment), err)
			}
		}

		// If the deployment is cancelled, terminate any deployer/hook pods.
		// NOTE: Do not mark the deployment as Failed just yet.
		// The deployment will be marked as Failed by the deployer pod controller
		// when the deployer pod failure state is picked up
		// Also, it will scale down the failed deployment and scale back up
		// the last successful completed deployment
		if deployutil.IsDeploymentCancelled(deployment) {
			deployerPods, err := c.podClient.getDeployerPodsFor(deployment.Namespace, deployment.Name)
			if err != nil {
				return fmt.Errorf("couldn't fetch deployer pods for %s while trying to cancel deployment: %v", deployutil.LabelForDeployment(deployment), err)
			}
			glog.V(4).Infof("Cancelling %d deployer pods for deployment %s", len(deployerPods), deployutil.LabelForDeployment(deployment))
			zeroDelay := int64(1)
			for _, deployerPod := range deployerPods {
				// Set the ActiveDeadlineSeconds on the pod so it's terminated very soon.
				if deployerPod.Spec.ActiveDeadlineSeconds == nil || *deployerPod.Spec.ActiveDeadlineSeconds != zeroDelay {
					deployerPod.Spec.ActiveDeadlineSeconds = &zeroDelay
					if _, err := c.podClient.updatePod(deployerPod.Namespace, &deployerPod); err != nil {
						c.recorder.Eventf(deployment, "failedCancellation", "Error cancelling deployer pod %s for deployment %s: %v", deployerPod.Name, deployutil.LabelForDeployment(deployment), err)
						return fmt.Errorf("couldn't cancel deployer pod %s for deployment %s: %v", deployutil.LabelForDeployment(deployment), err)
					}
					glog.V(4).Infof("Cancelled deployer pod %s for deployment %s", deployerPod.Name, deployutil.LabelForDeployment(deployment))
				}
			}
			c.recorder.Eventf(deployment, "cancelled", "Cancelled deployment")
		}
	case deployapi.DeploymentStatusFailed:
		// Nothing to do in this terminal state.
		glog.V(4).Infof("Ignoring deployment %s (status %s)", deployutil.LabelForDeployment(deployment), currentStatus)
	case deployapi.DeploymentStatusComplete:
		// now list any pods in the namespace that have the specified label
		deployerPods, err := c.podClient.getDeployerPodsFor(deployment.Namespace, deployment.Name)
		if err != nil {
			return fmt.Errorf("couldn't fetch deployer pods for %s after successful completion: %v", deployutil.LabelForDeployment(deployment), err)
		}
		glog.V(4).Infof("Deleting %d deployer pods for deployment %s", len(deployerPods), deployutil.LabelForDeployment(deployment))
		cleanedAll := true
		for _, deployerPod := range deployerPods {
			if err := c.podClient.deletePod(deployerPod.Namespace, deployerPod.Name); err != nil {
				if !kerrors.IsNotFound(err) {
					// if the pod deletion failed, then log the error and continue
					// we will try to delete any remaining deployer pods and return an error later
					kutil.HandleError(fmt.Errorf("couldn't delete completed deployer pod %s/%s for deployment %s: %v", deployment.Namespace, deployerPod.Name, deployutil.LabelForDeployment(deployment), err))
					cleanedAll = false
				}
				// Already deleted
			} else {
				glog.V(4).Infof("Deleted completed deployer pod %s/%s for deployment %s", deployment.Namespace, deployerPod.Name, deployutil.LabelForDeployment(deployment))
			}
		}

		if !cleanedAll {
			return fmt.Errorf("couldn't clean up all deployer pods for %s", deployutil.LabelForDeployment(deployment))
		}
	}

	if currentStatus != nextStatus {
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
		if _, err := c.deploymentClient.updateDeployment(deployment.Namespace, deployment); err != nil {
			c.recorder.Eventf(deployment, "failedUpdate", "Error updating deployment %s status to %s", deployutil.LabelForDeployment(deployment), nextStatus)
			return fmt.Errorf("couldn't update deployment %s to status %s: %v", deployutil.LabelForDeployment(deployment), nextStatus, err)
		}
		glog.V(4).Infof("Updated deployment %s status from %s to %s", deployutil.LabelForDeployment(deployment), currentStatus, nextStatus)
	}
	return nil
}

// makeDeployerPod creates a pod which implements deployment behavior. The pod is correlated to
// the deployment with an annotation.
func (c *DeploymentController) makeDeployerPod(deployment *kapi.ReplicationController) (*kapi.Pod, error) {
	deploymentConfig, err := c.decodeConfig(deployment)
	if err != nil {
		return nil, err
	}

	container, err := c.makeContainer(&deploymentConfig.Template.Strategy)
	if err != nil {
		return nil, err
	}

	// Add deployment environment variables to the container.
	envVars := []kapi.EnvVar{}
	for _, env := range container.Env {
		envVars = append(envVars, env)
	}
	envVars = append(envVars, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAME", Value: deployment.Name})
	envVars = append(envVars, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAMESPACE", Value: deployment.Namespace})

	// Assigning to a variable since its address is required
	maxDeploymentDurationSeconds := deployapi.MaxDeploymentDurationSeconds

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: deployutil.DeployerPodNameForDeployment(deployment.Name),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
			Labels: map[string]string{
				deployapi.DeployerPodForDeploymentLabel: deployment.Name,
			},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:      "deployment",
					Command:   container.Command,
					Args:      container.Args,
					Image:     container.Image,
					Env:       envVars,
					Resources: deploymentConfig.Template.Strategy.Resources,
				},
			},
			ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
			// Setting the node selector on the deployer pod so that it is created
			// on the same set of nodes as the pods.
			NodeSelector:   deployment.Spec.Template.Spec.NodeSelector,
			RestartPolicy:  kapi.RestartPolicyNever,
			ServiceAccount: c.serviceAccount,
		},
	}

	pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent

	return pod, nil
}

// deploymentClient abstracts access to deployments.
type deploymentClient interface {
	getDeployment(namespace, name string) (*kapi.ReplicationController, error)
	updateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// podClient abstracts access to pods.
type podClient interface {
	getPod(namespace, name string) (*kapi.Pod, error)
	createPod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	deletePod(namespace, name string) error
	updatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	getDeployerPodsFor(namespace, name string) ([]kapi.Pod, error)
}

// deploymentClientImpl is a pluggable deploymentClient.
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

// podClientImpl is a pluggable podClient.
type podClientImpl struct {
	getPodFunc             func(namespace, name string) (*kapi.Pod, error)
	createPodFunc          func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	deletePodFunc          func(namespace, name string) error
	updatePodFunc          func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	getDeployerPodsForFunc func(namespace, name string) ([]kapi.Pod, error)
}

func (i *podClientImpl) getPod(namespace, name string) (*kapi.Pod, error) {
	return i.getPodFunc(namespace, name)
}

func (i *podClientImpl) createPod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.createPodFunc(namespace, pod)
}

func (i *podClientImpl) deletePod(namespace, name string) error {
	return i.deletePodFunc(namespace, name)
}

func (i *podClientImpl) updatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.updatePodFunc(namespace, pod)
}

func (i *podClientImpl) getDeployerPodsFor(namespace, name string) ([]kapi.Pod, error) {
	return i.getDeployerPodsForFunc(namespace, name)
}
