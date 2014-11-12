package controller

import (
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// CustomPodDeploymentController implements the DeploymentStrategyTypeCustomPod deployment strategy.
// Its behavior is to delegate the deployment logic to a pod. The status of the resulting Deployment
// will follow the status of the corresponding pod.
type CustomPodDeploymentController struct {
	DeploymentInterface dcDeploymentInterface
	PodControl          PodControlInterface
	Environment         []kapi.EnvVar
	NextDeployment      func() *deployapi.Deployment
	NextPod             func() *kapi.Pod
	DeploymentStore     cache.Store
	DefaultImage        string
	UseLocalImages      bool
}

type dcDeploymentInterface interface {
	UpdateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error)
}

type PodControlInterface interface {
	createPod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	deletePod(namespace, id string) error
}

type RealPodControl struct {
	KubeClient kclient.Interface
}

func (r RealPodControl) createPod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return r.KubeClient.Pods(namespace).Create(pod)
}

func (r RealPodControl) deletePod(namespace, id string) error {
	return r.KubeClient.Pods(namespace).Delete(id)
}

// Run begins watching and synchronizing deployment states.
func (dc *CustomPodDeploymentController) Run() {
	go util.Forever(func() { dc.HandleDeployment() }, 0)
	go util.Forever(func() { dc.HandlePod() }, 0)
}

// Invokes the appropriate handler for the current state of the given deployment.
func (dc *CustomPodDeploymentController) HandleDeployment() error {
	deployment := dc.NextDeployment()

	if deployment.Strategy.Type != deployapi.DeploymentStrategyTypeCustomPod {
		glog.V(4).Infof("Dropping deployment %s due to incompatible strategy type %s", deployment.Name, deployment.Strategy)
		return nil
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), deployment.Namespace)
	glog.V(4).Infof("Synchronizing deployment.Name: %v status: %v resourceVersion: %v",
		deployment.Name, deployment.Status, deployment.ResourceVersion)

	// TODO: this can all be simplified because the deployment event loop only creates new pods
	// for the pod state machine to handle
	if deployment.Strategy.Type != deployapi.DeploymentStrategyTypeCustomPod {
		glog.V(4).Infof("Dropping deployment %v", deployment.Name)
		return nil
	}

	if deployment.Status != deployapi.DeploymentStatusNew {
		glog.V(4).Infof("Dropping deployment %v", deployment.Name)
		return nil
	}

	deploymentPod := dc.makeDeploymentPod(deployment)
	glog.V(2).Infof("Attempting to create deployment pod: %+v", deploymentPod)
	if pod, err := dc.PodControl.createPod(deployment.Namespace, deploymentPod); err != nil {
		glog.V(2).Infof("Received error creating pod: %v", err)
		deployment.Status = deployapi.DeploymentStatusFailed
	} else {
		glog.V(4).Infof("Successfully created pod %+v", pod)
		deployment.Status = deployapi.DeploymentStatusPending
	}

	return dc.saveDeployment(ctx, deployment)
}

func (dc *CustomPodDeploymentController) HandlePod() error {
	pod := dc.NextPod()
	ctx := kapi.WithNamespace(kapi.NewContext(), pod.Namespace)
	glog.V(2).Infof("Synchronizing pod id: %v status: %v", pod.Name, pod.CurrentState.Status)

	// assumption: filter prevents this label from not being present
	id := pod.Labels["deployment"]
	obj, exists := dc.DeploymentStore.Get(id)
	if !exists {
		return kerrors.NewNotFound("Deployment", id)
	}
	deployment := obj.(*deployapi.Deployment)

	if deployment.Status == deployapi.DeploymentStatusComplete || deployment.Status == deployapi.DeploymentStatusFailed {
		return nil
	}
	currentDeploymentStatus := deployment.Status

	switch pod.CurrentState.Status {
	case kapi.PodRunning:
		deployment.Status = deployapi.DeploymentStatusRunning
	case kapi.PodSucceeded:
		deployment.Status = dc.inspectTerminatedDeploymentPod(deployment, pod)
	}

	if currentDeploymentStatus != deployment.Status {
		return dc.saveDeployment(ctx, deployment)
	}

	return nil
}

func deploymentPodID(deployment *deployapi.Deployment) string {
	return "deploy-" + deployment.Name
}

func (dc *CustomPodDeploymentController) inspectTerminatedDeploymentPod(deployment *deployapi.Deployment, pod *kapi.Pod) deployapi.DeploymentStatus {
	nextStatus := deployment.Status
	if pod.CurrentState.Status != kapi.PodSucceeded {
		glog.V(2).Infof("The deployment has not yet finished. Pod status is %s. Continuing", pod.CurrentState.Status)
		return nextStatus
	}

	nextStatus = deployapi.DeploymentStatusComplete
	for _, info := range pod.CurrentState.Info {
		if info.State.Termination != nil && info.State.Termination.ExitCode != 0 {
			nextStatus = deployapi.DeploymentStatusFailed
		}
	}

	if nextStatus == deployapi.DeploymentStatusComplete {
		podID := deploymentPodID(deployment)
		glog.V(2).Infof("Removing deployment pod for ID %v", podID)
		dc.PodControl.deletePod(deployment.Namespace, podID)
	}

	glog.V(4).Infof("The deployment pod has finished. Setting deployment state to %s", deployment.Status)
	return nextStatus
}

func (dc *CustomPodDeploymentController) saveDeployment(ctx kapi.Context, deployment *deployapi.Deployment) error {
	glog.V(4).Infof("Saving deployment %v status: %v", deployment.Name, deployment.Status)
	_, err := dc.DeploymentInterface.UpdateDeployment(ctx, deployment)
	if err != nil {
		glog.V(2).Infof("Received error while saving deployment %v: %v", deployment.Name, err)
	}
	return err
}

func (dc *CustomPodDeploymentController) makeDeploymentPod(deployment *deployapi.Deployment) *kapi.Pod {
	podID := deploymentPodID(deployment)

	envVars := deployment.Strategy.CustomPod.Environment
	envVars = append(envVars, kapi.EnvVar{Name: "KUBERNETES_deployment.Name", Value: deployment.Name})
	for _, env := range dc.Environment {
		envVars = append(envVars, env)
	}

	image := deployment.Strategy.CustomPod.Image
	if len(image) == 0 {
		image = dc.DefaultImage
	}

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: podID,
			Labels: map[string]string{
				"deployment": deployment.Name,
			},
		},
		DesiredState: kapi.PodState{
			Manifest: kapi.ContainerManifest{
				Version: "v1beta1",
				Containers: []kapi.Container{
					{
						Name:  "deployment",
						Image: image,
						Env:   envVars,
					},
				},
				RestartPolicy: kapi.RestartPolicy{
					Never: &kapi.RestartPolicyNever{},
				},
			},
		},
	}
	if dc.UseLocalImages {
		pod.DesiredState.Manifest.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	}

	return pod
}
