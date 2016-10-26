package support

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	"github.com/openshift/origin/pkg/util/namer"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"

	_ "github.com/openshift/origin/pkg/api/install"
)

func nowFunc() *unversioned.Time {
	return &unversioned.Time{Time: time.Now().Add(-5 * time.Second)}
}

func newTestClient(config *deployapi.DeploymentConfig) *testclient.Fake {
	client := &testclient.Fake{}
	// when creating a lifecycle pod, we query the deployer pod for the start time to
	// calculate the active deadline seconds for the lifecycle pod.
	client.AddReactor("get", "pods", func(a testclient.Action) (handled bool, ret runtime.Object, err error) {
		action := a.(testclient.GetAction)
		if strings.HasPrefix(action.GetName(), config.Name) && strings.HasSuffix(action.GetName(), "-deploy") {
			return true, &kapi.Pod{
				ObjectMeta: kapi.ObjectMeta{
					Name: "deployer",
				},
				Status: kapi.PodStatus{
					StartTime: nowFunc(),
				},
			}, nil
		}
		return true, nil, nil
	})
	return client
}

func TestHookExecutor_executeExecNewCreatePodFailure(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}
	dc := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(dc, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
	client := newTestClient(dc)
	client.AddReactor("create", "pods", func(a testclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("could not create the pod")
	})
	executor := &HookExecutor{
		pods:    client,
		decoder: kapi.Codecs.UniversalDecoder(),
	}

	if err := executor.executeExecNewPod(hook, deployment, "hook", "test"); err == nil {
		t.Fatalf("expected an error")
	}
}

func TestHookExecutor_executeExecNewPodSucceeded(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
	deployment.Spec.Template.Spec.NodeSelector = map[string]string{"labelKey1": "labelValue1", "labelKey2": "labelValue2"}

	client := newTestClient(config)
	podCreated := make(chan struct{})

	var createdPod *kapi.Pod
	client.AddReactor("create", "pods", func(a testclient.Action) (handled bool, ret runtime.Object, err error) {
		defer close(podCreated)
		action := a.(testclient.CreateAction)
		object := action.GetObject()
		createdPod = object.(*kapi.Pod)
		return true, createdPod, nil
	})
	podsWatch := watch.NewFake()
	client.AddWatchReactor("pods", testclient.DefaultWatchReactor(podsWatch, nil))

	podLogs := &bytes.Buffer{}
	// Simulate creation of the lifecycle pod
	go func() {
		<-podCreated
		podsWatch.Add(createdPod)
		podCopy, _ := kapi.Scheme.Copy(createdPod)
		updatedPod := podCopy.(*kapi.Pod)
		updatedPod.Status.Phase = kapi.PodSucceeded
		podsWatch.Modify(updatedPod)
	}()

	executor := &HookExecutor{
		pods:    client,
		out:     podLogs,
		decoder: kapi.Codecs.UniversalDecoder(),
		getPodLogs: func(*kapi.Pod) (io.ReadCloser, error) {
			return ioutil.NopCloser(strings.NewReader("test")), nil
		},
	}

	err := executor.executeExecNewPod(hook, deployment, "hook", "test")

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if e, a := "--> test: Running hook pod ...\ntest--> test: Success\n", podLogs.String(); e != a {
		t.Fatalf("expected pod logs to be %q, got %q", e, a)
	}

	if e, a := deployment.Spec.Template.Spec.NodeSelector, createdPod.Spec.NodeSelector; !reflect.DeepEqual(e, a) {
		t.Fatalf("expected pod NodeSelector %v, got %v", e, a)
	}

	if createdPod.Spec.ActiveDeadlineSeconds == nil {
		t.Fatalf("expected ActiveDeadlineSeconds to be set on the deployment hook executor pod")
	}

	if *createdPod.Spec.ActiveDeadlineSeconds >= deployapi.MaxDeploymentDurationSeconds {
		t.Fatalf("expected ActiveDeadlineSeconds %+v to be lower than %+v", *createdPod.Spec.ActiveDeadlineSeconds, deployapi.MaxDeploymentDurationSeconds)
	}
}

func TestHookExecutor_executeExecNewPodFailed(t *testing.T) {
	hook := &deployapi.LifecycleHook{
		FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))

	client := newTestClient(config)
	podCreated := make(chan struct{})

	var createdPod *kapi.Pod
	client.AddReactor("create", "pods", func(a testclient.Action) (handled bool, ret runtime.Object, err error) {
		defer close(podCreated)
		action := a.(testclient.CreateAction)
		object := action.GetObject()
		createdPod = object.(*kapi.Pod)
		return true, createdPod, nil
	})
	podsWatch := watch.NewFake()
	client.AddWatchReactor("pods", testclient.DefaultWatchReactor(podsWatch, nil))

	go func() {
		<-podCreated
		podsWatch.Add(createdPod)
		podCopy, _ := kapi.Scheme.Copy(createdPod)
		updatedPod := podCopy.(*kapi.Pod)
		updatedPod.Status.Phase = kapi.PodFailed
		podsWatch.Modify(updatedPod)
	}()

	executor := &HookExecutor{
		pods:    client,
		out:     ioutil.Discard,
		decoder: kapi.Codecs.UniversalDecoder(),
		getPodLogs: func(*kapi.Pod) (io.ReadCloser, error) {
			return ioutil.NopCloser(strings.NewReader("test")), nil
		},
	}

	err := executor.executeExecNewPod(hook, deployment, "hook", "test")
	if err == nil {
		t.Fatalf("expected an error, got none")
	}
	t.Logf("got expected error: %T", err)
}

func TestHookExecutor_makeHookPodInvalidContainerRef(t *testing.T) {
	deployerPod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: "deployer",
		},
		Status: kapi.PodStatus{
			StartTime: nowFunc(),
		},
	}
	hook := &deployapi.LifecycleHook{
		FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &deployapi.ExecNewPodHook{
			ContainerName: "undefined",
		},
	}

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))

	_, err := makeHookPod(hook, deployment, deployerPod, &config.Spec.Strategy, "hook")
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestHookExecutor_makeHookPod(t *testing.T) {
	deploymentName := "deployment-1"
	deploymentNamespace := "test"
	maxDeploymentDurationSeconds := deployapi.MaxDeploymentDurationSeconds
	gracePeriod := int64(10)
	deployerPod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: "deployer",
		},
		Status: kapi.PodStatus{
			StartTime: nowFunc(),
		},
	}

	tests := []struct {
		name                string
		hook                *deployapi.LifecycleHook
		expected            *kapi.Pod
		strategyLabels      map[string]string
		strategyAnnotations map[string]string
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
						deployapi.DeploymentPodTypeLabel:        "hook",
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
							VolumeMounts: []kapi.VolumeMount{
								{
									Name:      "volume-2",
									ReadOnly:  true,
									MountPath: "/mnt/volume-2",
								},
							},
						},
					},
					TerminationGracePeriodSeconds: &gracePeriod,
					ImagePullSecrets: []kapi.LocalObjectReference{
						{
							Name: "secret-1",
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
						deployapi.DeploymentPodTypeLabel:        "hook",
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
					TerminationGracePeriodSeconds: &gracePeriod,
					ImagePullSecrets: []kapi.LocalObjectReference{
						{
							Name: "secret-1",
						},
					},
				},
			},
		},
		{
			name: "labels and annotations",
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
						deployapi.DeploymentPodTypeLabel:        "hook",
						deployapi.DeployerPodForDeploymentLabel: deploymentName,
						"label1": "value1",
					},
					Annotations: map[string]string{
						deployapi.DeploymentAnnotation: deploymentName,
						"annotation2":                  "value2",
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
					TerminationGracePeriodSeconds: &gracePeriod,
					ImagePullSecrets: []kapi.LocalObjectReference{
						{
							Name: "secret-1",
						},
					},
				},
			},
			strategyLabels: map[string]string{
				deployapi.DeployerPodForDeploymentLabel: "ignoredValue",
				"label1": "value1",
			},
			strategyAnnotations: map[string]string{"annotation2": "value2"},
		},
	}

	for _, test := range tests {
		t.Logf("evaluating test: %s", test.name)
		config, deployment := deployment("deployment", "test", test.strategyLabels, test.strategyAnnotations)
		pod, err := makeHookPod(test.hook, deployment, deployerPod, &config.Spec.Strategy, "hook")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		for _, c := range pod.Spec.Containers {
			sort.Sort(envByNameAsc(c.Env))
		}
		for _, c := range test.expected.Spec.Containers {
			sort.Sort(envByNameAsc(c.Env))
		}

		if *pod.Spec.ActiveDeadlineSeconds >= *test.expected.Spec.ActiveDeadlineSeconds {
			t.Errorf("expected pod ActiveDeadlineSeconds %+v to be lower than %+v", *pod.Spec.ActiveDeadlineSeconds, *test.expected.Spec.ActiveDeadlineSeconds)
		}
		// Copy the ActiveDeadlineSeconds the deployer pod is running for 5 seconds already
		test.expected.Spec.ActiveDeadlineSeconds = pod.Spec.ActiveDeadlineSeconds
		if !kapi.Semantic.DeepEqual(pod, test.expected) {
			t.Errorf("unexpected pod diff: %v", diff.ObjectDiff(pod, test.expected))
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
	deployerPod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: "deployer",
		},
		Status: kapi.PodStatus{
			StartTime: nowFunc(),
		},
	}

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))

	pod, err := makeHookPod(hook, deployment, deployerPod, &config.Spec.Strategy, "hook")
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

		acceptorLogs := &bytes.Buffer{}
		acceptor := &AcceptNewlyObservedReadyPods{
			out:      acceptorLogs,
			timeout:  10 * time.Millisecond,
			interval: 1 * time.Millisecond,
			getDeploymentPodStore: func(deployment *kapi.ReplicationController) (cache.Store, chan struct{}) {
				return store, make(chan struct{})
			},
			acceptedPods: acceptedPods,
		}

		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
		deployment.Spec.Replicas = 1

		acceptor.out = &bytes.Buffer{}
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

func deployment(name, namespace string, strategyLabels, strategyAnnotations map[string]string) (*deployapi.DeploymentConfig, *kapi.ReplicationController) {
	config := &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: deployapi.DeploymentConfigStatus{
			LatestVersion: 1,
		},
		Spec: deployapi.DeploymentConfigSpec{
			Replicas: 1,
			Selector: map[string]string{"a": "b"},
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
				Resources: kapi.ResourceRequirements{
					Limits: kapi.ResourceList{
						kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
						kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
					},
				},
				Labels:      strategyLabels,
				Annotations: strategyAnnotations,
			},
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
							VolumeMounts: []kapi.VolumeMount{
								{
									Name:      "volume-2",
									ReadOnly:  true,
									MountPath: "/mnt/volume-2",
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
					ImagePullSecrets: []kapi.LocalObjectReference{
						{
							Name: "secret-1",
						},
					},
				},
				ObjectMeta: kapi.ObjectMeta{
					Labels: map[string]string{"a": "b"},
				},
			},
		},
	}
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
	deployment.Namespace = namespace
	return config, deployment
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
