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

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	appsv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	appsutil "github.com/openshift/origin/pkg/apps/util"

	"github.com/openshift/origin/pkg/api/apihelpers"
	_ "github.com/openshift/origin/pkg/api/install"
)

func nowFunc() *metav1.Time {
	return &metav1.Time{Time: time.Now().Add(-5 * time.Second)}
}

func newTestClient(config *appsapi.DeploymentConfig) *fake.Clientset {
	client := &fake.Clientset{}
	// when creating a lifecycle pod, we query the deployer pod for the start time to
	// calculate the active deadline seconds for the lifecycle pod.
	client.AddReactor("get", "pods", func(a clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		action := a.(clientgotesting.GetAction)
		if strings.HasPrefix(action.GetName(), config.Name) && strings.HasSuffix(action.GetName(), "-deploy") {
			return true, &kapi.Pod{
				ObjectMeta: metav1.ObjectMeta{
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
	hook := &appsapi.LifecycleHook{
		FailurePolicy: appsapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &appsapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}
	dc := appstest.OkDeploymentConfig(1)
	deployment, _ := appsutil.MakeDeployment(dc, legacyscheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion))
	client := newTestClient(dc)
	client.AddReactor("create", "pods", func(a clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("could not create the pod")
	})
	executor := &hookExecutor{
		pods:    client.Core(),
		decoder: legacyscheme.Codecs.UniversalDecoder(),
	}

	if err := executor.executeExecNewPod(hook, deployment, "hook", "test"); err == nil {
		t.Fatalf("expected an error")
	}
}

func TestHookExecutor_executeExecNewPodSucceeded(t *testing.T) {
	hook := &appsapi.LifecycleHook{
		FailurePolicy: appsapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &appsapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	config := appstest.OkDeploymentConfig(1)
	deployment, _ := appsutil.MakeDeployment(config, legacyscheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion))
	deployment.Spec.Template.Spec.NodeSelector = map[string]string{"labelKey1": "labelValue1", "labelKey2": "labelValue2"}

	client := newTestClient(config)
	podCreated := make(chan struct{})

	var createdPod *kapi.Pod
	client.AddReactor("create", "pods", func(a clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		defer close(podCreated)
		action := a.(clientgotesting.CreateAction)
		object := action.GetObject()
		createdPod = object.(*kapi.Pod)
		return true, createdPod, nil
	})
	podsWatch := watch.NewFake()
	client.AddWatchReactor("pods", clientgotesting.DefaultWatchReactor(podsWatch, nil))

	podLogs := &bytes.Buffer{}
	// Simulate creation of the lifecycle pod
	go func() {
		<-podCreated
		podsWatch.Add(createdPod)
		updatedPod := createdPod.DeepCopy()
		updatedPod.Status.Phase = kapi.PodSucceeded
		podsWatch.Modify(updatedPod)
	}()

	executor := &hookExecutor{
		pods:    client.Core(),
		out:     podLogs,
		decoder: legacyscheme.Codecs.UniversalDecoder(),
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

	if *createdPod.Spec.ActiveDeadlineSeconds >= appsapi.MaxDeploymentDurationSeconds {
		t.Fatalf("expected ActiveDeadlineSeconds %+v to be lower than %+v", *createdPod.Spec.ActiveDeadlineSeconds, appsapi.MaxDeploymentDurationSeconds)
	}
}

func TestHookExecutor_executeExecNewPodFailed(t *testing.T) {
	hook := &appsapi.LifecycleHook{
		FailurePolicy: appsapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &appsapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	config := appstest.OkDeploymentConfig(1)
	deployment, _ := appsutil.MakeDeployment(config, legacyscheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion))

	client := newTestClient(config)
	podCreated := make(chan struct{})

	var createdPod *kapi.Pod
	client.AddReactor("create", "pods", func(a clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		defer close(podCreated)
		action := a.(clientgotesting.CreateAction)
		object := action.GetObject()
		createdPod = object.(*kapi.Pod)
		return true, createdPod, nil
	})
	podsWatch := watch.NewFake()
	client.AddWatchReactor("pods", clientgotesting.DefaultWatchReactor(podsWatch, nil))

	go func() {
		<-podCreated
		podsWatch.Add(createdPod)
		updatedPod := createdPod.DeepCopy()
		updatedPod.Status.Phase = kapi.PodFailed
		podsWatch.Modify(updatedPod)
	}()

	executor := &hookExecutor{
		pods:    client.Core(),
		out:     ioutil.Discard,
		decoder: legacyscheme.Codecs.UniversalDecoder(),
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
	hook := &appsapi.LifecycleHook{
		FailurePolicy: appsapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &appsapi.ExecNewPodHook{
			ContainerName: "undefined",
		},
	}

	config := appstest.OkDeploymentConfig(1)
	deployment, _ := appsutil.MakeDeployment(config, legacyscheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion))

	_, err := makeHookPod(hook, deployment, &config.Spec.Strategy, "hook", nowFunc().Time)
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestHookExecutor_makeHookPod(t *testing.T) {
	deploymentName := "deployment-1"
	deploymentNamespace := "test"
	maxDeploymentDurationSeconds := appsapi.MaxDeploymentDurationSeconds
	gracePeriod := int64(10)

	tests := []struct {
		name                string
		hook                *appsapi.LifecycleHook
		expected            *kapi.Pod
		strategyLabels      map[string]string
		strategyAnnotations map[string]string
	}{
		{
			name: "overrides",
			hook: &appsapi.LifecycleHook{
				FailurePolicy: appsapi.LifecycleHookFailurePolicyAbort,
				ExecNewPod: &appsapi.ExecNewPodHook{
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
				ObjectMeta: metav1.ObjectMeta{
					Name: apihelpers.GetPodName(deploymentName, "hook"),
					Labels: map[string]string{
						appsapi.DeploymentPodTypeLabel:        "hook",
						appsapi.DeployerPodForDeploymentLabel: deploymentName,
					},
					Annotations: map[string]string{
						appsapi.DeploymentAnnotation: deploymentName,
					},
				},
				Spec: kapi.PodSpec{
					RestartPolicy: kapi.RestartPolicyNever,
					Volumes: []kapi.Volume{
						{
							Name: "volume-2",
							VolumeSource: kapi.VolumeSource{
								EmptyDir: &kapi.EmptyDirVolumeSource{},
							},
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
					},
					TerminationGracePeriodSeconds: &gracePeriod,
					ImagePullSecrets: []kapi.LocalObjectReference{
						{
							Name: "secret-1",
						},
					},
					SecurityContext: &kapi.PodSecurityContext{},
				},
			},
		},
		{
			name: "no overrides",
			hook: &appsapi.LifecycleHook{
				FailurePolicy: appsapi.LifecycleHookFailurePolicyAbort,
				ExecNewPod: &appsapi.ExecNewPodHook{
					ContainerName: "container1",
				},
			},
			expected: &kapi.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: apihelpers.GetPodName(deploymentName, "hook"),
					Labels: map[string]string{
						appsapi.DeploymentPodTypeLabel:        "hook",
						appsapi.DeployerPodForDeploymentLabel: deploymentName,
					},
					Annotations: map[string]string{
						appsapi.DeploymentAnnotation: deploymentName,
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
							ImagePullPolicy: kapi.PullIfNotPresent,
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
					SecurityContext: &kapi.PodSecurityContext{},
				},
			},
		},
		{
			name: "labels and annotations",
			hook: &appsapi.LifecycleHook{
				FailurePolicy: appsapi.LifecycleHookFailurePolicyAbort,
				ExecNewPod: &appsapi.ExecNewPodHook{
					ContainerName: "container1",
				},
			},
			expected: &kapi.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: apihelpers.GetPodName(deploymentName, "hook"),
					Labels: map[string]string{
						appsapi.DeploymentPodTypeLabel:        "hook",
						appsapi.DeployerPodForDeploymentLabel: deploymentName,
						"label1": "value1",
					},
					Annotations: map[string]string{
						appsapi.DeploymentAnnotation: deploymentName,
						"annotation2":                "value2",
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
							ImagePullPolicy: kapi.PullIfNotPresent,
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
					SecurityContext: &kapi.PodSecurityContext{},
				},
			},
			strategyLabels: map[string]string{
				appsapi.DeployerPodForDeploymentLabel: "ignoredValue",
				"label1": "value1",
			},
			strategyAnnotations: map[string]string{"annotation2": "value2"},
		},
		{
			name: "allways pull image",
			hook: &appsapi.LifecycleHook{
				FailurePolicy: appsapi.LifecycleHookFailurePolicyAbort,
				ExecNewPod: &appsapi.ExecNewPodHook{
					ContainerName: "container2",
				},
			},
			expected: &kapi.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: apihelpers.GetPodName(deploymentName, "hook"),
					Labels: map[string]string{
						appsapi.DeploymentPodTypeLabel:        "hook",
						appsapi.DeployerPodForDeploymentLabel: deploymentName,
					},
					Annotations: map[string]string{
						appsapi.DeploymentAnnotation: deploymentName,
					},
				},
				Spec: kapi.PodSpec{
					RestartPolicy:         kapi.RestartPolicyNever,
					ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
					Containers: []kapi.Container{
						{
							Name:  "lifecycle",
							Image: "registry:8080/repo1:ref2",
							Env: []kapi.EnvVar{
								{
									Name:  "OPENSHIFT_DEPLOYMENT_NAME",
									Value: deploymentName,
								},
								{
									Name:  "OPENSHIFT_DEPLOYMENT_NAMESPACE",
									Value: deploymentNamespace,
								},
							},
							ImagePullPolicy: kapi.PullAlways,
						},
					},
					TerminationGracePeriodSeconds: &gracePeriod,
					ImagePullSecrets: []kapi.LocalObjectReference{
						{
							Name: "secret-1",
						},
					},
					SecurityContext: &kapi.PodSecurityContext{},
				},
			},
		},
	}

	for _, test := range tests {
		t.Logf("evaluating test: %s", test.name)
		config, deployment := deployment("deployment", "test", test.strategyLabels, test.strategyAnnotations)
		pod, err := makeHookPod(test.hook, deployment, &config.Spec.Strategy, "hook", nowFunc().Time)
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
		if !kapihelper.Semantic.DeepEqual(pod, test.expected) {
			t.Errorf("unexpected pod diff: %v", diff.ObjectReflectDiff(pod, test.expected))
		}
	}
}

func TestHookExecutor_makeHookPodRestart(t *testing.T) {
	hook := &appsapi.LifecycleHook{
		FailurePolicy: appsapi.LifecycleHookFailurePolicyRetry,
		ExecNewPod: &appsapi.ExecNewPodHook{
			ContainerName: "container1",
		},
	}

	config := appstest.OkDeploymentConfig(1)
	deployment, _ := appsutil.MakeDeployment(config, legacyscheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion))

	pod, err := makeHookPod(hook, deployment, &config.Spec.Strategy, "hook", nowFunc().Time)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if e, a := kapi.RestartPolicyOnFailure, pod.Spec.RestartPolicy; e != a {
		t.Errorf("expected pod restart policy %s, got %s", e, a)
	}
}

func deployment(name, namespace string, strategyLabels, strategyAnnotations map[string]string) (*appsapi.DeploymentConfig, *kapi.ReplicationController) {
	config := &appsapi.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: appsapi.DeploymentConfigStatus{
			LatestVersion: 1,
		},
		Spec: appsapi.DeploymentConfigSpec{
			Replicas: 1,
			Selector: map[string]string{"a": "b"},
			Strategy: appsapi.DeploymentStrategy{
				Type: appsapi.DeploymentStrategyTypeRecreate,
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
							ImagePullPolicy: kapi.PullAlways,
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
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "b"},
				},
			},
		},
	}
	deployment, _ := appsutil.MakeDeployment(config, legacyscheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion))
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
