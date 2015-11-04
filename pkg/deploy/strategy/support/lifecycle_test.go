package support

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/client/cache"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

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
		podClient: &HookExecutorPodClientImpl{
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

	podLogs := &bytes.Buffer{}
	var createdPod *kapi.Pod
	executor := &HookExecutor{
		podClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				createdPod = pod
				return createdPod, nil
			},
			PodWatchFunc: func(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
				createdPod.Status.Phase = kapi.PodSucceeded
				return func() *kapi.Pod { return createdPod }
			},
		},
		podLogDestination: podLogs,
		podLogStream: func(namespace, name string, opts *kapi.PodLogOptions) (io.ReadCloser, error) {
			return ioutil.NopCloser(strings.NewReader("test")), nil
		},
	}

	err := executor.executeExecNewPod(hook, deployment, "hook")

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if e, a := "test", podLogs.String(); e != a {
		t.Fatalf("expected pod logs to be %q, got %q", e, a)
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
		podClient: &HookExecutorPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				createdPod = pod
				return createdPod, nil
			},
			PodWatchFunc: func(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
				createdPod.Status.Phase = kapi.PodFailed
				return func() *kapi.Pod { return createdPod }
			},
		},
		podLogDestination: ioutil.Discard,
		podLogStream: func(namespace, name string, opts *kapi.PodLogOptions) (io.ReadCloser, error) {
			return nil, fmt.Errorf("can't access logs")
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

func TestHookExecutor_makeHookPod(t *testing.T) {
	deploymentName := "deployment-1"
	deploymentNamespace := "test"
	maxDeploymentDurationSeconds := deployapi.MaxDeploymentDurationSeconds

	tests := []struct {
		name     string
		hook     *deployapi.LifecycleHook
		expected *kapi.Pod
	}{
		{
			name: "overrides",
			hook: &deployapi.LifecycleHook{
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
					Volumes: []string{"volume-2"},
				},
			},
			expected: &kapi.Pod{
				ObjectMeta: kapi.ObjectMeta{
					Name: namer.GetPodName(deploymentName, "hook"),
					Labels: map[string]string{
						deployapi.DeployerPodForDeploymentLabel: deploymentName,
					},
					Annotations: map[string]string{
						deployapi.DeploymentAnnotation: deploymentName,
					},
				},
				Spec: kapi.PodSpec{
					RestartPolicy: kapi.RestartPolicyNever,
					Volumes: []kapi.Volume{
						{
							Name: "volume-2",
						},
					},
					ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
					Containers: []kapi.Container{
						{
							Name:    "lifecycle",
							Image:   "registry:8080/repo1:ref1",
							Command: []string{"overridden"},
							Env: []kapi.EnvVar{
								{
									Name:  "name",
									Value: "value",
								},
								{
									Name:  "ENV1",
									Value: "overridden",
								},
								{
									Name:  "OPENSHIFT_DEPLOYMENT_NAME",
									Value: deploymentName,
								},
								{
									Name:  "OPENSHIFT_DEPLOYMENT_NAMESPACE",
									Value: deploymentNamespace,
								},
							},
							Resources: kapi.ResourceRequirements{
								Limits: kapi.ResourceList{
									kapi.ResourceCPU:    resource.MustParse("10"),
									kapi.ResourceMemory: resource.MustParse("10M"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "no overrides",
			hook: &deployapi.LifecycleHook{
				FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
				ExecNewPod: &deployapi.ExecNewPodHook{
					ContainerName: "container1",
				},
			},
			expected: &kapi.Pod{
				ObjectMeta: kapi.ObjectMeta{
					Name: namer.GetPodName(deploymentName, "hook"),
					Labels: map[string]string{
						deployapi.DeployerPodForDeploymentLabel: deploymentName,
					},
					Annotations: map[string]string{
						deployapi.DeploymentAnnotation: deploymentName,
					},
				},
				Spec: kapi.PodSpec{
					RestartPolicy:         kapi.RestartPolicyNever,
					ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
					Containers: []kapi.Container{
						{
							Name:  "lifecycle",
							Image: "registry:8080/repo1:ref1",
							Env: []kapi.EnvVar{
								{
									Name:  "ENV1",
									Value: "VAL1",
								},
								{
									Name:  "OPENSHIFT_DEPLOYMENT_NAME",
									Value: deploymentName,
								},
								{
									Name:  "OPENSHIFT_DEPLOYMENT_NAMESPACE",
									Value: deploymentNamespace,
								},
							},
							Resources: kapi.ResourceRequirements{
								Limits: kapi.ResourceList{
									kapi.ResourceCPU:    resource.MustParse("10"),
									kapi.ResourceMemory: resource.MustParse("10M"),
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Logf("evaluating test: %s", test.name)
		pod, err := makeHookPod(test.hook, deployment("deployment", "test"), "hook")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		for _, c := range pod.Spec.Containers {
			sort.Sort(envByNameAsc(c.Env))
		}
		for _, c := range test.expected.Spec.Containers {
			sort.Sort(envByNameAsc(c.Env))
		}
		if !kapi.Semantic.DeepEqual(pod, test.expected) {
			t.Errorf("unexpected pod diff: %v", kutil.ObjectDiff(pod, test.expected))
		}
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
		acceptedPods := sets.NewString()
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

func deployment(name, namespace string) *kapi.ReplicationController {
	config := &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		LatestVersion: 1,
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
				Resources: kapi.ResourceRequirements{
					Limits: kapi.ResourceList{
						kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
						kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
					},
				},
			},
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{"a": "b"},
				Template: &kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name:  "container1",
								Image: "registry:8080/repo1:ref1",
								Env: []kapi.EnvVar{
									{
										Name:  "ENV1",
										Value: "VAL1",
									},
								},
								ImagePullPolicy: kapi.PullIfNotPresent,
								Resources: kapi.ResourceRequirements{
									Limits: kapi.ResourceList{
										kapi.ResourceCPU:    resource.MustParse("10"),
										kapi.ResourceMemory: resource.MustParse("10M"),
									},
								},
							},
							{
								Name:            "container2",
								Image:           "registry:8080/repo1:ref2",
								ImagePullPolicy: kapi.PullIfNotPresent,
							},
						},
						Volumes: []kapi.Volume{
							{
								Name: "volume-1",
							},
							{
								Name: "volume-2",
							},
						},
						RestartPolicy: kapi.RestartPolicyAlways,
						DNSPolicy:     kapi.DNSClusterFirst,
					},
					ObjectMeta: kapi.ObjectMeta{
						Labels: map[string]string{"a": "b"},
					},
				},
			},
		},
	}
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Namespace = namespace
	return deployment
}

type envByNameAsc []kapi.EnvVar

func (a envByNameAsc) Len() int {
	return len(a)
}
func (a envByNameAsc) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a envByNameAsc) Less(i, j int) bool {
	return a[j].Name < a[i].Name
}
