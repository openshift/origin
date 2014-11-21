package controller

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/controller/test"
)

func TestHandleNewDeploymentCreatePodOk(t *testing.T) {
	var (
		updatedDeployment *deployapi.Deployment
		createdPod        *kapi.Pod
		expectedContainer = basicContainer()
	)

	controller := &DeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
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
		NextDeployment: func() *deployapi.Deployment {
			deployment := basicDeployment()
			deployment.Status = deployapi.DeploymentStatusNew
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

	if e, a := deployapi.DeploymentStatusPending, updatedDeployment.Status; e != a {
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

	actualContainer := createdPod.DesiredState.Manifest.Containers[0]

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
	var updatedDeployment *deployapi.Deployment

	controller := &DeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, fmt.Errorf("Failed to create pod %s", pod.Name)
			},
		},
		NextDeployment: func() *deployapi.Deployment {
			deployment := basicDeployment()
			deployment.Status = deployapi.DeploymentStatusNew
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

	if e, a := deployapi.DeploymentStatusFailed, updatedDeployment.Status; e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

func TestHandleNewDeploymentCreatePodAlreadyExists(t *testing.T) {
	var updatedDeployment *deployapi.Deployment

	controller := &DeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				updatedDeployment = deployment
				return updatedDeployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return nil, kerrors.NewAlreadyExists("pod", pod.Name)
			},
		},
		NextDeployment: func() *deployapi.Deployment {
			deployment := basicDeployment()
			deployment.Status = deployapi.DeploymentStatusNew
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

	if e, a := deployapi.DeploymentStatusPending, updatedDeployment.Status; e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

func TestHandleUncorrelatedPod(t *testing.T) {
	controller := &DeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				t.Fatalf("Unexpected deployment update")
				return nil, nil
			},
		},
		PodInterface:   &testDcPodInterface{},
		NextDeployment: func() *deployapi.Deployment { return nil },
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
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				t.Fatalf("Unexpected deployment update")
				return nil, nil
			},
		},
		PodInterface:    &testDcPodInterface{},
		NextDeployment:  func() *deployapi.Deployment { return nil },
		NextPod:         func() *kapi.Pod { return runningPod() },
		DeploymentStore: deploytest.NewFakeDeploymentStore(nil),
	}

	// Verify no-op
	controller.HandlePod()
}

func TestHandlePodRunning(t *testing.T) {
	var updatedDeployment *deployapi.Deployment

	controller := &DeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
		PodInterface: &testDcPodInterface{},
		NextDeployment: func() *deployapi.Deployment {
			return nil
		},
		NextPod:         func() *kapi.Pod { return runningPod() },
		DeploymentStore: deploytest.NewFakeDeploymentStore(pendingDeployment()),
	}

	controller.HandlePod()

	if updatedDeployment == nil {
		t.Fatalf("Expected a deployment to be updated")
	}

	if e, a := deployapi.DeploymentStatusRunning, updatedDeployment.Status; e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

func TestHandlePodTerminatedOk(t *testing.T) {
	var updatedDeployment *deployapi.Deployment
	var deletedPodId string

	controller := &DeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			DeletePodFunc: func(namespace, id string) error {
				deletedPodId = id
				return nil
			},
		},
		NextDeployment:  func() *deployapi.Deployment { return nil },
		NextPod:         func() *kapi.Pod { return succeededPod() },
		DeploymentStore: deploytest.NewFakeDeploymentStore(runningDeployment()),
	}

	controller.HandlePod()

	if updatedDeployment == nil {
		t.Fatalf("Expected a deployment to be updated")
	}

	if e, a := deployapi.DeploymentStatusComplete, updatedDeployment.Status; e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}

	if len(deletedPodId) == 0 {
		t.Fatalf("expected pod to be deleted")
	}
}

func TestHandlePodTerminatedNotOk(t *testing.T) {
	var updatedDeployment *deployapi.Deployment

	controller := &DeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			DeletePodFunc: func(namespace, id string) error {
				t.Fatalf("unexpected delete of pod %s", id)
				return nil
			},
		},
		ContainerCreator: &testContainerCreator{
			CreateContainerFunc: func(strategy *deployapi.DeploymentStrategy) *kapi.Container {
				return basicContainer()
			},
		},
		NextDeployment:  func() *deployapi.Deployment { return nil },
		NextPod:         func() *kapi.Pod { return failedPod() },
		DeploymentStore: deploytest.NewFakeDeploymentStore(runningDeployment()),
	}

	controller.HandlePod()

	if updatedDeployment == nil {
		t.Fatalf("Expected a deployment to be updated")
	}

	if e, a := deployapi.DeploymentStatusFailed, updatedDeployment.Status; e != a {
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
	UpdateDeploymentFunc func(deployment *deployapi.Deployment) (*deployapi.Deployment, error)
}

func (i *testDcDeploymentInterface) UpdateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	return i.UpdateDeploymentFunc(deployment)
}

type testDcPodInterface struct {
	CreatePodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePodFunc func(namespace, id string) error
}

func (i *testDcPodInterface) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.CreatePodFunc(namespace, pod)
}

func (i *testDcPodInterface) DeletePod(namespace, id string) error {
	return i.DeletePodFunc(namespace, id)
}

func basicDeployment() *deployapi.Deployment {
	return &deployapi.Deployment{
		ObjectMeta: kapi.ObjectMeta{Name: "deploy1"},
		Status:     deployapi.DeploymentStatusNew,
		Strategy: deployapi.DeploymentStrategy{
			Type: deployapi.DeploymentStrategyTypeRecreate,
		},
		ControllerTemplate: kapi.ReplicationControllerState{
			PodTemplate: kapi.PodTemplate{
				DesiredState: kapi.PodState{
					Manifest: kapi.ContainerManifest{
						Containers: []kapi.Container{
							{
								Name:  "container1",
								Image: "registry:8080/repo1:ref1",
							},
						},
					},
				},
			},
		},
	}
}

func pendingDeployment() *deployapi.Deployment {
	d := basicDeployment()
	d.Status = deployapi.DeploymentStatusPending
	return d
}

func runningDeployment() *deployapi.Deployment {
	d := basicDeployment()
	d.Status = deployapi.DeploymentStatusRunning
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
		CurrentState: kapi.PodState{
			Info: kapi.PodInfo{
				"container1": kapi.ContainerStatus{},
			},
		},
	}
}

func succeededPod() *kapi.Pod {
	p := basicPod()
	p.CurrentState.Status = kapi.PodSucceeded
	return p
}

func failedPod() *kapi.Pod {
	p := basicPod()
	p.CurrentState.Status = kapi.PodFailed
	p.CurrentState.Info["container1"] = kapi.ContainerStatus{
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
	p.CurrentState.Status = kapi.PodRunning
	return p
}
