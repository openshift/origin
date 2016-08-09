package deployment

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

var (
	env   = []kapi.EnvVar{{Name: "ENV1", Value: "VAL1"}}
	codec = kapi.Codecs.LegacyCodec(deployapiv1.SchemeGroupVersion)
)

func okDeploymentController(fake kclient.Interface, deployment *kapi.ReplicationController, hookPodNames []string, related bool, deployerStatus kapi.PodPhase) *DeploymentController {
	rcInformer := framework.NewSharedIndexInformer(&cache.ListWatch{}, &kapi.ReplicationController{}, 2*time.Minute, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	podInformer := framework.NewSharedIndexInformer(&cache.ListWatch{}, &kapi.Pod{}, 2*time.Minute, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	c := NewDeploymentController(rcInformer, podInformer, fake, "sa:test", "openshift/origin-deployer", env, codec)

	// deployer pod
	if deployment != nil {
		pod := deployerPod(deployment, "", related)
		pod.Status.Phase = deployerStatus
		c.podStore.Add(pod)
	}

	// hook pods
	for _, name := range hookPodNames {
		pod := deployerPod(deployment, name, related)
		c.podStore.Add(pod)
	}

	return c
}

func deployerPod(deployment *kapi.ReplicationController, alternateName string, related bool) *kapi.Pod {
	deployerPodName := deployutil.DeployerPodNameForDeployment(deployment.Name)
	if len(alternateName) > 0 {
		deployerPodName = alternateName
	}

	deployment.Namespace = "test"

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:      deployerPodName,
			Namespace: deployment.Namespace,
			Labels: map[string]string{
				deployapi.DeployerPodForDeploymentLabel: deployment.Name,
			},
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
		},
	}

	if !related {
		delete(pod.Annotations, deployapi.DeploymentAnnotation)
	}

	return pod
}

func okContainer() *kapi.Container {
	return &kapi.Container{
		Image:   "openshift/origin-deployer",
		Command: []string{"/bin/echo", "hello", "world"},
		Env:     env,
		Resources: kapi.ResourceRequirements{
			Limits: kapi.ResourceList{
				kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
				kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
			},
		},
	}
}

// TestHandle_createPodOk ensures that a deployer pod created in response
// to a new deployment is valid.
func TestHandle_createPodOk(t *testing.T) {
	var (
		updatedDeployment *kapi.ReplicationController
		createdPod        *kapi.Pod
		expectedContainer = okContainer()
	)

	fake := &ktestclient.Fake{}
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		pod := action.(ktestclient.CreateAction).GetObject().(*kapi.Pod)
		createdPod = pod
		return true, pod, nil
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	// Verify new -> pending
	config := deploytest.OkDeploymentConfig(1)
	config.Spec.Strategy = deploytest.OkCustomStrategy()
	deployment, _ := deployutil.MakeDeployment(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	deployment.Spec.Template.Spec.NodeSelector = map[string]string{"labelKey1": "labelValue1", "labelKey2": "labelValue2"}

	controller := okDeploymentController(fake, nil, nil, true, kapi.PodUnknown)

	if err := controller.Handle(deployment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatalf("expected an updated deployment")
	}

	if e, a := deployapi.DeploymentStatusPending, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}

	if createdPod == nil {
		t.Fatalf("expected a pod to be created")
	}

	if e := deployutil.DeployerPodNameFor(updatedDeployment); len(e) == 0 {
		t.Fatalf("missing deployment pod annotation")
	}

	if e, a := createdPod.Name, deployutil.DeployerPodNameFor(updatedDeployment); e != a {
		t.Fatalf("expected deployment pod annotation %s, got %s", e, a)
	}

	if e := deployutil.DeploymentNameFor(createdPod); len(e) == 0 {
		t.Fatalf("missing deployment annotation")
	}

	if e, a := updatedDeployment.Name, deployutil.DeploymentNameFor(createdPod); e != a {
		t.Fatalf("expected pod deployment annotation %s, got %s", e, a)
	}

	if e, a := deployment.Spec.Template.Spec.NodeSelector, createdPod.Spec.NodeSelector; !reflect.DeepEqual(e, a) {
		t.Fatalf("expected pod NodeSelector %v, got %v", e, a)
	}

	if createdPod.Spec.ActiveDeadlineSeconds == nil {
		t.Fatalf("expected ActiveDeadlineSeconds to be set on the deployer pod")
	}

	if *createdPod.Spec.ActiveDeadlineSeconds != deployapi.MaxDeploymentDurationSeconds {
		t.Fatalf("expected ActiveDeadlineSeconds on the deployer pod to be set to %d; found: %d", deployapi.MaxDeploymentDurationSeconds, *createdPod.Spec.ActiveDeadlineSeconds)
	}

	actualContainer := createdPod.Spec.Containers[0]

	if e, a := expectedContainer.Image, actualContainer.Image; e != a {
		t.Fatalf("expected container image %s, got %s", expectedContainer.Image, actualContainer.Image)
	}

	if e, a := expectedContainer.Command[0], actualContainer.Command[0]; e != a {
		t.Fatalf("expected container command %s, got %s", expectedContainer.Command[0], actualContainer.Command[0])
	}

	if e, a := expectedContainer.Env[0].Name, actualContainer.Env[0].Name; e != a {
		t.Fatalf("expected container env name %s, got %s", expectedContainer.Env[0].Name, actualContainer.Env[0].Name)
	}

	if e, a := expectedContainer.Env[0].Value, actualContainer.Env[0].Value; e != a {
		t.Fatalf("expected container env value %s, got %s", expectedContainer.Env[0].Value, actualContainer.Env[0].Value)
	}

	if e, a := expectedContainer.Resources, actualContainer.Resources; !kapi.Semantic.DeepEqual(e, a) {
		t.Fatalf("expected container resources %v, got %v", expectedContainer.Resources, actualContainer.Resources)
	}
}

// TestHandle_createPodFail ensures that an API failure while creating a
// deployer pod results in a nonfatal error.
func TestHandle_createPodFail(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	fake := &ktestclient.Fake{}
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(ktestclient.CreateAction).GetObject().(*kapi.Pod).Name
		return true, nil, fmt.Errorf("failed to create pod %q", name)
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)

	controller := okDeploymentController(fake, nil, nil, true, kapi.PodUnknown)

	err := controller.Handle(deployment)
	if err == nil {
		t.Fatalf("expected an error")
	}

	if _, isFatal := err.(fatalError); isFatal {
		t.Fatalf("expected a nonfatal error, got a %#v", err)
	}
}

// TestHandle_deployerPodAlreadyExists ensures that attempts to create a
// deployer pod which was already created don't result in an error
// (effectively skipping the handling as redundant).
func TestHandle_deployerPodAlreadyExists(t *testing.T) {
	tests := []struct {
		name string

		podPhase kapi.PodPhase
		expected deployapi.DeploymentStatus
	}{
		{
			name: "pending",

			podPhase: kapi.PodPending,
			expected: deployapi.DeploymentStatusPending,
		},
		{
			name: "running",

			podPhase: kapi.PodRunning,
			expected: deployapi.DeploymentStatusRunning,
		},
		{
			name: "complete",

			podPhase: kapi.PodFailed,
			expected: deployapi.DeploymentStatusFailed,
		},
		{
			name: "failed",

			podPhase: kapi.PodSucceeded,
			expected: deployapi.DeploymentStatusComplete,
		},
	}

	for _, test := range tests {
		var updatedDeployment *kapi.ReplicationController

		config := deploytest.OkDeploymentConfig(1)
		deployment, _ := deployutil.MakeDeployment(config, codec)
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
		deployerPodName := deployutil.DeployerPodNameForDeployment(deployment.Name)

		fake := &ktestclient.Fake{}
		fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			name := action.(ktestclient.CreateAction).GetObject().(*kapi.Pod).Name
			return true, nil, kerrors.NewAlreadyExists(kapi.Resource("Pod"), name)
		})
		fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
			updatedDeployment = rc
			return true, rc, nil
		})

		controller := okDeploymentController(fake, deployment, nil, true, test.podPhase)

		if err := controller.Handle(deployment); err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		if updatedDeployment.Annotations[deployapi.DeploymentPodAnnotation] != deployerPodName {
			t.Errorf("%s: deployment not updated with pod name annotation", test.name)
			continue
		}

		if e, a := string(test.expected), updatedDeployment.Annotations[deployapi.DeploymentStatusAnnotation]; e != a {
			t.Errorf("%s: deployment status not updated. Expected %q, got %q", test.name, e, a)
		}
	}
}

// TestHandle_unrelatedPodAlreadyExists ensures that attempts to create a
// deployer pod, when a pod with the same name but missing annotations results
// a transition to failed.
func TestHandle_unrelatedPodAlreadyExists(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)

	fake := &ktestclient.Fake{}
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(ktestclient.CreateAction).GetObject().(*kapi.Pod).Name
		return true, nil, kerrors.NewAlreadyExists(kapi.Resource("Pod"), name)
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	controller := okDeploymentController(fake, deployment, nil, false, kapi.PodRunning)

	if err := controller.Handle(deployment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, exists := updatedDeployment.Annotations[deployapi.DeploymentPodAnnotation]; exists {
		t.Fatalf("deployment updated with pod name annotation")
	}

	if e, a := deployapi.DeploymentFailedUnrelatedDeploymentExists, updatedDeployment.Annotations[deployapi.DeploymentStatusReasonAnnotation]; e != a {
		t.Fatalf("expected reason annotation %s, got %s", e, a)
	}

	if e, a := deployapi.DeploymentStatusFailed, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected deployment status %s, got %s", e, a)
	}
}

// TestHandle_unrelatedPodAlreadyExistsTestScaled ensures that attempts to create a
// deployer pod, when a pod with the same name but be scaled to zero results
// a transition to failed.
func TestHandle_unrelatedPodAlreadyExistsTestScaled(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	config := deploytest.TestDeploymentConfig(deploytest.OkDeploymentConfig(1))
	deployment, _ := deployutil.MakeDeployment(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	deployment.Spec.Replicas = 1

	fake := &ktestclient.Fake{}
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(ktestclient.CreateAction).GetObject().(*kapi.Pod).Name
		return true, nil, kerrors.NewAlreadyExists(kapi.Resource("Pod"), name)
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	controller := okDeploymentController(fake, deployment, nil, false, kapi.PodRunning)

	if err := controller.Handle(deployment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, exists := updatedDeployment.Annotations[deployapi.DeploymentPodAnnotation]; exists {
		t.Fatalf("deployment updated with pod name annotation")
	}

	if e, a := deployapi.DeploymentFailedUnrelatedDeploymentExists, updatedDeployment.Annotations[deployapi.DeploymentStatusReasonAnnotation]; e != a {
		t.Fatalf("expected reason annotation %s, got %s", e, a)
	}

	if e, a := deployapi.DeploymentStatusFailed, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected deployment status %s, got %s", e, a)
	}
	if e, a := int32(0), updatedDeployment.Spec.Replicas; e != a {
		t.Fatalf("expected failed deployment to be scaled to zero: %d", a)
	}
}

// TestHandle_noop ensures that pending, running, and failed states result in
// no action by the controller (as long as the deployment hasn't been cancelled
// and the deployer pod status is synced with the deployment status).
func TestHandle_noop(t *testing.T) {
	tests := []struct {
		name string

		podPhase        kapi.PodPhase
		deploymentPhase deployapi.DeploymentStatus
	}{
		{
			name: "pending",

			podPhase:        kapi.PodPending,
			deploymentPhase: deployapi.DeploymentStatusPending,
		},
		{
			name: "running",

			podPhase:        kapi.PodRunning,
			deploymentPhase: deployapi.DeploymentStatusRunning,
		},
		{
			name: "complete",

			podPhase:        kapi.PodFailed,
			deploymentPhase: deployapi.DeploymentStatusFailed,
		},
	}

	for _, test := range tests {
		fake := &ktestclient.Fake{}

		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(test.deploymentPhase)

		controller := okDeploymentController(fake, deployment, nil, true, test.podPhase)

		if err := controller.Handle(deployment); err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		if len(fake.Actions()) > 0 {
			t.Errorf("%s: unexpected actions: %v", test.name, fake.Actions())
		}
	}
}

// TestHandle_failedTest ensures that failed test deployments have their
// replicas set to zero.
func TestHandle_failedTest(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	fake := &ktestclient.Fake{}
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	// Verify successful cleanup
	config := deploytest.TestDeploymentConfig(deploytest.OkDeploymentConfig(1))
	deployment, _ := deployutil.MakeDeployment(config, codec)
	deployment.Spec.Replicas = 1
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)

	controller := okDeploymentController(fake, deployment, nil, true, kapi.PodFailed)

	if err := controller.Handle(deployment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatal("deployment not updated")
	}
	if e, a := int32(0), updatedDeployment.Spec.Replicas; e != a {
		t.Fatalf("expected updated deployment replicas to be %d, got %d", e, a)
	}
}

// TestHandle_cleanupPodOk ensures that deployer pods are cleaned up for
// deployments in a completed state.
func TestHandle_cleanupPodOk(t *testing.T) {
	hookPods := []string{"pre", "mid", "post"}
	deletedPodNames := []string{}

	fake := &ktestclient.Fake{}
	fake.AddReactor("delete", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(ktestclient.DeleteAction).GetName()
		deletedPodNames = append(deletedPodNames, name)
		return true, nil, nil
	})
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected deployment update")
		return true, nil, nil
	})

	// Verify successful cleanup
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)

	controller := okDeploymentController(fake, deployment, hookPods, true, kapi.PodSucceeded)
	hookPods = append(hookPods, deployment.Name)

	if err := controller.Handle(deployment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(hookPods)
	sort.Strings(deletedPodNames)
	if !reflect.DeepEqual(deletedPodNames, deletedPodNames) {
		t.Fatalf("pod deletions - expected: %v, actual: %v", hookPods, deletedPodNames)
	}

}

// TestHandle_cleanupPodOkTest ensures that deployer pods are cleaned up for
// deployments in a completed state on test deployment configs, and
// replicas is set back to zero.
func TestHandle_cleanupPodOkTest(t *testing.T) {
	hookPods := []string{"pre", "post"}
	deletedPodNames := []string{}
	var updatedDeployment *kapi.ReplicationController

	fake := &ktestclient.Fake{}
	fake.AddReactor("delete", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(ktestclient.DeleteAction).GetName()
		deletedPodNames = append(deletedPodNames, name)
		return true, nil, nil
	})
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	// Verify successful cleanup
	config := deploytest.TestDeploymentConfig(deploytest.OkDeploymentConfig(1))
	deployment, _ := deployutil.MakeDeployment(config, codec)
	deployment.Spec.Replicas = 1
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)

	controller := okDeploymentController(fake, deployment, hookPods, true, kapi.PodSucceeded)
	hookPods = append(hookPods, deployment.Name)

	if err := controller.Handle(deployment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(hookPods)
	sort.Strings(deletedPodNames)
	if !reflect.DeepEqual(deletedPodNames, deletedPodNames) {
		t.Fatalf("pod deletions - expected: %v, actual: %v", hookPods, deletedPodNames)
	}
	if updatedDeployment == nil {
		t.Fatal("deployment not updated")
	}
	if e, a := int32(0), updatedDeployment.Spec.Replicas; e != a {
		t.Fatalf("expected updated deployment replicas to be %d, got %d", e, a)
	}
}

// TestHandle_cleanupPodNoop ensures that an attempt to delete pods is not made
// if the deployer pods are not listed based on a label query
func TestHandle_cleanupPodNoop(t *testing.T) {
	fake := &ktestclient.Fake{}
	fake.AddReactor("delete", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to delete pod")
		return true, nil, nil
	})
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected deployment update")
		return true, nil, nil
	})

	// Verify no-op
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)

	controller := okDeploymentController(fake, deployment, nil, true, kapi.PodSucceeded)
	pod := deployerPod(deployment, "", true)
	pod.Labels[deployapi.DeployerPodForDeploymentLabel] = "unrelated"
	controller.podStore.Update(pod)

	if err := controller.Handle(deployment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_cleanupPodFail ensures that a failed attempt to clean up the
// deployer pod for a completed deployment results in an actionable error.
func TestHandle_cleanupPodFail(t *testing.T) {
	fake := &ktestclient.Fake{}
	fake.AddReactor("delete", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kerrors.NewInternalError(fmt.Errorf("deployer pod internal error"))
	})
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected deployment update")
		return true, nil, nil
	})

	// Verify error
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)

	controller := okDeploymentController(fake, deployment, nil, true, kapi.PodSucceeded)

	err := controller.Handle(deployment)
	if err == nil {
		t.Fatal("expected an actionable error")
	}
	if _, isActionable := err.(actionableError); !isActionable {
		t.Fatalf("expected an actionable error, got %#v", err)
	}
}

// TestHandle_cancelNew ensures that a New cancelled deployment will be transitioned
// to Pending even if the deployer pod is Running.
func TestHandle_cancelNew(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	fake := &ktestclient.Fake{}
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue

	controller := okDeploymentController(fake, deployment, nil, true, kapi.PodRunning)

	if err := controller.Handle(deployment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e, a := deployapi.DeploymentStatusPending, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected deployment status %s, got %s", e, a)
	}
}

// TestHandle_cleanupNewWithDeployers ensures that we will try to cleanup deployer pods
// for a cancelled deployment.
func TestHandle_cleanupNewWithDeployers(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController
	deletedDeployer := false

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue

	fake := &ktestclient.Fake{}
	fake.AddReactor("delete", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		deletedDeployer = true
		return true, nil, nil
	})
	fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		updatedDeployment = rc
		return true, nil, nil
	})

	controller := okDeploymentController(fake, deployment, nil, true, kapi.PodRunning)

	if err := controller.Handle(deployment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e, a := deployapi.DeploymentStatusPending, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected deployment status %s, got %s", e, a)
	}
	if !deletedDeployer {
		t.Fatalf("expected deployer delete")
	}
}

// TestHandle_cleanupPostNew ensures that deployer pods are deleted
// for cancelled deployments in all post-New phases.
func TestHandle_cleanupPostNew(t *testing.T) {
	hookPods := []string{"pre", "post"}

	tests := []struct {
		name string

		deploymentPhase deployapi.DeploymentStatus
		podPhase        kapi.PodPhase

		expected int
	}{
		{
			name: "pending",

			deploymentPhase: deployapi.DeploymentStatusPending,
			podPhase:        kapi.PodPending,

			expected: len(hookPods) + 1,
		},
		{
			name: "running",

			deploymentPhase: deployapi.DeploymentStatusRunning,
			podPhase:        kapi.PodRunning,

			expected: len(hookPods) + 1,
		},
		{
			name: "failed",

			deploymentPhase: deployapi.DeploymentStatusFailed,
			podPhase:        kapi.PodFailed,

			expected: len(hookPods) + 1,
		},
		{
			name: "complete",

			deploymentPhase: deployapi.DeploymentStatusComplete,
			podPhase:        kapi.PodSucceeded,

			expected: len(hookPods) + 1,
		},
	}

	for _, test := range tests {
		deletedPods := 0

		fake := &ktestclient.Fake{}
		fake.AddReactor("delete", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			deletedPods++
			return true, nil, nil
		})
		fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			// None of these tests should transition the phase.
			t.Errorf("%s: unexpected call to update a deployment", test.name)
			return true, nil, nil
		})

		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
		deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(test.deploymentPhase)

		controller := okDeploymentController(fake, deployment, hookPods, true, test.podPhase)

		if err := controller.Handle(deployment); err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		if e, a := test.expected, deletedPods; e != a {
			t.Errorf("%s: expected %d deleted pods, got %d", test.name, e, a)
		}
	}
}

// TestHandle_deployerPodDisappeared ensures that a pending/running deployment
// is failed when its deployer pod vanishes.
func TestHandle_deployerPodDisappeared(t *testing.T) {
	tests := []struct {
		name  string
		phase deployapi.DeploymentStatus
	}{
		{
			name:  "pending",
			phase: deployapi.DeploymentStatusPending,
		},
		{
			name:  "running",
			phase: deployapi.DeploymentStatusRunning,
		},
	}

	for _, test := range tests {
		var updatedDeployment *kapi.ReplicationController
		updateCalled := false

		fake := &ktestclient.Fake{}
		fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
			updatedDeployment = rc
			updateCalled = true
			return true, nil, nil
		})

		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(test.phase)

		controller := okDeploymentController(fake, nil, nil, true, kapi.PodUnknown)

		if err := controller.Handle(deployment); err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		if !updateCalled {
			t.Errorf("%s: expected update", test.name)
			continue
		}

		if e, a := deployapi.DeploymentStatusFailed, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
			t.Errorf("%s: expected deployment status %q, got %q", test.name, e, a)
		}
	}
}

// TestHandle_transitionFromDeployer ensures that pod status drives deployment status.
func TestHandle_transitionFromDeployer(t *testing.T) {
	tests := []struct {
		name string

		podPhase        kapi.PodPhase
		deploymentPhase deployapi.DeploymentStatus

		expected deployapi.DeploymentStatus
	}{
		{
			name: "New -> Pending",

			podPhase:        kapi.PodPending,
			deploymentPhase: deployapi.DeploymentStatusNew,

			expected: deployapi.DeploymentStatusPending,
		},
		{
			name: "New -> Running",

			podPhase:        kapi.PodRunning,
			deploymentPhase: deployapi.DeploymentStatusNew,

			expected: deployapi.DeploymentStatusRunning,
		},
		{
			name: "New -> Complete",

			podPhase:        kapi.PodSucceeded,
			deploymentPhase: deployapi.DeploymentStatusNew,

			expected: deployapi.DeploymentStatusComplete,
		},
		{
			name: "New -> Failed",

			podPhase:        kapi.PodFailed,
			deploymentPhase: deployapi.DeploymentStatusNew,

			expected: deployapi.DeploymentStatusFailed,
		},
		{
			name: "Pending -> Running",

			podPhase:        kapi.PodRunning,
			deploymentPhase: deployapi.DeploymentStatusPending,

			expected: deployapi.DeploymentStatusRunning,
		},
		{
			name: "Pending -> Complete",

			podPhase:        kapi.PodSucceeded,
			deploymentPhase: deployapi.DeploymentStatusPending,

			expected: deployapi.DeploymentStatusComplete,
		},
		{
			name: "Pending -> Failed",

			podPhase:        kapi.PodFailed,
			deploymentPhase: deployapi.DeploymentStatusPending,

			expected: deployapi.DeploymentStatusFailed,
		},
		{
			name: "Running -> Complete",

			podPhase:        kapi.PodSucceeded,
			deploymentPhase: deployapi.DeploymentStatusRunning,

			expected: deployapi.DeploymentStatusComplete,
		},
		{
			name: "Running -> Failed",

			podPhase:        kapi.PodFailed,
			deploymentPhase: deployapi.DeploymentStatusRunning,

			expected: deployapi.DeploymentStatusFailed,
		},
	}

	for _, test := range tests {
		var updatedDeployment *kapi.ReplicationController
		updateCalled := false

		fake := &ktestclient.Fake{}
		fake.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			rc := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
			updatedDeployment = rc
			updateCalled = true
			return true, nil, nil
		})

		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(test.deploymentPhase)

		controller := okDeploymentController(fake, deployment, nil, true, test.podPhase)

		if err := controller.Handle(deployment); err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		if !updateCalled {
			t.Errorf("%s: expected update", test.name)
			continue
		}

		if e, a := test.expected, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
			t.Errorf("%s: expected deployment status %q, got %q", test.name, e, a)
		}
	}
}

func expectMapContains(t *testing.T, exists, expected map[string]string, what string) {
	if expected == nil {
		return
	}
	for k, v := range expected {
		value, ok := exists[k]
		if ok && value != v {
			t.Errorf("expected %s[%s]=%s, got %s", what, k, v, value)
		} else if !ok {
			t.Errorf("expected %s %s: not present", what, k)
		}
	}
}

func TestDeployerCustomLabelsAndAnnotations(t *testing.T) {
	testCases := []struct {
		name         string
		strategy     deployapi.DeploymentStrategy
		labels       map[string]string
		annotations  map[string]string
		verifyLabels bool
	}{
		{name: "labels and annotations", strategy: deploytest.OkStrategy(), labels: map[string]string{"label1": "value1"}, annotations: map[string]string{"annotation1": "value1"}, verifyLabels: true},
		{name: "custom strategy, no annotations", strategy: deploytest.OkCustomStrategy(), labels: map[string]string{"label2": "value2", "label3": "value3"}, verifyLabels: true},
		{name: "custom strategy, no labels", strategy: deploytest.OkCustomStrategy(), annotations: map[string]string{"annotation3": "value3"}, verifyLabels: true},
		{name: "no overrride", strategy: deploytest.OkStrategy(), labels: map[string]string{deployapi.DeployerPodForDeploymentLabel: "ignored"}, verifyLabels: false},
	}

	for _, test := range testCases {
		t.Logf("evaluating test case %s", test.name)
		config := deploytest.OkDeploymentConfig(1)
		config.Spec.Strategy = test.strategy
		config.Spec.Strategy.Labels = test.labels
		config.Spec.Strategy.Annotations = test.annotations
		deployment, _ := deployutil.MakeDeployment(config, codec)

		fake := &ktestclient.Fake{}
		fake.AddReactor("create", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, deployerPod(deployment, "", true), nil
		})

		controller := okDeploymentController(fake, nil, nil, true, kapi.PodUnknown)

		podTemplate, err := controller.makeDeployerPod(deployment)
		if err != nil {
			t.Fatal(err)
		}

		nameLabel, ok := podTemplate.Labels[deployapi.DeployerPodForDeploymentLabel]
		if ok && nameLabel != deployment.Name {
			t.Errorf("label %s expected %s, got %s", deployapi.DeployerPodForDeploymentLabel, deployment.Name, nameLabel)
		} else if !ok {
			t.Errorf("label %s not present", deployapi.DeployerPodForDeploymentLabel)
		}
		if test.verifyLabels {
			expectMapContains(t, podTemplate.Labels, test.labels, "labels")
		}
		expectMapContains(t, podTemplate.Annotations, test.annotations, "annotations")
	}
}
