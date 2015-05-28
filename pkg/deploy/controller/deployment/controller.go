package deployment

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"

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

func (e fatalError) Error() string {
	return fmt.Sprintf("fatal error handling Deployment: %s", string(e))
}

// Handle processes deployment and either creates a deployer pod or responds
// to a terminal deployment status.
func (c *DeploymentController) Handle(deployment *kapi.ReplicationController) error {
	currentStatus := deployutil.DeploymentStatusFor(deployment)
	nextStatus := currentStatus

	switch currentStatus {
	case deployapi.DeploymentStatusNew:
		podTemplate, err := c.makeDeployerPod(deployment)
		if err != nil {
			return fatalError(fmt.Sprintf("couldn't make deployer pod for %s/%s: %v", deployment.Namespace, deployutil.LabelForDeployment(deployment), err))
		}

		deploymentPod, err := c.podClient.createPod(deployment.Namespace, podTemplate)
		if err != nil {
			if kerrors.IsAlreadyExists(err) {
				// If the pod already exists, it's possible that a previous CreatePod succeeded but
				// the deployment state update failed and now we're re-entering.
				// Ensure that the pod is the one we created by verifying the annotation on it
				existingPod, err := c.podClient.getPod(deployment.Namespace, deployutil.DeployerPodNameForDeployment(deployment))
				if err != nil {
					c.recorder.Eventf(deployment, "failedCreate", "Error getting existing deployer pod for %s/%s: %v", deployment.Namespace, deployutil.LabelForDeployment(deployment), err)
					return fmt.Errorf("couldn't fetch existing deployer pod for %s/%s: %v", deployment.Namespace, deployutil.LabelForDeployment(deployment), err)
				}
				// TODO: Investigate checking the container image of the running pod and
				// comparing with the intended deployer pod image.
				// If we do so, we'll need to ensure that changes to 'unrelated' pods
				// don't result in updates to the deployment
				// So, the image check will have to be done in other areas of the code as well
				if deployutil.DeploymentNameFor(existingPod) == deployment.Name {
					// we'll just set the deploymentPod so that pod name annotation
					// can be set on the deployment below
					deploymentPod = existingPod
				} else {
					c.recorder.Eventf(deployment, "failedCreate", "Error creating deployer pod for %s/%s since another pod with the same name exists", deployment.Namespace, deployutil.LabelForDeployment(deployment))

					// we seem to have an unrelated pod running with the same name as the deployment
					// set the deployment status to Failed
					failedStatus := string(deployapi.DeploymentStatusFailed)
					deployment.Annotations[deployapi.DeploymentStatusAnnotation] = failedStatus
					deployment.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentFailedUnrelatedDeploymentExists
					if _, err := c.deploymentClient.updateDeployment(deployment.Namespace, deployment); err != nil {
						c.recorder.Eventf(deployment, "failedUpdate", "Error updating Deployment %s/%s status to %s", deployment.Namespace, deployutil.LabelForDeployment(deployment), failedStatus)
						glog.Errorf("Error updating Deployment %s/%s status to %s", deployment.Namespace, deployutil.LabelForDeployment(deployment), failedStatus)
					} else {
						glog.V(4).Infof("Updated Deployment %s/%s status to %s", deployment.Namespace, deployutil.LabelForDeployment(deployment), failedStatus)
					}
					return fatalError(fmt.Sprintf("couldn't create deployer pod for %s/%s since an unrelated pod with the same name exists", deployment.Namespace, deployutil.LabelForDeployment(deployment)))
				}
			} else {
				c.recorder.Eventf(deployment, "failedCreate", "Error creating deployer pod for %s/%s: %v", deployment.Namespace, deployutil.LabelForDeployment(deployment), err)
				return fmt.Errorf("couldn't create deployer pod for %s/%s: %v", deployment.Namespace, deployutil.LabelForDeployment(deployment), err)
			}
		} else {
			glog.V(4).Infof("Created pod %s for Deployment %s/%s", deploymentPod.Name, deployment.Namespace, deployutil.LabelForDeployment(deployment))
		}

		deployment.Annotations[deployapi.DeploymentPodAnnotation] = deploymentPod.Name
		nextStatus = deployapi.DeploymentStatusPending
	case deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
		deployapi.DeploymentStatusFailed:
		glog.V(4).Infof("Ignoring Deployment %s/%s (status %s)", deployment.Namespace, deployutil.LabelForDeployment(deployment), currentStatus)
	case deployapi.DeploymentStatusComplete:
		// Automatically clean up successful pods
		// TODO: Could probably do a lookup here to skip the delete call, but it's not worth adding
		// yet since (delete retries will only normally occur during full a re-sync).
		podName := deployutil.DeployerPodNameFor(deployment)
		if err := c.podClient.deletePod(deployment.Namespace, podName); err != nil {
			if !kerrors.IsNotFound(err) {
				return fmt.Errorf("couldn't delete completed deployer pod %s/%s for Deployment %s: %v", deployment.Namespace, podName, deployutil.LabelForDeployment(deployment), err)
			}
			// Already deleted
		} else {
			glog.V(4).Infof("Deleted completed deployer pod %s/%s for Deployment %s", deployment.Namespace, podName, deployutil.LabelForDeployment(deployment))
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
			Name: deployutil.DeployerPodNameForDeployment(deployment),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
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
			RestartPolicy:         kapi.RestartPolicyNever,
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
	getPodFunc    func(namespace, name string) (*kapi.Pod, error)
	createPodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	deletePodFunc func(namespace, name string) error
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
