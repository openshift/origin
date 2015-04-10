package deployment

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

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
}

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string { return "fatal error handling deployment: " + string(e) }

// Handle processes deployment and either creates a deployer pod or responds
// to a terminal deployment status.
func (c *DeploymentController) Handle(deployment *kapi.ReplicationController) error {
	currentStatus := statusFor(deployment)
	nextStatus := currentStatus

	switch currentStatus {
	case deployapi.DeploymentStatusNew:
		podTemplate, err := c.makeDeployerPod(deployment)
		if err != nil {
			return fatalError(fmt.Sprintf("couldn't make deployer pod for %s: %v", labelForDeployment(deployment), err))
		}

		deploymentPod, err := c.podClient.createPod(deployment.Namespace, podTemplate)
		if err != nil {
			// If the pod already exists, it's possible that a previous CreatePod succeeded but
			// the deployment state update failed and now we're re-entering.
			if !kerrors.IsAlreadyExists(err) {
				return fmt.Errorf("couldn't create deployer pod for %s: %v", labelForDeployment(deployment), err)
			}
		} else {
			glog.V(2).Infof("Created pod %s for deployment %s", deploymentPod.Name, labelForDeployment(deployment))
		}

		deployment.Annotations[deployapi.DeploymentPodAnnotation] = deploymentPod.Name
		nextStatus = deployapi.DeploymentStatusPending
	case deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
		deployapi.DeploymentStatusFailed:
		glog.V(4).Infof("Ignoring deployment %s (status %s)", labelForDeployment(deployment), currentStatus)
	case deployapi.DeploymentStatusComplete:
		// Automatically clean up successful pods
		// TODO: Could probably do a lookup here to skip the delete call, but it's not worth adding
		// yet since (delete retries will only normally occur during full a re-sync).
		podName := deployment.Annotations[deployapi.DeploymentPodAnnotation]
		if err := c.podClient.deletePod(deployment.Namespace, podName); err != nil {
			if !kerrors.IsNotFound(err) {
				return fmt.Errorf("couldn't delete completed deployer pod %s/%s for deployment %s: %v", deployment.Namespace, podName, labelForDeployment(deployment), err)
			}
			// Already deleted
		} else {
			glog.V(4).Infof("Deleted completed deployer pod %s/%s for deployment %s", deployment.Namespace, podName, labelForDeployment(deployment))
		}
	}

	if currentStatus != nextStatus {
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
		if _, err := c.deploymentClient.updateDeployment(deployment.Namespace, deployment); err != nil {
			return fmt.Errorf("couldn't update deployment %s to status %s: %v", labelForDeployment(deployment), nextStatus, err)
		}
		glog.V(2).Infof("Updated deployment %s status from %s to %s", labelForDeployment(deployment), currentStatus, nextStatus)
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

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			GenerateName: deployutil.DeployerPodNameForDeployment(deployment),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:    "deployment",
					Command: container.Command,
					Args:    container.Args,
					Image:   container.Image,
					Env:     envVars,
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}

	pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent

	return pod, nil
}

// labelForDeployment builds a string identifier for a DeploymentConfig.
func labelForDeployment(deployment *kapi.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// statusFor gets the DeploymentStatus for deployment from its annotations.
func statusFor(deployment *kapi.ReplicationController) deployapi.DeploymentStatus {
	return deployapi.DeploymentStatus(deployment.Annotations[deployapi.DeploymentStatusAnnotation])
}

// deploymentClient abstracts access to deployments.
type deploymentClient interface {
	getDeployment(namespace, name string) (*kapi.ReplicationController, error)
	updateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// podClient abstracts access to pods.
type podClient interface {
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
	createPodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	deletePodFunc func(namespace, name string) error
}

func (i *podClientImpl) createPod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.createPodFunc(namespace, pod)
}

func (i *podClientImpl) deletePod(namespace, name string) error {
	return i.deletePodFunc(namespace, name)
}
