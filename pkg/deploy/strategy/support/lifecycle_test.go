package support

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestHookExecutor_executeExecNewCreatePodFailure(t *testing.T) {
	hook := &deployapi.ExecNewPodHook{
		ContainerName: "container1",
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, fmt.Errorf("couldn't create pod")
			},
			PodWatchFunc: func(namespace, name, resourceVersion string) func() *kapi.Pod {
				return func() *kapi.Pod { return nil }
			},
		},
	}

	err := executor.executeExecNewPod(hook, deployment)

	if err == nil {
		t.Fatalf("expected an error")
	}
	t.Logf("got expected error: %s", err)
}

func TestHookExecutor_executeExecNewPodSucceeded(t *testing.T) {
	hook := &deployapi.ExecNewPodHook{
		ContainerName: "container1",
	}

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)

	var createdPod *kapi.Pod
	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				createdPod = pod
				return createdPod, nil
			},
			PodWatchFunc: func(namespace, name, resourceVersion string) func() *kapi.Pod {
				createdPod.Status.Phase = kapi.PodSucceeded
				return func() *kapi.Pod { return createdPod }
			},
		},
	}

	err := executor.executeExecNewPod(hook, deployment)

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if createdPod.Spec.ActiveDeadlineSeconds == nil {
		t.Fatalf("expected ActiveDeadlineSeconds to be set on the deployment hook executor pod")
	}

	if *createdPod.Spec.ActiveDeadlineSeconds != deployapi.MaxDeploymentDurationSeconds {
		t.Fatalf("expected ActiveDeadlineSeconds to be set to %d; found: %d", deployapi.MaxDeploymentDurationSeconds, *createdPod.Spec.ActiveDeadlineSeconds)
	}
}

func TestHookExecutor_executeExecNewPodFailed(t *testing.T) {
	hook := &deployapi.ExecNewPodHook{
		ContainerName: "container1",
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	var createdPod *kapi.Pod
	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				createdPod = pod
				return createdPod, nil
			},
			PodWatchFunc: func(namespace, name, resourceVersion string) func() *kapi.Pod {
				createdPod.Status.Phase = kapi.PodFailed
				return func() *kapi.Pod { return createdPod }
			},
		},
	}

	err := executor.executeExecNewPod(hook, deployment)

	if err == nil {
		t.Fatalf("expected an error", err)
	}
	t.Logf("got expected error: %s", err)
}

func TestHookExecutor_buildContainerInvalidContainerRef(t *testing.T) {
	hook := &deployapi.ExecNewPodHook{
		ContainerName: "undefined",
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	_, err := buildContainer(hook, deployment)

	if err == nil {
		t.Fatalf("expected an error")
	}
	t.Logf("got expected error: %s", err)
}

func TestHookExecutor_buildContainerOk(t *testing.T) {
	hook := &deployapi.ExecNewPodHook{
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
	}

	config := deploytest.OkDeploymentConfig(1)

	cpuLimit := resource.MustParse("10")
	memoryLimit := resource.MustParse("10M")
	config.Template.ControllerTemplate.Template.Spec.Containers[0].Resources = kapi.ResourceRequirements{
		Limits: kapi.ResourceList{
			kapi.ResourceCPU:    cpuLimit,
			kapi.ResourceMemory: memoryLimit,
		},
	}

	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)

	podSpec, err := buildContainer(hook, deployment)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	gotContainer := podSpec.Spec.Containers[0]

	// Verify the correct image was selected
	if e, a := deployment.Spec.Template.Spec.Containers[0].Image, gotContainer.Image; e != a {
		t.Fatalf("expected container image %s, got %s", e, a)
	}

	// Verify command overriding
	if e, a := "overridden", gotContainer.Command[0]; e != a {
		t.Fatalf("expected container command %s, got %s", e, a)
	}

	// Verify environment merging
	expectedEnv := map[string]string{
		"name": "value",
		"ENV1": "overridden",
	}

	for k, v := range expectedEnv {
		found := false
		for _, env := range gotContainer.Env {
			if env.Name == k && env.Value == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find %s=%s in pod environment", k, v)
		}
	}

	for _, env := range gotContainer.Env {
		val, found := expectedEnv[env.Name]
		if !found || val != env.Value {
			t.Errorf("container has unexpected environment entry %s=%s", env.Name, env.Value)
		}
	}

	// Verify resource limit inheritance
	if cpu := gotContainer.Resources.Limits.Cpu(); cpu.Value() != cpuLimit.Value() {
		t.Errorf("expected cpu %v, got: %v", cpuLimit, cpu)
	}
	if memory := gotContainer.Resources.Limits.Memory(); memory.Value() != memoryLimit.Value() {
		t.Errorf("expected memory %v, got: %v", memoryLimit, memory)
	}

	// Verify restart policy
	if e, a := kapi.RestartPolicyNever, podSpec.Spec.RestartPolicy; e != a {
		t.Fatalf("expected restart policy %s, got %s", e, a)
	}
}
