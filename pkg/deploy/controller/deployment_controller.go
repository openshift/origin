package controller

import (
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
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
	// DeploymentInterface provides access to deployments.
	DeploymentInterface dcDeploymentInterface
	// PodInterface provides access to pods.
	PodInterface dcPodInterface
	// NextDeployment blocks until the next deployment is available.
	NextDeployment func() *kapi.ReplicationController
	// NextPod blocks until the next pod is available.
	NextPod func() *kapi.Pod
	// DeploymentStore is a cache of deployments.
	DeploymentStore cache.Store
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

// DeploymentContainerCreator knows how to create a deployment pod's container based on
// the deployment's strategy.
type DeploymentContainerCreator interface {
	CreateContainer(*deployapi.DeploymentStrategy) *kapi.Container
}

type dcDeploymentInterface interface {
	UpdateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

type dcPodInterface interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePod(namespace, id string) error
}

// Run begins watching and synchronizing deployment states.
func (dc *DeploymentController) Run() {
	go util.Until(func() { dc.HandleDeployment() }, 0, dc.Stop)
	go util.Until(func() { dc.HandlePod() }, 0, dc.Stop)
}

// HandleDeployment processes a new deployment and creates a new Pod which implements the specific
// deployment behavior. The deployment and pod are correlated with annotations. If the pod was
// successfully created, the deployment's status is transitioned to pending; otherwise, the status
// is transitioned to failed.
func (dc *DeploymentController) HandleDeployment() {
	deployment := dc.NextDeployment()

	if deployment.Annotations[deployapi.DeploymentStatusAnnotation] != string(deployapi.DeploymentStatusNew) {
		glog.V(4).Infof("Ignoring deployment %s with non-New status", deployment.Name)
		return
	}

	// TODO: transition to a failed state? seems like yes since this is probably not recoverable
	var deploymentPod *kapi.Pod
	var deploymentPodError error
	if deploymentPod, deploymentPodError = dc.makeDeploymentPod(deployment); deploymentPodError != nil {
		glog.V(0).Infof("Failed to make deployment pod for %s: %v", deployment.Name, deploymentPodError)
		return
	}

	nextStatus := deployment.Annotations[deployapi.DeploymentStatusAnnotation]
	if pod, err := dc.PodInterface.CreatePod(deployment.Namespace, deploymentPod); err != nil {
		// If the pod already exists, it's possible that a previous CreatePod succeeded but
		// the deployment state update failed and now we're re-entering.
		if kerrors.IsAlreadyExists(err) {
			nextStatus = string(deployapi.DeploymentStatusPending)
		} else {
			glog.Infof("Error creating pod for deployment %s: %v", deployment.Name, err)
			nextStatus = string(deployapi.DeploymentStatusFailed)
		}
	} else {
		glog.V(2).Infof("Created pod %s for deployment %s", pod.Name, deployment.Name)
		deployment.Annotations[deployapi.DeploymentPodAnnotation] = pod.Name
		nextStatus = string(deployapi.DeploymentStatusPending)
	}

	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = nextStatus

	glog.V(2).Infof("Updating deployment %s status %s -> %s", deployment.Name, deployment.Status, nextStatus)
	if _, err := dc.DeploymentInterface.UpdateDeployment(deployment.Namespace, deployment); err != nil {
		glog.V(2).Infof("Failed to update deployment %s: %v", deployment.Name, err)
	}
}

// HandlePod reconciles a pod's current state with its associated deployment and updates the
// deployment appropriately.
func (dc *DeploymentController) HandlePod() {
	pod := dc.NextPod()

	// Verify the assumption that we'll be given only pods correlated to a deployment
	deploymentID, hasDeploymentID := pod.Annotations[deployapi.DeploymentAnnotation]
	if !hasDeploymentID {
		glog.V(2).Infof("Unexpected state: Pod %s has no deployment annotation; skipping", pod.Name)
		return
	}

	deploymentObj, deploymentExists := dc.DeploymentStore.Get(deploymentID)
	if !deploymentExists {
		glog.V(2).Infof("Couldn't find deployment %s associated with pod %s", deploymentID, pod.Name)
		return
	}

	deployment := deploymentObj.(*kapi.ReplicationController)
	nextDeploymentStatus := deployment.Annotations[deployapi.DeploymentStatusAnnotation]

	switch pod.Status.Phase {
	case kapi.PodRunning:
		nextDeploymentStatus = string(deployapi.DeploymentStatusRunning)
	case kapi.PodSucceeded, kapi.PodFailed:
		nextDeploymentStatus = string(deployapi.DeploymentStatusComplete)
		// Detect failure based on the container state
		for _, info := range pod.Status.Info {
			if info.State.Termination != nil && info.State.Termination.ExitCode != 0 {
				nextDeploymentStatus = string(deployapi.DeploymentStatusFailed)
			}
		}

		// Automatically clean up successful pods
		if nextDeploymentStatus == string(deployapi.DeploymentStatusComplete) {
			if err := dc.PodInterface.DeletePod(deployment.Namespace, pod.Name); err != nil {
				glog.V(4).Infof("Couldn't delete completed pod %s for deployment %s: %#v", pod.Name, deployment.Name, err)
			} else {
				glog.V(4).Infof("Deleted completed pod %s for deployment %s", pod.Name, deployment.Name)
			}
		}
	}

	if deployment.Annotations[deployapi.DeploymentStatusAnnotation] != nextDeploymentStatus {
		glog.V(2).Infof("Updating deployment %s status %s -> %s", deployment.Name, deployment.Annotations[deployapi.DeploymentStatusAnnotation], nextDeploymentStatus)
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = nextDeploymentStatus
		if _, err := dc.DeploymentInterface.UpdateDeployment(pod.Namespace, deployment); err != nil {
			glog.V(2).Infof("Failed to update deployment %v: %v", deployment.Name, err)
		}
	}
}

// makeDeploymentPod creates a pod which implements deployment behavior. The pod is correlated to
// the deployment with an annotation.
func (dc *DeploymentController) makeDeploymentPod(deployment *kapi.ReplicationController) (*kapi.Pod, error) {
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
