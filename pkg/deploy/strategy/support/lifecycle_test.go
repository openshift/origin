package support

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/client/cache"
	kutil "k8s.io/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	namer "github.com/openshift/origin/pkg/util/namer"
)

func TestHookExecutor_executeExecNewCreatePodFailure(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, fmt.Errorf("couldn't create pod")
			},
			PodWatchFunc: func(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
				return func() *kapi.Pod { return nil }
			},
		},
	}

	err := executor.executeExecNewPod(hook, deployment, "hook")

	if err == nil {
		t.Fatalf("expected an error")
	}
	t.Logf("got expected error: %s", err)
}

func TestHookExecutor_executeExecNewPodSucceeded(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Spec.Template.Spec.NodeSelector = map[string]string{"labelKey1": "labelValue1", "labelKey2": "labelValue2"}

	var createdPod *kapi.Pod
	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				createdPod = pod
				return createdPod, nil
			},
			PodWatchFunc: func(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
				createdPod.Status.Phase = kapi.PodSucceeded
				return func() *kapi.Pod { return createdPod }
			},
		},
	}

	err := executor.executeExecNewPod(hook, deployment, "hook")

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if e, a := deployment.Spec.Template.Spec.NodeSelector, createdPod.Spec.NodeSelector; !reflect.DeepEqual(e, a) {
		t.Fatalf("expected pod NodeSelector %v, got %v", e, a)
	}

	if createdPod.Spec.ActiveDeadlineSeconds == nil {
		t.Fatalf("expected ActiveDeadlineSeconds to be set on the deployment hook executor pod")
	}

	if *createdPod.Spec.ActiveDeadlineSeconds != deployapi.MaxDeploymentDurationSeconds {
		t.Fatalf("expected ActiveDeadlineSeconds to be set to %d; found: %d", deployapi.MaxDeploymentDurationSeconds, *createdPod.Spec.ActiveDeadlineSeconds)
	}
}

func TestHookExecutor_executeExecNewPodFailed(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	var createdPod *kapi.Pod
	executor := &HookExecutor{
		PodClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				createdPod = pod
				return createdPod, nil
			},
			PodWatchFunc: func(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
				createdPod.Status.Phase = kapi.PodFailed
				return func() *kapi.Pod { return createdPod }
			},
		},
	}

	err := executor.executeExecNewPod(hook, deployment, "hook")

	if err == nil {
		t.Fatalf("expected an error, got none")
	}
	t.Logf("got expected error: %s", err)
}

func TestHookExecutor_makeHookPodInvalidContainerRef(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "undefined",
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	_, err := makeHookPod(hook, deployment, "hook")

	if err == nil {
		t.Fatalf("expected an error")
	}
	t.Logf("got expected error: %s", err)
}

func TestHookExecutor_makeHookPodOk(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
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

	pod, err := makeHookPod(hook, deployment, "hook")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if e, a := namer.GetPodName(deployment.Name, "hook"), pod.Name; e != a {
		t.Errorf("expected pod name %s, got %s", e, a)
	}

	if e, a := kapi.RestartPolicyNever, pod.Spec.RestartPolicy; e != a {
		t.Errorf("expected pod restart policy %s, got %s", e, a)
	}

	gotContainer := pod.Spec.Containers[0]

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
		"OPENSHIFT_DEPLOYMENT_NAME":      deployment.Name,
		"OPENSHIFT_DEPLOYMENT_NAMESPACE": deployment.Namespace,
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
	if e, a := kapi.RestartPolicyNever, pod.Spec.RestartPolicy; e != a {
		t.Fatalf("expected restart policy %s, got %s", e, a)
	}

	// Verify correlation stuff
	if l, e, a := deployapi.DeployerPodForDeploymentLabel,
		deployment.Name,
		pod.Labels[deployapi.DeployerPodForDeploymentLabel]; e != a {
		t.Errorf("expected label %s=%s, got %s", l, e, a)
	}
	if l, e, a := deployapi.DeploymentAnnotation,
		deployment.Name,
		pod.Annotations[deployapi.DeploymentAnnotation]; e != a {
		t.Errorf("expected annotation %s=%s, got %s", l, e, a)
	}
}

func TestHookExecutor_makeHookPodRestart(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		FailurePolicy: deployapi.LifecycleHookFailurePolicyRetry,
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	pod, err := makeHookPod(hook, deployment, "hook")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if e, a := kapi.RestartPolicyOnFailure, pod.Spec.RestartPolicy; e != a {
		t.Errorf("expected pod restart policy %s, got %s", e, a)
	}
}

func TestAcceptNewlyObservedReadyPods_scenarios(t *testing.T) {
	scenarios := []struct {
		name string
		// any pods which are previously accepted
		acceptedPods []string
		// the current pods which will be in the store; pod name -> ready
		currentPods map[string]bool
		// whether or not the scenario should result in acceptance
		accepted bool
	}{
		{
			name:         "all ready, none previously accepted",
			accepted:     true,
			acceptedPods: []string{},
			currentPods: map[string]bool{
				"pod-1": true,
				"pod-2": true,
			},
		},
		{
			name:         "some ready, none previously accepted",
			accepted:     false,
			acceptedPods: []string{},
			currentPods: map[string]bool{
				"pod-1": false,
				"pod-2": true,
			},
		},
		{
			name:         "previously accepted has become unready, new are ready",
			accepted:     true,
			acceptedPods: []string{"pod-1"},
			currentPods: map[string]bool{
				// this pod should be ignored because it was previously accepted
				"pod-1": false,
				"pod-2": true,
			},
		},
		{
			name:         "previously accepted all ready, new is unready",
			accepted:     false,
			acceptedPods: []string{"pod-1"},
			currentPods: map[string]bool{
				"pod-1": true,
				"pod-2": false,
			},
		},
	}
	for _, s := range scenarios {
		t.Logf("running scenario: %s", s.name)

		// Populate the store with real pods with the desired ready condition.
		store := cache.NewStore(cache.MetaNamespaceKeyFunc)
		for podName, ready := range s.currentPods {
			status := kapi.ConditionTrue
			if !ready {
				status = kapi.ConditionFalse
			}
			pod := &kapi.Pod{
				ObjectMeta: kapi.ObjectMeta{
					Name: podName,
				},
				Status: kapi.PodStatus{
					Conditions: []kapi.PodCondition{
						{
							Type:   kapi.PodReady,
							Status: status,
						},
					},
				},
			}
			store.Add(pod)
		}

		// Set up accepted pods for the scenario.
		acceptedPods := kutil.NewStringSet()
		for _, podName := range s.acceptedPods {
			acceptedPods.Insert(podName)
		}

		acceptor := &AcceptNewlyObservedReadyPods{
			timeout:  10 * time.Millisecond,
			interval: 1 * time.Millisecond,
			getDeploymentPodStore: func(deployment *kapi.ReplicationController) (cache.Store, chan struct{}) {
				return store, make(chan struct{})
			},
			acceptedPods: acceptedPods,
		}

		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
		deployment.Spec.Replicas = 1

		err := acceptor.Accept(deployment)

		if s.accepted {
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		} else {
			if err == nil {
				t.Fatalf("expected an error")
			}
			t.Logf("got expected error: %s", err)
		}
	}
}
