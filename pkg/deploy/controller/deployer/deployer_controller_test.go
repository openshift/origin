package deployment

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	"k8s.io/kubernetes/pkg/api/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	kinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"

	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	_ "github.com/openshift/origin/pkg/deploy/apis/apps/install"
	deploytest "github.com/openshift/origin/pkg/deploy/apis/apps/test"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

var (
	env   = []kapi.EnvVar{{Name: "ENV1", Value: "VAL1"}}
	codec = kapi.Codecs.LegacyCodec(deployapiv1.SchemeGroupVersion)
)

func alwaysReady() bool { return true }

type deploymentController struct {
	*DeploymentController
	podIndexer cache.Indexer
}

func okDeploymentController(client kclientset.Interface, deployment *v1.ReplicationController, hookPodNames []string, related bool, deployerStatus v1.PodPhase) *deploymentController {
	informerFactory := kinformers.NewSharedInformerFactory(client, 0)
	rcInformer := informerFactory.Core().V1().ReplicationControllers()
	podInformer := informerFactory.Core().V1().Pods()

	c := NewDeployerController(rcInformer, podInformer, client, "sa:test", "openshift/origin-deployer", env, codec)
	c.podListerSynced = alwaysReady
	c.rcListerSynced = alwaysReady

	// deployer pod
	if deployment != nil {
		pod := deployerPod(deployment, "", related)
		pod.Status.Phase = deployerStatus
		podInformer.Informer().GetIndexer().Add(pod)
	}

	// hook pods
	for _, name := range hookPodNames {
		pod := deployerPod(deployment, name, related)
		podInformer.Informer().GetIndexer().Add(pod)
	}

	return &deploymentController{
		c,
		podInformer.Informer().GetIndexer(),
	}
}

func deployerPod(deployment *v1.ReplicationController, alternateName string, related bool) *v1.Pod {
	deployerPodName := deployutil.DeployerPodNameForDeployment(deployment.Name)
	if len(alternateName) > 0 {
		deployerPodName = alternateName
	}

	deployment.Namespace = "test"

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
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

func okContainer() *v1.Container {
	return &v1.Container{
		Image:   "openshift/origin-deployer",
		Command: []string{"/bin/echo", "hello", "world"},
		Env:     deployutil.CopyApiEnvVarToV1EnvVar(env),
		Resources: v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceName(v1.ResourceCPU):    resource.MustParse("10"),
				v1.ResourceName(v1.ResourceMemory): resource.MustParse("10G"),
			},
		},
	}
}

// TestHandle_createPodOk ensures that a deployer pod created in response
// to a new deployment is valid.
func TestHandle_createPodOk(t *testing.T) {
	var (
		updatedDeployment *v1.ReplicationController
		createdPod        *v1.Pod
		expectedContainer = okContainer()
	)

	client := &fake.Clientset{}
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		pod := action.(clientgotesting.CreateAction).GetObject().(*v1.Pod)
		createdPod = pod
		return true, pod, nil
	})
	client.AddReactor("patch", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, nil
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	// Verify new -> pending
	config := deploytest.OkDeploymentConfig(1)
	config.Spec.Strategy = deploytest.OkCustomStrategy()
	deployment, _ := deployutil.MakeDeploymentV1(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	deployment.Spec.Template.Spec.NodeSelector = map[string]string{"labelKey1": "labelValue1", "labelKey2": "labelValue2"}

	controller := okDeploymentController(client, nil, nil, true, v1.PodUnknown)

	if err := controller.handle(deployment, false); err != nil {
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

	if e, a := expectedContainer.Resources, actualContainer.Resources; !kapihelper.Semantic.DeepEqual(e, a) {
		t.Fatalf("expected container resources %v, got %v", expectedContainer.Resources, actualContainer.Resources)
	}
}

// TestHandle_createPodFail ensures that an API failure while creating a
// deployer pod results in a nonfatal error.
func TestHandle_createPodFail(t *testing.T) {
	var updatedDeployment *v1.ReplicationController

	client := &fake.Clientset{}
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(clientgotesting.CreateAction).GetObject().(*v1.Pod).Name
		return true, nil, fmt.Errorf("failed to create pod %q", name)
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeploymentV1(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)

	controller := okDeploymentController(client, nil, nil, true, v1.PodUnknown)

	err := controller.handle(deployment, false)
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

		podPhase v1.PodPhase
		expected deployapi.DeploymentStatus
	}{
		{
			name: "pending",

			podPhase: v1.PodPending,
			expected: deployapi.DeploymentStatusPending,
		},
		{
			name: "running",

			podPhase: v1.PodRunning,
			expected: deployapi.DeploymentStatusRunning,
		},
		{
			name: "complete",

			podPhase: v1.PodFailed,
			expected: deployapi.DeploymentStatusFailed,
		},
		{
			name: "failed",

			podPhase: v1.PodSucceeded,
			expected: deployapi.DeploymentStatusComplete,
		},
	}

	for _, test := range tests {
		var updatedDeployment *v1.ReplicationController

		config := deploytest.OkDeploymentConfig(1)
		deployment, _ := deployutil.MakeDeploymentV1(config, codec)
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
		deployerPodName := deployutil.DeployerPodNameForDeployment(deployment.Name)

		client := &fake.Clientset{}
		client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			name := action.(clientgotesting.CreateAction).GetObject().(*v1.Pod).Name
			return true, nil, kerrors.NewAlreadyExists(kapi.Resource("Pod"), name)
		})
		client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
			updatedDeployment = rc
			return true, rc, nil
		})

		controller := okDeploymentController(client, deployment, nil, true, test.podPhase)

		if err := controller.handle(deployment, false); err != nil {
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
	var updatedDeployment *v1.ReplicationController

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeploymentV1(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)

	client := &fake.Clientset{}
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(clientgotesting.CreateAction).GetObject().(*v1.Pod).Name
		return true, nil, kerrors.NewAlreadyExists(kapi.Resource("Pod"), name)
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	controller := okDeploymentController(client, deployment, nil, false, v1.PodRunning)

	if err := controller.handle(deployment, false); err != nil {
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
	var updatedDeployment *v1.ReplicationController

	config := deploytest.TestDeploymentConfig(deploytest.OkDeploymentConfig(1))
	deployment, _ := deployutil.MakeDeploymentV1(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	one := int32(1)
	deployment.Spec.Replicas = &one

	client := &fake.Clientset{}
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(clientgotesting.CreateAction).GetObject().(*v1.Pod).Name
		return true, nil, kerrors.NewAlreadyExists(kapi.Resource("Pod"), name)
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	controller := okDeploymentController(client, deployment, nil, false, v1.PodRunning)

	if err := controller.handle(deployment, false); err != nil {
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
	if e, a := int32(0), *updatedDeployment.Spec.Replicas; e != a {
		t.Fatalf("expected failed deployment to be scaled to zero: %d", a)
	}
}

// TestHandle_noop ensures that pending, running, and failed states result in
// no action by the controller (as long as the deployment hasn't been cancelled
// and the deployer pod status is synced with the deployment status).
func TestHandle_noop(t *testing.T) {
	tests := []struct {
		name string

		podPhase        v1.PodPhase
		deploymentPhase deployapi.DeploymentStatus
	}{
		{
			name: "pending",

			podPhase:        v1.PodPending,
			deploymentPhase: deployapi.DeploymentStatusPending,
		},
		{
			name: "running",

			podPhase:        v1.PodRunning,
			deploymentPhase: deployapi.DeploymentStatusRunning,
		},
		{
			name: "complete",

			podPhase:        v1.PodFailed,
			deploymentPhase: deployapi.DeploymentStatusFailed,
		},
	}

	for _, test := range tests {
		client := fake.NewSimpleClientset()

		deployment, _ := deployutil.MakeDeploymentV1(deploytest.OkDeploymentConfig(1), codec)
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(test.deploymentPhase)

		controller := okDeploymentController(client, deployment, nil, true, test.podPhase)

		if err := controller.handle(deployment, false); err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		hasPatch := func(actions []clientgotesting.Action) bool {
			for _, a := range actions {
				if a.GetVerb() == "patch" {
					return true
				}
			}
			return false
		}

		// Expect only patching for ownerRefs
		if len(client.Actions()) != 1 && hasPatch(client.Actions()) {
			t.Errorf("%s: unexpected %d actions: %#+v", test.name, len(client.Actions()), client.Actions())
		}
	}
}

// TestHandle_failedTest ensures that failed test deployments have their
// replicas set to zero.
func TestHandle_failedTest(t *testing.T) {
	var updatedDeployment *v1.ReplicationController

	client := &fake.Clientset{}
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	// Verify successful cleanup
	config := deploytest.TestDeploymentConfig(deploytest.OkDeploymentConfig(1))
	deployment, _ := deployutil.MakeDeploymentV1(config, codec)
	one := int32(1)
	deployment.Spec.Replicas = &one
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)

	controller := okDeploymentController(client, deployment, nil, true, v1.PodFailed)

	if err := controller.handle(deployment, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatal("deployment not updated")
	}
	if e, a := int32(0), *updatedDeployment.Spec.Replicas; e != a {
		t.Fatalf("expected updated deployment replicas to be %d, got %d", e, a)
	}
}

// TestHandle_cleanupPodOk ensures that deployer pods are cleaned up for
// deployments in a completed state.
func TestHandle_cleanupPodOk(t *testing.T) {
	hookPods := []string{"pre", "mid", "post"}
	deletedPodNames := []string{}

	client := &fake.Clientset{}
	client.AddReactor("delete", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(clientgotesting.DeleteAction).GetName()
		deletedPodNames = append(deletedPodNames, name)
		return true, nil, nil
	})
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected deployment update")
		return true, nil, nil
	})

	// Verify successful cleanup
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeploymentV1(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)

	controller := okDeploymentController(client, deployment, hookPods, true, v1.PodSucceeded)
	hookPods = append(hookPods, deployment.Name)

	if err := controller.handle(deployment, false); err != nil {
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
	var updatedDeployment *v1.ReplicationController

	client := &fake.Clientset{}
	client.AddReactor("delete", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(clientgotesting.DeleteAction).GetName()
		deletedPodNames = append(deletedPodNames, name)
		return true, nil, nil
	})
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	// Verify successful cleanup
	config := deploytest.TestDeploymentConfig(deploytest.OkDeploymentConfig(1))
	deployment, _ := deployutil.MakeDeploymentV1(config, codec)
	one := int32(1)
	deployment.Spec.Replicas = &one
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)

	controller := okDeploymentController(client, deployment, hookPods, true, v1.PodSucceeded)
	hookPods = append(hookPods, deployment.Name)

	if err := controller.handle(deployment, false); err != nil {
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
	if e, a := int32(0), *updatedDeployment.Spec.Replicas; e != a {
		t.Fatalf("expected updated deployment replicas to be %d, got %d", e, a)
	}
}

// TestHandle_cleanupPodNoop ensures that an attempt to delete pods is not made
// if the deployer pods are not listed based on a label query
func TestHandle_cleanupPodNoop(t *testing.T) {
	client := &fake.Clientset{}
	client.AddReactor("delete", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to delete pod")
		return true, nil, nil
	})
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected deployment update")
		return true, nil, nil
	})

	// Verify no-op
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeploymentV1(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)

	controller := okDeploymentController(client, deployment, nil, true, v1.PodSucceeded)
	pod := deployerPod(deployment, "", true)
	pod.Labels[deployapi.DeployerPodForDeploymentLabel] = "unrelated"
	controller.podIndexer.Update(pod)

	if err := controller.handle(deployment, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_cleanupPodFail ensures that a failed attempt to clean up the
// deployer pod for a completed deployment results in an actionable error.
func TestHandle_cleanupPodFail(t *testing.T) {
	client := &fake.Clientset{}
	client.AddReactor("delete", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kerrors.NewInternalError(fmt.Errorf("deployer pod internal error"))
	})
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected deployment update")
		return true, nil, nil
	})

	// Verify error
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeploymentV1(config, codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)

	controller := okDeploymentController(client, deployment, nil, true, v1.PodSucceeded)

	err := controller.handle(deployment, false)
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
	var updatedDeployment *v1.ReplicationController

	client := &fake.Clientset{}
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
		updatedDeployment = rc
		return true, rc, nil
	})

	deployment, _ := deployutil.MakeDeploymentV1(deploytest.OkDeploymentConfig(1), codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue

	controller := okDeploymentController(client, deployment, nil, true, v1.PodRunning)

	if err := controller.handle(deployment, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e, a := deployapi.DeploymentStatusPending, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected deployment status %s, got %s", e, a)
	}
}

// TestHandle_cleanupNewWithDeployers ensures that we will try to cleanup deployer pods
// for a cancelled deployment.
func TestHandle_cleanupNewWithDeployers(t *testing.T) {
	var updatedDeployment *v1.ReplicationController
	deletedDeployer := false

	deployment, _ := deployutil.MakeDeploymentV1(deploytest.OkDeploymentConfig(1), codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue

	client := &fake.Clientset{}
	client.AddReactor("delete", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		deletedDeployer = true
		return true, nil, nil
	})
	client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatalf("unexpected call to create pod")
		return true, nil, nil
	})
	client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
		updatedDeployment = rc
		return true, nil, nil
	})

	controller := okDeploymentController(client, deployment, nil, true, v1.PodRunning)

	if err := controller.handle(deployment, false); err != nil {
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
		podPhase        v1.PodPhase

		expected int
	}{
		{
			name: "pending",

			deploymentPhase: deployapi.DeploymentStatusPending,
			podPhase:        v1.PodPending,

			expected: len(hookPods) + 1,
		},
		{
			name: "running",

			deploymentPhase: deployapi.DeploymentStatusRunning,
			podPhase:        v1.PodRunning,

			expected: len(hookPods) + 1,
		},
		{
			name: "failed",

			deploymentPhase: deployapi.DeploymentStatusFailed,
			podPhase:        v1.PodFailed,

			expected: len(hookPods) + 1,
		},
		{
			name: "complete",

			deploymentPhase: deployapi.DeploymentStatusComplete,
			podPhase:        v1.PodSucceeded,

			expected: len(hookPods) + 1,
		},
	}

	for _, test := range tests {
		deletedPods := 0

		client := &fake.Clientset{}
		client.AddReactor("delete", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			deletedPods++
			return true, nil, nil
		})
		client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			// None of these tests should transition the phase.
			t.Errorf("%s: unexpected call to update a deployment", test.name)
			return true, nil, nil
		})

		deployment, _ := deployutil.MakeDeploymentV1(deploytest.OkDeploymentConfig(1), codec)
		deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(test.deploymentPhase)

		controller := okDeploymentController(client, deployment, hookPods, true, test.podPhase)

		if err := controller.handle(deployment, false); err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		if e, a := test.expected, deletedPods; e != a {
			t.Errorf("%s: expected %d deleted pods, got %d", test.name, e, a)
		}
	}
}

// TestHandle_deployerPodDisappeared ensures that a pending/running deployment
// is failed when its deployer pod vanishes. Ensure that pending deployments
// wont fail instantly on a missing deployer pod because it may take some time
// for it to appear in the pod cache.
func TestHandle_deployerPodDisappeared(t *testing.T) {
	tests := []struct {
		name          string
		phase         deployapi.DeploymentStatus
		willBeDropped bool
		shouldRetry   bool
	}{
		{
			name:          "pending - retry",
			phase:         deployapi.DeploymentStatusPending,
			willBeDropped: false,
			shouldRetry:   true,
		},
		{
			name:          "pending - fail",
			phase:         deployapi.DeploymentStatusPending,
			willBeDropped: true,
			shouldRetry:   false,
		},
		{
			name:  "running",
			phase: deployapi.DeploymentStatusRunning,
		},
	}

	for _, test := range tests {
		var updatedDeployment *v1.ReplicationController
		updateCalled := false

		client := &fake.Clientset{}
		client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
			updatedDeployment = rc
			updateCalled = true
			return true, nil, nil
		})

		deployment, err := deployutil.MakeDeploymentV1(deploytest.OkDeploymentConfig(1), codec)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(test.phase)
		updatedDeployment = deployment

		controller := okDeploymentController(client, nil, nil, true, v1.PodUnknown)

		err = controller.handle(deployment, test.willBeDropped)
		if !test.shouldRetry && err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		if test.shouldRetry && err == nil {
			t.Errorf("%s: expected an error so that the deployment can be retried, got none", test.name)
			continue
		}

		if !test.shouldRetry && !updateCalled {
			t.Errorf("%s: expected update", test.name)
			continue
		}

		if test.shouldRetry && updateCalled {
			t.Errorf("%s: unexpected update", test.name)
			continue
		}

		gotStatus := deployutil.DeploymentStatusFor(updatedDeployment)
		if !test.shouldRetry && deployapi.DeploymentStatusFailed != gotStatus {
			t.Errorf("%s: expected deployment status %q, got %q", test.name, deployapi.DeploymentStatusFailed, gotStatus)
			continue
		}

		if test.shouldRetry && deployapi.DeploymentStatusPending != gotStatus {
			t.Errorf("%s: expected deployment status %q, got %q", test.name, deployapi.DeploymentStatusPending, gotStatus)
			continue
		}
	}
}

// TestHandle_transitionFromDeployer ensures that pod status drives deployment status.
func TestHandle_transitionFromDeployer(t *testing.T) {
	tests := []struct {
		name string

		podPhase        v1.PodPhase
		deploymentPhase deployapi.DeploymentStatus

		expected deployapi.DeploymentStatus
	}{
		{
			name: "New -> Pending",

			podPhase:        v1.PodPending,
			deploymentPhase: deployapi.DeploymentStatusNew,

			expected: deployapi.DeploymentStatusPending,
		},
		{
			name: "New -> Running",

			podPhase:        v1.PodRunning,
			deploymentPhase: deployapi.DeploymentStatusNew,

			expected: deployapi.DeploymentStatusRunning,
		},
		{
			name: "New -> Complete",

			podPhase:        v1.PodSucceeded,
			deploymentPhase: deployapi.DeploymentStatusNew,

			expected: deployapi.DeploymentStatusComplete,
		},
		{
			name: "New -> Failed",

			podPhase:        v1.PodFailed,
			deploymentPhase: deployapi.DeploymentStatusNew,

			expected: deployapi.DeploymentStatusFailed,
		},
		{
			name: "Pending -> Running",

			podPhase:        v1.PodRunning,
			deploymentPhase: deployapi.DeploymentStatusPending,

			expected: deployapi.DeploymentStatusRunning,
		},
		{
			name: "Pending -> Complete",

			podPhase:        v1.PodSucceeded,
			deploymentPhase: deployapi.DeploymentStatusPending,

			expected: deployapi.DeploymentStatusComplete,
		},
		{
			name: "Pending -> Failed",

			podPhase:        v1.PodFailed,
			deploymentPhase: deployapi.DeploymentStatusPending,

			expected: deployapi.DeploymentStatusFailed,
		},
		{
			name: "Running -> Complete",

			podPhase:        v1.PodSucceeded,
			deploymentPhase: deployapi.DeploymentStatusRunning,

			expected: deployapi.DeploymentStatusComplete,
		},
		{
			name: "Running -> Failed",

			podPhase:        v1.PodFailed,
			deploymentPhase: deployapi.DeploymentStatusRunning,

			expected: deployapi.DeploymentStatusFailed,
		},
	}

	for _, test := range tests {
		var updatedDeployment *v1.ReplicationController
		updateCalled := false

		client := &fake.Clientset{}
		client.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			rc := action.(clientgotesting.UpdateAction).GetObject().(*v1.ReplicationController)
			updatedDeployment = rc
			updateCalled = true
			return true, nil, nil
		})

		deployment, _ := deployutil.MakeDeploymentV1(deploytest.OkDeploymentConfig(1), codec)
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(test.deploymentPhase)

		controller := okDeploymentController(client, deployment, nil, true, test.podPhase)

		if err := controller.handle(deployment, false); err != nil {
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
		deployment, _ := deployutil.MakeDeploymentV1(config, codec)

		client := &fake.Clientset{}
		client.AddReactor("create", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, deployerPod(deployment, "", true), nil
		})

		controller := okDeploymentController(client, nil, nil, true, v1.PodUnknown)

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
