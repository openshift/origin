package controller

import (
  "testing"

  kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
  deployapi "github.com/openshift/origin/pkg/deploy/api"
)

type testDcDeploymentInterface struct {
  UpdateDeploymentFunc func(deployment *deployapi.Deployment) (*deployapi.Deployment, error)
}

func (i *testDcDeploymentInterface) UpdateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
  return i.UpdateDeploymentFunc(deployment)
}

type testDcPodInterface struct {
  GetPodFunc    func(id string) (*kapi.Pod, error)
  CreatePodFunc func(pod *kapi.Pod) (*kapi.Pod, error)
  DeletePodFunc func(id string) error
}

func (i *testDcPodInterface) GetPod(ctx kapi.Context, id string) (*kapi.Pod, error) {
  return i.GetPodFunc(id)
}

func (i *testDcPodInterface) CreatePod(ctx kapi.Context, pod *kapi.Pod) (*kapi.Pod, error) {
  return i.CreatePodFunc(pod)
}

func (i *testDcPodInterface) DeletePod(ctx kapi.Context, id string) error {
  return i.DeletePodFunc(id)
}

func TestHandleNewDeployment(t *testing.T) {
  var updatedDeployment *deployapi.Deployment
  var newPod *kapi.Pod

  controller := &DeploymentController{
    DeploymentInterface: &testDcDeploymentInterface{
      UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
        updatedDeployment = deployment
        return deployment, nil
      },
    },
    PodInterface: &testDcPodInterface{
      CreatePodFunc: func(pod *kapi.Pod) (*kapi.Pod, error) {
        newPod = pod
        return pod, nil
      },
    },
    NextDeployment: func() *deployapi.Deployment {
      deployment := basicDeployment()
      deployment.State = deployapi.DeploymentStateNew
      return deployment
    },
  }

  // Verify new -> pending
  controller.HandleDeployment()

  if newPod == nil {
    t.Fatal("expected a new pod to be created")
  }

  if updatedDeployment == nil {
    t.Fatal("expected an updated deployment")
  }

  if e, a := deployapi.DeploymentStatePending, updatedDeployment.State; e != a {
    t.Fatalf("expected new deployment state %s, got %s", e, a)
  }
}

func TestHandlePendingDeploymentPendingPod(t *testing.T) {
  controller := &DeploymentController{
    DeploymentInterface: &testDcDeploymentInterface{
      UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
        t.Fatalf("unexpected deployment update")
        return nil, nil
      },
    },
    PodInterface: &testDcPodInterface{
      GetPodFunc: func(id string) (*kapi.Pod, error) {
        return &kapi.Pod{
          CurrentState: kapi.PodState{
            Status: kapi.PodWaiting,
          },
        }, nil
      },
    },
    NextDeployment: func() *deployapi.Deployment {
      deployment := basicDeployment()
      deployment.State = deployapi.DeploymentStatePending
      return deployment
    },
  }

  // Verify pending -> pending (no-op) given the pod isn't yet running
  controller.HandleDeployment()
}

func TestHandlePendingDeploymentRunningPod(t *testing.T) {
  var updatedDeployment *deployapi.Deployment

  controller := &DeploymentController{
    DeploymentInterface: &testDcDeploymentInterface{
      UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
        updatedDeployment = deployment
        return deployment, nil
      },
    },
    PodInterface: &testDcPodInterface{
      GetPodFunc: func(id string) (*kapi.Pod, error) {
        return &kapi.Pod{
          CurrentState: kapi.PodState{
            Status: kapi.PodRunning,
          },
        }, nil
      },
    },
    NextDeployment: func() *deployapi.Deployment {
      deployment := basicDeployment()
      deployment.State = deployapi.DeploymentStatePending
      return deployment
    },
  }

  // Verify pending -> running now that the pod is running
  controller.HandleDeployment()

  if updatedDeployment == nil {
    t.Fatalf("expected an updated deployment")
  }

  if e, a := deployapi.DeploymentStateRunning, updatedDeployment.State; e != a {
    t.Fatalf("expected updated deployment state %s, got %s", e, a)
  }
}

func TestHandleRunningDeploymentRunningPod(t *testing.T) {
  controller := &DeploymentController{
    DeploymentInterface: &testDcDeploymentInterface{
      UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
        t.Fatalf("unexpected deployment update")
        return nil, nil
      },
    },
    PodInterface: &testDcPodInterface{
      GetPodFunc: func(id string) (*kapi.Pod, error) {
        return &kapi.Pod{
          CurrentState: kapi.PodState{
            Status: kapi.PodRunning,
          },
        }, nil
      },
    },
    NextDeployment: func() *deployapi.Deployment {
      deployment := basicDeployment()
      deployment.State = deployapi.DeploymentStateRunning
      return deployment
    },
  }

  // Verify no-op
  controller.HandleDeployment()
}

func TestHandleRunningDeploymentTerminatedOkPod(t *testing.T) {
  var updatedDeployment *deployapi.Deployment
  podDeleted := false

  controller := &DeploymentController{
    DeploymentInterface: &testDcDeploymentInterface{
      UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
        updatedDeployment = deployment
        return deployment, nil
      },
    },
    PodInterface: &testDcPodInterface{
      GetPodFunc: func(id string) (*kapi.Pod, error) {
        return terminatedPod(0), nil
      },
      DeletePodFunc: func(id string) error {
        podDeleted = true
        return nil
      },
    },
    NextDeployment: func() *deployapi.Deployment {
      deployment := basicDeployment()
      deployment.State = deployapi.DeploymentStateRunning
      return deployment
    },
  }

  // Verify running -> complete since the pod terminated with a 0 exit code
  controller.HandleDeployment()

  if updatedDeployment == nil {
    t.Fatalf("expected an updated deployment")
  }

  if e, a := deployapi.DeploymentStateComplete, updatedDeployment.State; e != a {
    t.Fatalf("expected updated deployment state %s, got %s", e, a)
  }

  if !podDeleted {
    t.Fatalf("expected pod to be deleted")
  }
}

func TestHandleRunningDeploymentTerminatedFailedPod(t *testing.T) {
  var updatedDeployment *deployapi.Deployment

  controller := &DeploymentController{
    DeploymentInterface: &testDcDeploymentInterface{
      UpdateDeploymentFunc: func(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
        updatedDeployment = deployment
        return deployment, nil
      },
    },
    PodInterface: &testDcPodInterface{
      GetPodFunc: func(id string) (*kapi.Pod, error) {
        return terminatedPod(1), nil
      },
      DeletePodFunc: func(id string) error {
        t.Fatalf("unexpected delete of pod %s", id)
        return nil
      },
    },
    NextDeployment: func() *deployapi.Deployment {
      deployment := basicDeployment()
      deployment.State = deployapi.DeploymentStateRunning
      return deployment
    },
  }

  // Verify running -> failed since the pod terminated with a nonzero exit code
  controller.HandleDeployment()

  if updatedDeployment == nil {
    t.Fatalf("expected an updated deployment")
  }

  if e, a := deployapi.DeploymentStateFailed, updatedDeployment.State; e != a {
    t.Fatalf("expected updated deployment state %s, got %s", e, a)
  }
}

func basicDeployment() *deployapi.Deployment {
  return &deployapi.Deployment{
    JSONBase: kapi.JSONBase{ID: "deploy1"},
    State:    deployapi.DeploymentStateNew,
    Strategy: deployapi.DeploymentStrategy{
      Type: "customPod",
      CustomPod: &deployapi.CustomPodDeploymentStrategy{
        Image:       "registry:8080/repo1:ref1",
        Environment: []kapi.EnvVar{},
      },
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

func terminatedPod(exitCode int) *kapi.Pod {
  return &kapi.Pod{
    CurrentState: kapi.PodState{
      Status: kapi.PodTerminated,
      Info: kapi.PodInfo{
        "container1": kapi.ContainerStatus{
          State: kapi.ContainerState{
            Termination: &kapi.ContainerStateTerminated{
              ExitCode: exitCode,
            },
          },
        },
      },
    },
  }
}
