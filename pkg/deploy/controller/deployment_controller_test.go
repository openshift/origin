package controller

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/controller/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestHandleNewDeploymentCreatePodOk(t *testing.T) {
	var (
		updatedDeployment *kapi.ReplicationController
		createdPod        *kapi.Pod
		expectedContainer = basicContainer()
	)

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				createdPod = pod
				return pod, nil
			},
		},
		NextDeployment: func() *kapi.ReplicationController {
			deployment := basicDeployment()
			deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
			return deployment
		},
		ContainerCreator: &testContainerCreator{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				return expectedContainer
			},
		},
	}

	// Verify new -> pending
	controller.HandleDeployment()

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
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(namspace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, fmt.Errorf("Failed to create pod %s", pod.Name)
			},
		},
		NextDeployment: func() *kapi.ReplicationController {
			deployment := basicDeployment()
			deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
			return deployment
		},
		ContainerCreator: &testContainerCreator{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				return basicContainer()
			},
		},
	}

	// Verify new -> failed
	controller.HandleDeployment()

	if updatedDeployment == nil {
		t.Fatalf("expected an updated deployment")
	}

	if e, a := string(deployapi.DeploymentStatusFailed), updatedDeployment.Annotations[deployapi.DeploymentStatusAnnotation]; e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

func TestHandleNewDeploymentCreatePodAlreadyExists(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, kerrors.NewAlreadyExists("pod", pod.Name)
			},
		},
		NextDeployment: func() *kapi.ReplicationController {
			deployment := basicDeployment()
			deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
			return deployment
		},
		ContainerCreator: &testContainerCreator{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				return basicContainer()
			},
		},
	}

	// Verify new -> pending
	controller.HandleDeployment()

	if updatedDeployment == nil {
		t.Fatalf("expected an updated deployment")
	}

	if e, a := string(deployapi.DeploymentStatusPending), updatedDeployment.Annotations[deployapi.DeploymentStatusAnnotation]; e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

func TestHandleUncorrelatedPod(t *testing.T) {
	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("Unexpected deployment update")
				return nil, nil
			},
		},
		PodInterface:   &testDcPodInterface{},
		NextDeployment: func() *kapi.ReplicationController { return nil },
		NextPod: func() *kapi.Pod {
			pod := runningPod()
			pod.Annotations = make(map[string]string)
			return pod
		},
		DeploymentStore: deploytest.NewFakeDeploymentStore(pendingDeployment()),
	}

	// Verify no-op
	controller.HandlePod()
}

func TestHandleOrphanedPod(t *testing.T) {
	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("Unexpected deployment update")
				return nil, nil
			},
		},
		PodInterface:    &testDcPodInterface{},
		NextDeployment:  func() *kapi.ReplicationController { return nil },
		NextPod:         func() *kapi.Pod { return runningPod() },
		DeploymentStore: deploytest.NewFakeDeploymentStore(nil),
	}

	// Verify no-op
	controller.HandlePod()
}

func TestHandlePodRunning(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
		PodInterface: &testDcPodInterface{},
		NextDeployment: func() *kapi.ReplicationController {
			return nil
		},
		NextPod:         func() *kapi.Pod { return runningPod() },
		DeploymentStore: deploytest.NewFakeDeploymentStore(pendingDeployment()),
	}

	controller.HandlePod()

	if updatedDeployment == nil {
		t.Fatalf("Expected a deployment to be updated")
	}

	if e, a := string(deployapi.DeploymentStatusRunning), updatedDeployment.Annotations[deployapi.DeploymentStatusAnnotation]; e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

func TestHandlePodTerminatedOk(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController
	var deletedPodID string

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			DeletePodFunc: func(namespace, name string) error {
				deletedPodID = name
				return nil
			},
		},
		NextDeployment:  func() *kapi.ReplicationController { return nil },
		NextPod:         func() *kapi.Pod { return succeededPod() },
		DeploymentStore: deploytest.NewFakeDeploymentStore(runningDeployment()),
	}

	controller.HandlePod()

	if updatedDeployment == nil {
		t.Fatalf("Expected a deployment to be updated")
	}

	if e, a := string(deployapi.DeploymentStatusComplete), updatedDeployment.Annotations[deployapi.DeploymentStatusAnnotation]; e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}

	if len(deletedPodID) == 0 {
		t.Fatalf("expected pod to be deleted")
	}
}

func TestHandlePodTerminatedNotOk(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeploymentController{
		Codec: api.Codec,
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			DeletePodFunc: func(namespace, name string) error {
				t.Fatalf("unexpected delete of pod %s", name)
				return nil
			},
		},
		ContainerCreator: &testContainerCreator{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				return basicContainer()
			},
		},
		NextDeployment:  func() *kapi.ReplicationController { return nil },
		NextPod:         func() *kapi.Pod { return failedPod() },
		DeploymentStore: deploytest.NewFakeDeploymentStore(runningDeployment()),
	}

	controller.HandlePod()

	if updatedDeployment == nil {
		t.Fatalf("Expected a deployment to be updated")
	}

	if e, a := string(deployapi.DeploymentStatusFailed), updatedDeployment.Annotations[deployapi.DeploymentStatusAnnotation]; e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

type testContainerCreator struct {
	CreateContainerFunc func(strategy *deployapi.DeploymentStrategy) *kapi.Container
}

func (t *testContainerCreator) CreateContainer(strategy *deployapi.DeploymentStrategy) *kapi.Container {
	return t.CreateContainerFunc(strategy)
}

type testDcDeploymentInterface struct {
	UpdateDeploymentFunc func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *testDcDeploymentInterface) UpdateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.UpdateDeploymentFunc(namespace, deployment)
}

type testDcPodInterface struct {
	CreatePodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePodFunc func(namespace, name string) error
}

func (i *testDcPodInterface) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.CreatePodFunc(namespace, pod)
}

func (i *testDcPodInterface) DeletePod(namespace, name string) error {
	return i.DeletePodFunc(namespace, name)
}

func basicDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "deploy1"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerManual,
			},
		},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{
					"name": "test-pod",
				},
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
						Labels: map[string]string{
							"name": "test-pod",
						},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name:  "container-1",
								Image: "registry:8080/openshift/test-image:ref-1",
							},
						},
					},
				},
			},
		},
	}
}

func basicDeployment() *kapi.ReplicationController {
	config := basicDeploymentConfig()
	encodedConfig, _ := deployutil.EncodeDeploymentConfig(config, api.Codec)
	return &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: "deploy1",
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation:        config.Name,
				deployapi.DeploymentStatusAnnotation:        string(deployapi.DeploymentStatusNew),
				deployapi.DeploymentEncodedConfigAnnotation: encodedConfig,
			},
			Labels: config.Labels,
		},
		Spec: kapi.ReplicationControllerSpec{
			Template: &kapi.PodTemplateSpec{
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "container1",
							Image: "registry:8080/repo1:ref1",
						},
					},
				},
			},
		},
	}
}

func pendingDeployment() *kapi.ReplicationController {
	d := basicDeployment()
	d.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusPending)
	return d
}

func runningDeployment() *kapi.ReplicationController {
	d := basicDeployment()
	d.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)
	return d
}

func basicContainer() *kapi.Container {
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

func basicPod() *kapi.Pod {
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
	p := basicPod()
	p.Status.Phase = kapi.PodSucceeded
	return p
}

func failedPod() *kapi.Pod {
	p := basicPod()
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
	p := basicPod()
	p.Status.Phase = kapi.PodRunning
	return p
}
