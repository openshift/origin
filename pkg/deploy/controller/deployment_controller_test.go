package controller

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestHandleNewDeploymentCreatePodOk(t *testing.T) {
	var (
		updatedDeployment *kapi.ReplicationController
		createdPod        *kapi.Pod
		expectedContainer = okContainer()
	)

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		PodClient: &DeploymentControllerPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				createdPod = pod
				return pod, nil
			},
		},
		ContainerCreator: &DeploymentContainerCreatorImpl{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				return expectedContainer
			},
		},
	}

	// Verify new -> pending
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	err := controller.HandleDeployment(deployment)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatalf("expected an updated deployment")
	}

	if e, a := string(deployapi.DeploymentStatusPending), updatedDeployment.Annotations[deployapi.DeploymentStatusAnnotation]; e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}

	if createdPod == nil {
		t.Fatalf("expected a pod to be created")
	}

	if e, a := createdPod.Name, updatedDeployment.Annotations[deployapi.DeploymentPodAnnotation]; e != a {
		t.Fatalf("expected deployment pod annotation %s, got %s", e, a)
	}

	if e, a := updatedDeployment.Name, createdPod.Annotations[deployapi.DeploymentAnnotation]; e != a {
		t.Fatalf("expected pod deployment annotation %s, got %s", e, a)
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
}

func TestHandleNewDeploymentCreatePodFail(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			UpdateDeploymentFunc: func(namspace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		PodClient: &DeploymentControllerPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, fmt.Errorf("Failed to create pod %s", pod.Name)
			},
		},
		ContainerCreator: &DeploymentContainerCreatorImpl{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				return okContainer()
			},
		},
	}

	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	err := controller.HandleDeployment(deployment)

	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestHandleNewDeploymentCreatePodAlreadyExists(t *testing.T) {
	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
		PodClient: &DeploymentControllerPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, kerrors.NewAlreadyExists("Pod", pod.Name)
			},
		},
		ContainerCreator: &DeploymentContainerCreatorImpl{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				return okContainer()
			},
		},
	}

	// Verify no-op
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusPending)
	err := controller.HandleDeployment(deployment)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleDeploymentNoops(t *testing.T) {
	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
		PodClient: &DeploymentControllerPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to create pod")
				return nil, nil
			},
		},
		ContainerCreator: &DeploymentContainerCreatorImpl{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				t.Fatalf("unexpected call to create container")
				return nil
			},
		},
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
		err := controller.HandleDeployment(deployment)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestHandleDeploymentPodCleanupOk(t *testing.T) {
	podName := "pod"
	deletedPodName := ""
	deletedPodNamespace := ""

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
		PodClient: &DeploymentControllerPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to create pod")
				return nil, nil
			},
			DeletePodFunc: func(namespace, name string) error {
				deletedPodNamespace = namespace
				deletedPodName = name
				return nil
			},
		},
		ContainerCreator: &DeploymentContainerCreatorImpl{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				t.Fatalf("unexpected call to create container")
				return nil
			},
		},
	}

	// Verify successful cleanup
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
	deployment.Annotations[deployapi.DeploymentPodAnnotation] = podName
	err := controller.HandleDeployment(deployment)

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

func TestHandleDeploymentPodCleanupNoop(t *testing.T) {
	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
		PodClient: &DeploymentControllerPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to create pod")
				return nil, nil
			},
			DeletePodFunc: func(namespace, name string) error {
				return kerrors.NewNotFound("Pod", name)
			},
		},
		ContainerCreator: &DeploymentContainerCreatorImpl{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				t.Fatalf("unexpected call to create container")
				return nil
			},
		},
	}

	// Verify no-op
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
	deployment.Annotations[deployapi.DeploymentPodAnnotation] = "pod"
	err := controller.HandleDeployment(deployment)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleDeploymentPodCleanupFailure(t *testing.T) {
	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
		PodClient: &DeploymentControllerPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected call to create pod")
				return nil, nil
			},
			DeletePodFunc: func(namespace, name string) error {
				return kerrors.NewInternalError(fmt.Errorf("test error"))
			},
		},
		ContainerCreator: &DeploymentContainerCreatorImpl{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				t.Fatalf("unexpected call to create container")
				return nil
			},
		},
	}

	// Verify error
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
	deployment.Annotations[deployapi.DeploymentPodAnnotation] = "pod"
	err := controller.HandleDeployment(deployment)

	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestHandleUncorrelatedPod(t *testing.T) {
	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
	}

	// Verify no-op
	pod := runningPod()
	pod.Annotations = make(map[string]string)
	err := controller.HandlePod(pod)

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestHandleOrphanedPod(t *testing.T) {
	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("Unexpected deployment update")
				return nil, nil
			},
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("ReplicationController", name)
			},
		},
	}

	err := controller.HandlePod(runningPod())

	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestHandlePodRunning(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				config := deploytest.OkDeploymentConfig(1)
				deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
				deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusPending)
				return deployment, nil
			},
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
	}

	err := controller.HandlePod(runningPod())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatalf("expected deployment update")
	}

	if e, a := deployapi.DeploymentStatusRunning, statusFor(updatedDeployment); e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

func TestHandlePodTerminatedOk(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				config := deploytest.OkDeploymentConfig(1)
				deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
				deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)
				return deployment, nil
			},
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
	}

	err := controller.HandlePod(succeededPod())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatalf("expected deployment update")
	}

	if e, a := deployapi.DeploymentStatusComplete, statusFor(updatedDeployment); e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

func TestHandlePodTerminatedNotOk(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentClient: &DeploymentControllerDeploymentClientImpl{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				config := deploytest.OkDeploymentConfig(1)
				deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
				deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)
				return deployment, nil
			},
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
	}

	err := controller.HandlePod(failedPod())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatalf("expected deployment update")
	}

	if e, a := deployapi.DeploymentStatusFailed, statusFor(updatedDeployment); e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
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
	}
}

func okPod() *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: "deploy-deploy1",
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: "1234",
			},
		},
		Status: kapi.PodStatus{
			Info: kapi.PodInfo{
				"container1": kapi.ContainerStatus{},
			},
		},
	}
}

func succeededPod() *kapi.Pod {
	p := okPod()
	p.Status.Phase = kapi.PodSucceeded
	return p
}

func failedPod() *kapi.Pod {
	p := okPod()
	p.Status.Phase = kapi.PodFailed
	p.Status.Info["container1"] = kapi.ContainerStatus{
		State: kapi.ContainerState{
			Termination: &kapi.ContainerStateTerminated{
				ExitCode: 1,
			},
		},
	}
	return p
}

func runningPod() *kapi.Pod {
	p := okPod()
	p.Status.Phase = kapi.PodRunning
	return p
}
