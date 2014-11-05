package controller

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/controller/test"
)

type testDcDeploymentInterface struct {
	UpdateDeploymentFunc func(deployment *deployapi.Deployment) (*deployapi.Deployment, error)
}

func (i *testDcDeploymentInterface) UpdateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	return i.UpdateDeploymentFunc(deployment)
}

type testDcPodInterface struct {
	CreatePodFunc func(pod *kapi.Pod) (*kapi.Pod, error)
	DeletePodFunc func(id string) error
}

func (i *testDcPodInterface) CreatePod(ctx kapi.Context, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.CreatePodFunc(pod)
}

func (i *testDcPodInterface) DeletePod(ctx kapi.Context, id string) error {
	return i.DeletePodFunc(id)
}

func TestHandleNew(t *testing.T) {
	var (
		updatedDeployment *deployapi.Deployment
		createdPod        *kapi.Pod
	)

	controller := &CustomPodDeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			CreatePodFunc: func(pod *kapi.Pod) (*kapi.Pod, error) {
				createdPod = pod
				return pod, nil
			},
		},
		NextDeployment: func() *deployapi.Deployment {
			deployment := customPodDeployment()
			deployment.Status = deployapi.DeploymentStatusNew
			return deployment
		},
	}

	// Verify pending -> running now that the pod is running
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
}

func TestHandleNewDeploymentWrongType(t *testing.T) {
	controller := &CustomPodDeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				t.Fatalf("Unexpected call to updateDeployment")
				return nil, nil
			},
		},
		PodInterface: &testDcPodInterface{
			CreatePodFunc: func(pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("Unexpected call to createPod")
				return nil, nil
			},
		},
		NextDeployment: func() *deployapi.Deployment {
			deployment := basicDeployment()
			deployment.Status = deployapi.DeploymentStatusNew
			return deployment
		},
	}

	controller.HandleDeployment()
}

func TestHandlePodRunning(t *testing.T) {
	var updatedDeployment *deployapi.Deployment

	controller := &CustomPodDeploymentController{
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

	controller := &CustomPodDeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			DeletePodFunc: func(id string) error {
				deletedPodId = id
				return nil
			},
		},
		NextDeployment:  func() *deployapi.Deployment { return nil },
		NextPod:         func() *kapi.Pod { return terminatedPod(0) },
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

	controller := &CustomPodDeploymentController{
		DeploymentInterface: &testDcDeploymentInterface{
			UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
		PodInterface: &testDcPodInterface{
			DeletePodFunc: func(id string) error {
				t.Fatalf("unexpected delete of pod %s", id)
				return nil
			},
		},
		NextDeployment:  func() *deployapi.Deployment { return nil },
		NextPod:         func() *kapi.Pod { return terminatedPod(1) },
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

func basicDeployment() *deployapi.Deployment {
	return &deployapi.Deployment{
		TypeMeta: kapi.TypeMeta{ID: "deploy1"},
		Status:   deployapi.DeploymentStatusNew,
		Strategy: deployapi.DeploymentStrategy{
			Type: deployapi.DeploymentStrategyTypeBasic,
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

func customPodDeployment() *deployapi.Deployment {
	d := basicDeployment()
	d.Strategy = deployapi.DeploymentStrategy{
		Type: deployapi.DeploymentStrategyTypeCustomPod,
		CustomPod: &deployapi.CustomPodDeploymentStrategy{
			Image:       "registry:8080/repo1:ref1",
			Environment: []kapi.EnvVar{},
		},
	}

	return d
}

func pendingDeployment() *deployapi.Deployment {
	d := customPodDeployment()
	d.Status = deployapi.DeploymentStatusPending
	return d
}

func runningDeployment() *deployapi.Deployment {
	d := customPodDeployment()
	d.Status = deployapi.DeploymentStatusRunning
	return d
}

func basicPod() *kapi.Pod {
	return &kapi.Pod{
		CurrentState: kapi.PodState{
			Info: kapi.PodInfo{
				"container1": kapi.ContainerStatus{},
			},
		},
		Labels: map[string]string{
			"deployment": "1234",
		},
	}
}

func terminatedPod(exitCode int) *kapi.Pod {
	p := basicPod()
	p.CurrentState.Status = kapi.PodTerminated
	p.CurrentState.Info["container1"] = kapi.ContainerStatus{
		State: kapi.ContainerState{
			Termination: &kapi.ContainerStateTerminated{
				ExitCode: exitCode,
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
