package deployment

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// TestHandle_createPodOk ensures that a the deployer pod created in response
// to a new deployment is valid.
func TestHandle_createPodOk(t *testing.T) {
	var (
		updatedDeployment *kapi.ReplicationController
		createdPod        *kapi.Pod
		expectedContainer = okContainer()
	)

	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		podClient: &podClientImpl{
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				createdPod = pod
				return pod, nil
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			return expectedContainer, nil
		},
		recorder: &record.FakeRecorder{},
	}

	// Verify new -> pending
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	err := controller.Handle(deployment)

	if err != nil {
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

// TestHandle_makeContainerFail ensures that an internal (not API) failure to
// create a deployer pod results in a fatal error.
func TestHandle_makeContainerFail(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namspace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		podClient: &podClientImpl{
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to create pod")
				return nil, nil
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			return nil, fmt.Errorf("couldn't make container")
		},
		recorder: &record.FakeRecorder{},
	}

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	err := controller.Handle(deployment)

	if err == nil {
		t.Fatalf("expected an error")
	}

	if _, isFatal := err.(fatalError); !isFatal {
		t.Fatalf("expected a fatal error, got %v", err)
	}
}

// TestHandle_createPodFail ensures that an an API failure while creating a
// deployer pod results in a nonfatal error.
func TestHandle_createPodFail(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namspace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		podClient: &podClientImpl{
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, fmt.Errorf("Failed to create pod %s", pod.Name)
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			return okContainer(), nil
		},
		recorder: &record.FakeRecorder{},
	}

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	err := controller.Handle(deployment)

	if err == nil {
		t.Fatalf("expected an error")
	}

	if _, isFatal := err.(fatalError); isFatal {
		t.Fatalf("expected a nonfatal error, got a %#v", err)
	}
}

// TestHandle_deployerPodAlreadyExists ensures that attempts to create a
// deployer pod which  was already created don't result in an error
// (effectively skipping the handling as redundant).
func TestHandle_deployerPodAlreadyExists(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	deployerPod := relatedPod(deployment)

	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		podClient: &podClientImpl{
			getPodFunc: func(namespace, name string) (*kapi.Pod, error) {
				return deployerPod, nil
			},
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, kerrors.NewAlreadyExists("Pod", pod.Name)
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			return okContainer(), nil
		},
		recorder: &record.FakeRecorder{},
	}

	err := controller.Handle(deployment)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment.Annotations[deployapi.DeploymentPodAnnotation] != deployerPod.Name {
		t.Fatalf("deployment not updated with pod name annotation")
	}

	if updatedDeployment.Annotations[deployapi.DeploymentStatusAnnotation] != string(deployapi.DeploymentStatusPending) {
		t.Fatalf("deployment status not updated to pending")
	}
}

// TestHandle_unrelatedPodAlreadyExists ensures that attempts to create a
// deployer pod, when a pod with the same name but missing annotations results
// a transition to failed.
func TestHandle_unrelatedPodAlreadyExists(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	otherPod := unrelatedPod(deployment)

	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		podClient: &podClientImpl{
			getPodFunc: func(namespace, name string) (*kapi.Pod, error) {
				return otherPod, nil
			},
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, kerrors.NewAlreadyExists("Pod", pod.Name)
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			return okContainer(), nil
		},
		recorder: &record.FakeRecorder{},
	}

	err := controller.Handle(deployment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, exists := updatedDeployment.Annotations[deployapi.DeploymentPodAnnotation]; exists {
		t.Fatalf("deployment updated with pod name annotation")
	}

	if e, a := deployapi.DeploymentFailedUnrelatedDeploymentExists, deployment.Annotations[deployapi.DeploymentStatusReasonAnnotation]; e != a {
		t.Errorf("expected reason annotation %s, got %s", e, a)
	}

	if e, a := deployapi.DeploymentStatusFailed, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Errorf("expected deployment status %s, got %s", e, a)
	}
}

// TestHandle_noop ensures that pending, running, and failed states result in
// no action by the controller (as these represent in-progress or terminal
// states).
func TestHandle_noop(t *testing.T) {
	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
		podClient: &podClientImpl{
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to create pod")
				return nil, nil
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			t.Fatalf("unexpected call to make container")
			return nil, nil
		},
		recorder: &record.FakeRecorder{},
	}

	// Verify no-op
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)

	noopStatus := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
		deployapi.DeploymentStatusFailed,
	}
	for _, status := range noopStatus {
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(status)
		err := controller.Handle(deployment)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

// TestHandle_cleanupPodOk ensures that deployer pods are cleaned up for
// deployments in a completed state.
func TestHandle_cleanupPodOk(t *testing.T) {
	podName := "pod"
	deletedPodName := ""
	deletedPodNamespace := ""

	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
		podClient: &podClientImpl{
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to create pod")
				return nil, nil
			},
			deletePodFunc: func(namespace, name string) error {
				deletedPodNamespace = namespace
				deletedPodName = name
				return nil
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			t.Fatalf("unexpected call to make container")
			return nil, nil
		},
		recorder: &record.FakeRecorder{},
	}

	// Verify successful cleanup
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
	deployment.Annotations[deployapi.DeploymentPodAnnotation] = podName
	err := controller.Handle(deployment)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e, a := deployment.Namespace, deletedPodNamespace; e != a {
		t.Fatalf("expected deleted pod namespace %s, got %s", e, a)
	}

	if e, a := podName, deletedPodName; e != a {
		t.Fatalf("expected deleted pod name %s, got %s", e, a)
	}
}

// TestHandle_cleanupPodNoop ensures that repeated attempts to clean up an
// already-deleted deployer pod for a completed deployment safely do nothing.
func TestHandle_cleanupPodNoop(t *testing.T) {
	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
		podClient: &podClientImpl{
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to create pod")
				return nil, nil
			},
			deletePodFunc: func(namespace, name string) error {
				return kerrors.NewNotFound("Pod", name)
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			t.Fatalf("unexpected call to make container")
			return nil, nil
		},
		recorder: &record.FakeRecorder{},
	}

	// Verify no-op
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
	deployment.Annotations[deployapi.DeploymentPodAnnotation] = "pod"
	err := controller.Handle(deployment)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_cleanupPodFail ensures that a failed attempt to clean up the
// deployer pod for a completed deployment results in a nonfatal error.
func TestHandle_cleanupPodFail(t *testing.T) {
	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
		podClient: &podClientImpl{
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to create pod")
				return nil, nil
			},
			deletePodFunc: func(namespace, name string) error {
				return kerrors.NewInternalError(fmt.Errorf("test error"))
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			t.Fatalf("unexpected call to make container")
			return nil, nil
		},
		recorder: &record.FakeRecorder{},
	}

	// Verify error
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
	deployment.Annotations[deployapi.DeploymentPodAnnotation] = "pod"
	err := controller.Handle(deployment)

	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestHandle_cancelNew(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namspace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		podClient: &podClientImpl{
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to make container")
				return nil, nil
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			return okContainer(), nil
		},
		recorder: &record.FakeRecorder{},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue

	err := controller.Handle(deployment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e, a := deployapi.DeploymentStatusFailed, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected deployment status %s, got %s", e, a)
	}
}

// TestHandle_cancelPendingRunning ensures that deployer pods are terminated
// for deployments in post-New phases.
func TestHandle_cancelPendingRunning(t *testing.T) {
	var updatedPod *kapi.Pod

	controller := &DeploymentController{
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namspace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				// None of these tests should transition the phase.
				t.Errorf("unexpected call to updateDeployment")
				return nil, nil
			},
		},
		podClient: &podClientImpl{
			getPodFunc: func(namespace, name string) (*kapi.Pod, error) {
				return ttlNonZeroPod(), nil
			},
			updatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				updatedPod = pod
				return pod, nil
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			return okContainer(), nil
		},
		recorder: &record.FakeRecorder{},
	}

	cases := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
	}

	for _, status := range cases {
		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(status)
		deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
		err := controller.Handle(deployment)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if e, a := int64(1), *updatedPod.Spec.ActiveDeadlineSeconds; e != a {
			t.Errorf("expected ActiveDeadlineSeconds %d, got %d", e, a)
		}
	}
}

func okContainer() *kapi.Container {
	return &kapi.Container{
		Image:   "test/image",
		Command: []string{"command"},
		Env: []kapi.EnvVar{
			{
				Name:  "env1",
				Value: "val1",
			},
		},
		Resources: kapi.ResourceRequirements{
			Limits: kapi.ResourceList{
				kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
				kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
			},
		},
	}
}

func relatedPod(deployment *kapi.ReplicationController) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: deployment.Name,
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
		},
	}
}

func unrelatedPod(deployment *kapi.ReplicationController) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: deployment.Name,
			Annotations: map[string]string{
				"unrelatedKey": "unrelatedValue",
			},
		},
	}
}

func ttlNonZeroPod() *kapi.Pod {
	ttl := int64(10)
	return &kapi.Pod{
		Spec: kapi.PodSpec{
			ActiveDeadlineSeconds: &ttl,
		},
	}
}

func ttlZeroPod() *kapi.Pod {
	ttl := int64(1)
	return &kapi.Pod{
		Spec: kapi.PodSpec{
			ActiveDeadlineSeconds: &ttl,
		},
	}
}
