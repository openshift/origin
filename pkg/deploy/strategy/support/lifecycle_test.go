package support

import (
	"fmt"
	"testing"
	//"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	//api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestHookExecutor_executeExecNewPodInvalidContainerRef(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "undefined",
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to CreatePod")
				return nil, nil
			},
			WatchPodFunc: func(namespace, name string) (watch.Interface, error) {
				t.Fatalf("unexpected call to WatchPod")
				return nil, nil
			},
		},
	}

	err := executor.Execute(hook, deployment)

	if err == nil {
		t.Fatalf("expected an error")
	}
	t.Logf("got expected error: %s", err)
}

func TestHookExecutor_executeExecNewWatchFailure(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "undefined",
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to CreatePod")
				return nil, nil
			},
			WatchPodFunc: func(namespace, name string) (watch.Interface, error) {
				return nil, fmt.Errorf("couldn't make watch")
			},
		},
	}

	err := executor.Execute(hook, deployment)

	if err == nil {
		t.Fatalf("expected an error")
	}
	t.Logf("got expected error: %s", err)
}

func TestHookExecutor_executeExecNewCreatePodFailure(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	podWatch := newTestWatch()

	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, fmt.Errorf("couldn't create pod")
			},
			WatchPodFunc: func(namespace, name string) (watch.Interface, error) {
				return podWatch, nil
			},
		},
	}

	err := executor.Execute(hook, deployment)

	if err == nil {
		t.Fatalf("expected an error")
	}
	t.Logf("got expected error: %s", err)
}

func TestHookExecutor_executeExecNewPodSucceeded(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "container1",
			Command:       []string{"overridden"},
			Env: []kapi.EnvVar{
				{
					Name:  "name",
					Value: "value",
				},
				{
					Name:  "ENV1",
					Value: "overridden",
				},
			},
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	podWatch := newTestWatch()

	var createdPod *kapi.Pod
	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				go func() {
					obj, _ := kapi.Scheme.Copy(pod)
					cp := obj.(*kapi.Pod)
					cp.Status.Phase = kapi.PodSucceeded
					podWatch.events <- watch.Event{
						Type:   watch.Modified,
						Object: cp,
					}
				}()
				createdPod = pod
				return createdPod, nil
			},
			WatchPodFunc: func(namespace, name string) (watch.Interface, error) {
				return podWatch, nil
			},
		},
	}

	err := executor.Execute(hook, deployment)

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if e, a := deployment.Spec.Template.Spec.Containers[0].Image, createdPod.Spec.Containers[0].Image; e != a {
		t.Fatalf("expected container image %s, got %s", e, a)
	}

	if e, a := "overridden", createdPod.Spec.Containers[0].Command[0]; e != a {
		t.Fatalf("expected container command %s, got %s", e, a)
	}

	// Verify environment merging
	expectedEnv := map[string]string{
		"name": "value",
		"ENV1": "overridden",
	}

	for k, v := range expectedEnv {
		found := false
		for _, env := range createdPod.Spec.Containers[0].Env {
			if env.Name == k && env.Value == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find %s=%s in pod environment", k, v)
		}
	}

	for _, env := range createdPod.Spec.Containers[0].Env {
		val, found := expectedEnv[env.Name]
		if !found || val != env.Value {
			t.Errorf("container has unexpected environment entry %s=%s", env.Name, env.Value)
		}
	}

	if e, a := kapi.RestartPolicyNever, createdPod.Spec.RestartPolicy; e != a {
		t.Fatalf("expected restart policy %s, got %s", e, a)
	}
}

func TestHookExecutor_executeExecNewPodFailed(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	podWatch := newTestWatch()

	var createdPod *kapi.Pod
	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				go func() {
					obj, _ := kapi.Scheme.Copy(pod)
					cp := obj.(*kapi.Pod)
					cp.Status.Phase = kapi.PodFailed
					podWatch.events <- watch.Event{
						Type:   watch.Modified,
						Object: cp,
					}
				}()
				createdPod = pod
				return createdPod, nil
			},
			WatchPodFunc: func(namespace, name string) (watch.Interface, error) {
				return podWatch, nil
			},
		},
	}

	err := executor.Execute(hook, deployment)

	if err == nil {
		t.Fatalf("expected an error", err)
	}
	t.Logf("got expected error: %s", err)
}

type testWatch struct {
	events chan watch.Event
}

func newTestWatch() *testWatch {
	return &testWatch{make(chan watch.Event)}
}

func (w *testWatch) Stop() {
}

func (w *testWatch) ResultChan() <-chan watch.Event {
	return w.events
}
