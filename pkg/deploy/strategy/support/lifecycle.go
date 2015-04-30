package support

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// HookExecutor executes a deployment lifecycle hook.
type HookExecutor struct {
	// PodClient provides access to pods.
	PodClient HookExecutorPodClient
}

// Execute executes hook in the context of deployment.
func (e *HookExecutor) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
	if hook.ExecNewPod != nil {
		return e.executeExecNewPod(hook.ExecNewPod, deployment)
	}
	return nil
}

// executeExecNewPod executes a ExecNewPod hook by creating a new pod based on
// the hook parameters and deployment. The pod is then synchronously watched
// until the pod completes, and if the pod failed, an error is returned.
func (e *HookExecutor) executeExecNewPod(hook *deployapi.ExecNewPodHook, deployment *kapi.ReplicationController) error {
	// Build a pod spec from the hook config and deployment
	var baseContainer *kapi.Container
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == hook.ContainerName {
			baseContainer = &container
			break
		}
	}
	if baseContainer == nil {
		return fmt.Errorf("no container named '%s' found in deployment template", hook.ContainerName)
	}

	// Generate a name for the pod
	podName := kapi.SimpleNameGenerator.GenerateName(fmt.Sprintf("deployment-%s-hook-", deployment.Name))

	// Build a merged environment; hook environment takes precedence over base
	// container environment
	envMap := map[string]string{}
	mergedEnv := []kapi.EnvVar{}
	for _, env := range baseContainer.Env {
		envMap[env.Name] = env.Value
	}
	for _, env := range hook.Env {
		envMap[env.Name] = env.Value
	}
	for k, v := range envMap {
		mergedEnv = append(mergedEnv, kapi.EnvVar{Name: k, Value: v})
	}

	podSpec := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: podName,
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:    "lifecycle",
					Image:   baseContainer.Image,
					Command: hook.Command,
					Env:     mergedEnv,
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}

	// Set up a watch for the pod
	podWatch, err := e.PodClient.WatchPod(deployment.Namespace, podName)
	if err != nil {
		return fmt.Errorf("couldn't create watch for pod %s/%s: %s", deployment.Namespace, podName, err)
	}
	defer podWatch.Stop()

	// Try to create the pod
	pod, err := e.PodClient.CreatePod(deployment.Namespace, podSpec)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return fmt.Errorf("couldn't create lifecycle pod for %s: %v", labelForDeployment(deployment), err)
		}
	} else {
		glog.V(0).Infof("Created lifecycle pod %s for deployment %s", pod.Name, labelForDeployment(deployment))
	}

	// Wait for the pod to finish.
	// TODO: Delete pod before returning?
	glog.V(0).Infof("Waiting for hook pod %s/%s to complete", pod.Namespace, pod.Name)
	for {
		select {
		case event, ok := <-podWatch.ResultChan():
			if !ok {
				return fmt.Errorf("couldn't watch pod %s/%s", pod.Namespace, pod.Name)
			}
			if event.Type == watch.Error {
				return kerrors.FromObject(event.Object)
			}
			pod, podOk := event.Object.(*kapi.Pod)
			if !podOk {
				return fmt.Errorf("expected a pod event, got a %s", reflect.TypeOf(event.Object))
			}
			glog.V(0).Infof("Lifecycle pod %s/%s in phase %s", pod.Namespace, pod.Name, pod.Status.Phase)
			switch pod.Status.Phase {
			case kapi.PodSucceeded:
				return nil
			case kapi.PodFailed:
				// TODO: Add context
				return fmt.Errorf("pod failed")
			}
		}
	}
}

// labelForDeployment builds a string identifier for a deployment.
func labelForDeployment(deployment *kapi.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// HookExecutorPodClient abstracts access to pods.
type HookExecutorPodClient interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	WatchPod(namespace, name string) (watch.Interface, error)
}

// HookExecutorPodClientImpl is a pluggable HookExecutorPodClient.
type HookExecutorPodClientImpl struct {
	CreatePodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	WatchPodFunc  func(namespace, name string) (watch.Interface, error)
}

func (i *HookExecutorPodClientImpl) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.CreatePodFunc(namespace, pod)
}

func (i *HookExecutorPodClientImpl) WatchPod(namespace, name string) (watch.Interface, error) {
	return i.WatchPodFunc(namespace, name)
}
