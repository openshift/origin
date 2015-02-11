package controller

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentController performs a deployment by creating a pod which is defined by a strategy.
// The status of the resulting deployment will follow the status of the corresponding pod.
//
// Deployments are represented by a ReplicationController.
type DeploymentController struct {
	// ContainerCreator makes the container for the deployment pod based on the strategy.
	ContainerCreator DeploymentContainerCreator
	// DeploymentClient provides access to deployments.
	DeploymentClient DeploymentControllerDeploymentClient
	// PodClient provides access to pods.
	PodClient DeploymentControllerPodClient
	// NextDeployment blocks until the next deployment is available.
	NextDeployment func() *kapi.ReplicationController
	// NextPod blocks until the next pod is available.
	NextPod func() *kapi.Pod
	// Environment is a set of environment which should be injected into all deployment pod
	// containers, in addition to whatever environment is specified by the ContainerCreator.
	Environment []kapi.EnvVar
	// UseLocalImages configures the ImagePullPolicy for containers in the deployment pod.
	UseLocalImages bool
	// Codec is used to decode DeploymentConfigs.
	Codec runtime.Codec
	// Stop is an optional channel that controls when the controller exits.
	Stop <-chan struct{}
}

// Run begins watching and synchronizing deployment states.
func (dc *DeploymentController) Run() {
	go util.Until(func() {
		err := dc.HandleDeployment(dc.NextDeployment())
		if err != nil {
			glog.Errorf("%v", err)
		}
	}, 0, dc.Stop)

	go util.Until(func() {
		err := dc.HandlePod(dc.NextPod())
		if err != nil {
			glog.Errorf("%v", err)
		}
	}, 0, dc.Stop)
}

// HandleDeployment processes a new deployment and creates a new Pod which implements the specific
// deployment behavior. The deployment and pod are correlated with annotations. If the pod was
// successfully created, the deployment's status is transitioned to pending.
func (dc *DeploymentController) HandleDeployment(deployment *kapi.ReplicationController) error {
	currentStatus := statusFor(deployment)
	nextStatus := currentStatus

	switch currentStatus {
	case deployapi.DeploymentStatusNew:
		deploymentPod, makeDeployerPodErr := dc.makeDeployerPod(deployment)
		if makeDeployerPodErr != nil {
			return fmt.Errorf("couldn't make deployer pod for %s: %v", labelForDeployment(deployment), makeDeployerPodErr)
		}

		if _, err := dc.PodClient.CreatePod(deployment.Namespace, deploymentPod); err != nil {
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
		if err := dc.PodClient.DeletePod(deployment.Namespace, podName); err != nil {
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
		if _, err := dc.DeploymentClient.UpdateDeployment(deployment.Namespace, deployment); err != nil {
			return fmt.Errorf("Couldn't update deployment %s to status %s: %v", labelForDeployment(deployment), nextStatus, err)
		}
		glog.V(2).Infof("Updated deployment %s status from %s to %s", labelForDeployment(deployment), currentStatus, nextStatus)
	}

	return nil
}

// HandlePod reconciles a pod's current state with its associated deployment and updates the
// deployment appropriately.
func (dc *DeploymentController) HandlePod(pod *kapi.Pod) error {
	// Verify the assumption that we'll be given only pods correlated to a deployment
	deploymentName, hasDeploymentName := pod.Annotations[deployapi.DeploymentAnnotation]
	if !hasDeploymentName {
		glog.V(2).Infof("Ignoring pod %s; no deployment annotation found", pod.Name)
		return nil
	}

	deployment, deploymentErr := dc.DeploymentClient.GetDeployment(pod.Namespace, deploymentName)
	if deploymentErr != nil {
		return fmt.Errorf("couldn't get deployment %s/%s associated with pod %s", pod.Namespace, deploymentName, pod.Name)
	}

	currentStatus := statusFor(deployment)
	nextStatus := currentStatus

	switch pod.Status.Phase {
	case kapi.PodRunning:
		nextStatus = deployapi.DeploymentStatusRunning
	case kapi.PodSucceeded, kapi.PodFailed:
		nextStatus = deployapi.DeploymentStatusComplete
		// Detect failure based on the container state
		for _, info := range pod.Status.Info {
			if info.State.Termination != nil && info.State.Termination.ExitCode != 0 {
				nextStatus = deployapi.DeploymentStatusFailed
			}
		}
	}

	if currentStatus != nextStatus {
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
		if _, err := dc.DeploymentClient.UpdateDeployment(deployment.Namespace, deployment); err != nil {
			return fmt.Errorf("couldn't update deployment %s to status %s: %v", labelForDeployment(deployment), nextStatus, err)
		}
		glog.V(2).Infof("Updated deployment %s status from %s to %s", labelForDeployment(deployment), currentStatus, nextStatus)
	}

	return nil
}

// makeDeployerPod creates a pod which implements deployment behavior. The pod is correlated to
// the deployment with an annotation.
func (dc *DeploymentController) makeDeployerPod(deployment *kapi.ReplicationController) (*kapi.Pod, error) {
	var deploymentConfig *deployapi.DeploymentConfig
	var decodeError error
	if deploymentConfig, decodeError = deployutil.DecodeDeploymentConfig(deployment, dc.Codec); decodeError != nil {
		return nil, decodeError
	}

	container := dc.ContainerCreator.CreateContainer(&deploymentConfig.Template.Strategy)

	// Combine the container environment, controller environment, and deployment values into
	// the pod's environment.
	envVars := container.Env
	envVars = append(envVars, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAME", Value: deployment.Name})
	envVars = append(envVars, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAMESPACE", Value: deployment.Namespace})
	for _, env := range dc.Environment {
		envVars = append(envVars, env)
	}

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
					Image:   container.Image,
					Env:     envVars,
				},
			},
			RestartPolicy: kapi.RestartPolicy{
				Never: &kapi.RestartPolicyNever{},
			},
		},
	}

	if dc.UseLocalImages {
		pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	}

	return pod, nil
}

// labelFor builds a string identifier for a DeploymentConfig.
func labelForDeployment(deployment *kapi.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// statusFor gets the DeploymentStatus for deployment from its annotations.
func statusFor(deployment *kapi.ReplicationController) deployapi.DeploymentStatus {
	return deployapi.DeploymentStatus(deployment.Annotations[deployapi.DeploymentStatusAnnotation])
}

// DeploymentContainerCreator knows how to create a deployment pod's container based on
// the deployment's strategy.
type DeploymentContainerCreator interface {
	CreateContainer(*deployapi.DeploymentStrategy) *kapi.Container
}

// DeploymentControllerDeploymentClient abstracts access to deployments.
type DeploymentControllerDeploymentClient interface {
	GetDeployment(namespace, name string) (*kapi.ReplicationController, error)
	UpdateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// DeploymentControllerPodClient abstracts access to pods.
type DeploymentControllerPodClient interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePod(namespace, name string) error
}

// DeploymentContainerCreatorImpl is a pluggable DeploymentContainerCreator.
type DeploymentContainerCreatorImpl struct {
	CreateContainerFunc func(*deployapi.DeploymentStrategy) *kapi.Container
}

func (i *DeploymentContainerCreatorImpl) CreateContainer(strategy *deployapi.DeploymentStrategy) *kapi.Container {
	return i.CreateContainerFunc(strategy)
}

// DeploymentControllerDeploymentClientImpl is a pluggable deploymentControllerDeploymentClient.
type DeploymentControllerDeploymentClientImpl struct {
	GetDeploymentFunc    func(namespace, name string) (*kapi.ReplicationController, error)
	UpdateDeploymentFunc func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *DeploymentControllerDeploymentClientImpl) GetDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return i.GetDeploymentFunc(namespace, name)
}

func (i *DeploymentControllerDeploymentClientImpl) UpdateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.UpdateDeploymentFunc(namespace, deployment)
}

// deploymentControllerPodClientImpl is a pluggable deploymentControllerPodClient.
type DeploymentControllerPodClientImpl struct {
	CreatePodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePodFunc func(namespace, name string) error
}

func (i *DeploymentControllerPodClientImpl) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.CreatePodFunc(namespace, pod)
}

func (i *DeploymentControllerPodClientImpl) DeletePod(namespace, name string) error {
	return i.DeletePodFunc(namespace, name)
}
