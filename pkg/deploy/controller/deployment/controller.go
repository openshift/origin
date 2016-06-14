package deployment

import (
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/record"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	"github.com/openshift/origin/pkg/util"
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
	makeContainer func(strategy *deployapi.DeploymentStrategy) *kapi.Container
	// decodeConfig knows how to decode the deploymentConfig from a deployment's annotations.
	decodeConfig func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error)
	// recorder is used to record events.
	recorder record.EventRecorder
}

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string { return "fatal error handling deployment: " + string(e) }

// actionableError is an error on which users can act
type actionableError string

func (e actionableError) Error() string { return string(e) }

// Handle processes deployment and either creates a deployer pod or responds
// to a terminal deployment status.
func (c *DeploymentController) Handle(deployment *kapi.ReplicationController) error {
	currentStatus := deployutil.DeploymentStatusFor(deployment)
	nextStatus := currentStatus
	deploymentScaled := false

	switch currentStatus {
	case deployapi.DeploymentStatusNew:
		// If the deployment has been cancelled, don't create a deployer pod.
		// Instead try to delete any deployer pods found and transition the
		// deployment to Pending so that the deployment config controller
		// continues to see the deployment as in-flight. Eventually the deletion
		// of the deployer pod should cause a requeue of this deployment and
		// then it can be transitioned to Failed by this controller.
		if deployutil.IsDeploymentCancelled(deployment) {
			nextStatus = deployapi.DeploymentStatusPending
			if err := c.cleanupDeployerPods(deployment); err != nil {
				return err
			}
			break
		}

		// If the pod already exists, it's possible that a previous CreatePod
		// succeeded but the deployment state update failed and now we're re-
		// entering. Ensure that the pod is the one we created by verifying the
		// annotation on it, and throw a retryable error.
		existingPod, err := c.podClient.getPod(deployment.Namespace, deployutil.DeployerPodNameForDeployment(deployment.Name))
		if err != nil && !kerrors.IsNotFound(err) {
			return fmt.Errorf("couldn't fetch existing deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), err)
		}
		if err == nil && existingPod != nil {
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
				c.emitDeploymentEvent(deployment, kapi.EventTypeWarning, "FailedCreate", fmt.Sprintf("Error creating deployer pod since another pod with the same name (%q) exists", existingPod.Name))
				glog.V(2).Infof("Couldn't create deployer pod for %s since an unrelated pod with the same name (%q) exists", deployutil.LabelForDeployment(deployment), existingPod.Name)
			} else {
				// Update to pending relative to the existing validated deployer pod.
				deployment.Annotations[deployapi.DeploymentPodAnnotation] = existingPod.Name
				nextStatus = deployapi.DeploymentStatusPending
				glog.V(4).Infof("Detected existing deployer pod %s for deployment %s", existingPod.Name, deployutil.LabelForDeployment(deployment))
			}
			// Don't try and re-create the deployer pod.
			break
		}

		if _, ok := deployment.Annotations[deployapi.DeploymentIgnorePodAnnotation]; ok {
			return nil
		}

		// Generate a deployer pod spec.
		podTemplate, err := c.makeDeployerPod(deployment)
		if err != nil {
			// TODO: Make this an oc status error
			return fatalError(fmt.Sprintf("couldn't make deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), err))
		}
		// Create the deployer pod.
		deploymentPod, err := c.podClient.createPod(deployment.Namespace, podTemplate)
		// Retry on error.
		if err != nil {
			return actionableError(fmt.Sprintf("couldn't create deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), err))
		}
		deployment.Annotations[deployapi.DeploymentPodAnnotation] = deploymentPod.Name
		nextStatus = deployapi.DeploymentStatusPending
		glog.V(4).Infof("Created deployer pod %s for deployment %s", deploymentPod.Name, deployutil.LabelForDeployment(deployment))
	case deployapi.DeploymentStatusPending, deployapi.DeploymentStatusRunning:
		// If the deployer pod has vanished, consider the deployment a failure.
		deployerPodName := deployutil.DeployerPodNameForDeployment(deployment.Name)
		_, err := c.podClient.getPod(deployment.Namespace, deployerPodName)
		switch {
		case kerrors.IsNotFound(err):
			nextStatus = deployapi.DeploymentStatusFailed
			// If the deployment is cancelled here then we deleted the deployer in a previous
			// resync of the deployment.
			if !deployutil.IsDeploymentCancelled(deployment) {
				deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
				deployment.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentFailedDeployerPodNoLongerExists
				c.emitDeploymentEvent(deployment, kapi.EventTypeWarning, "Failed", fmt.Sprintf("Deployer pod %q has gone missing", deployerPodName))
				glog.V(4).Infof("Failing deployment %q because its deployer pod %q disappeared", deployutil.LabelForDeployment(deployment), deployerPodName)
			}

		case err != nil:
			// We'll try again later on resync. Continue to process cancellations.
			glog.V(4).Infof("Error getting deployer pod %s for deployment %s: %#v", deployerPodName, deployutil.LabelForDeployment(deployment), err)

		default: /* err == nil */
			// If the deployment has been cancelled, delete any deployer pods
			// found and transition the deployment to Pending so that the
			// deployment config controller continues to see the deployment
			// as in-flight. Eventually the deletion of the deployer pod should
			// cause a requeue of this deployment and then it can be transitioned
			// to Failed by this controller.
			if deployutil.IsDeploymentCancelled(deployment) {
				if err := c.cleanupDeployerPods(deployment); err != nil {
					return err
				}
			}
		}
	case deployapi.DeploymentStatusFailed:
		// Check for test deployment and ensure the deployment scale matches
		if config, err := c.decodeConfig(deployment); err == nil && config.Spec.Test {
			deploymentScaled = deployment.Spec.Replicas != 0
			deployment.Spec.Replicas = 0
		}
		// Try to cleanup once more a cancelled deployment in case hook pods
		// were created just after we issued the first cleanup request.
		if deployutil.IsDeploymentCancelled(deployment) {
			if err := c.cleanupDeployerPods(deployment); err != nil {
				return err
			}
		}
	case deployapi.DeploymentStatusComplete:
		// Check for test deployment and ensure the deployment scale matches
		if config, err := c.decodeConfig(deployment); err == nil && config.Spec.Test {
			deploymentScaled = deployment.Spec.Replicas != 0
			deployment.Spec.Replicas = 0
		}

		if err := c.cleanupDeployerPods(deployment); err != nil {
			return err
		}
	}

	if deployutil.CanTransitionPhase(currentStatus, nextStatus) || deploymentScaled {
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
		if _, err := c.deploymentClient.updateDeployment(deployment.Namespace, deployment); err != nil {
			return fmt.Errorf("couldn't update deployment %s to status %s: %v", deployutil.LabelForDeployment(deployment), nextStatus, err)
		}
		glog.V(4).Infof("Updated deployment %s status from %s to %s (scale: %d)", deployutil.LabelForDeployment(deployment), currentStatus, nextStatus, deployment.Spec.Replicas)
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

	container := c.makeContainer(&deploymentConfig.Spec.Strategy)

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
					Resources: deploymentConfig.Spec.Strategy.Resources,
				},
			},
			ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
			DNSPolicy:             deployment.Spec.Template.Spec.DNSPolicy,
			ImagePullSecrets:      deployment.Spec.Template.Spec.ImagePullSecrets,
			// Setting the node selector on the deployer pod so that it is created
			// on the same set of nodes as the pods.
			NodeSelector:       deployment.Spec.Template.Spec.NodeSelector,
			RestartPolicy:      kapi.RestartPolicyNever,
			ServiceAccountName: c.serviceAccount,
		},
	}

	// MergeInfo will not overwrite values unless the flag OverwriteExistingDstKey is set.
	util.MergeInto(pod.Labels, deploymentConfig.Spec.Strategy.Labels, 0)
	util.MergeInto(pod.Annotations, deploymentConfig.Spec.Strategy.Annotations, 0)

	pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent

	return pod, nil
}

func (c *DeploymentController) cleanupDeployerPods(deployment *kapi.ReplicationController) error {
	deployerPods, err := c.podClient.getDeployerPodsFor(deployment.Namespace, deployment.Name)
	if err != nil {
		return fmt.Errorf("couldn't fetch deployer pods for %q: %v", deployutil.LabelForDeployment(deployment), err)
	}

	cleanedAll := true
	for _, deployerPod := range deployerPods {
		if err := c.podClient.deletePod(deployerPod.Namespace, deployerPod.Name); err != nil && !kerrors.IsNotFound(err) {
			// if the pod deletion failed, then log the error and continue
			// we will try to delete any remaining deployer pods and return an error later
			utilruntime.HandleError(fmt.Errorf("couldn't delete completed deployer pod %q for deployment %q: %v", deployerPod.Name, deployutil.LabelForDeployment(deployment), err))
			cleanedAll = false
		}
	}

	if !cleanedAll {
		return actionableError(fmt.Sprintf("couldn't clean up all deployer pods for %s", deployment.Name))
	}
	return nil
}

func (c *DeploymentController) emitDeploymentEvent(deployment *kapi.ReplicationController, eventType, title, message string) {
	if config, _ := c.decodeConfig(deployment); config != nil {
		c.recorder.Eventf(config, eventType, title, fmt.Sprintf("%s: %s", deployment.Name, message))
	} else {
		c.recorder.Eventf(deployment, eventType, title, message)
	}
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
