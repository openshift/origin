package deployerpod

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// TestHandle_uncorrelatedPod ensures that pods uncorrelated with a deployment
// are ignored.
func TestHandle_uncorrelatedPod(t *testing.T) {
	controller := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected deployment update")
				return nil, nil
			},
		},
	}

	// Verify no-op
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	pod := runningPod(deployment)
	pod.Annotations = make(map[string]string)
	err := controller.Handle(pod)

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// TestHandle_orphanedPod ensures that deployer pods associated with a non-
// existent deployment results in all deployer pods being deleted.
func TestHandle_orphanedPod(t *testing.T) {
	deleted := sets.NewString()
	controller := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("Unexpected deployment update")
				return nil, nil
			},
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("ReplicationController", name)
			},
		},
		deployerPodsFor: func(namespace, name string) (*kapi.PodList, error) {
			mkpod := func(suffix string) kapi.Pod {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
				p := okPod(deployment)
				p.Name = p.Name + suffix
				return *p
			}
			return &kapi.PodList{
				Items: []kapi.Pod{
					mkpod(""),
					mkpod("-prehook"),
					mkpod("-posthook"),
				},
			}, nil
		},
		deletePod: func(namespace, name string) error {
			deleted.Insert(name)
			return nil
		},
	}

	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	err := controller.Handle(runningPod(deployment))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deployerName := deployutil.DeployerPodNameForDeployment(deployment.Name)
	if !deleted.HasAll(deployerName, deployerName+"-prehook", deployerName+"-posthook") {
		t.Fatalf("unexpected deleted names: %v", deleted.List())
	}
}

// TestHandle_runningPod ensures that a running deployer pod results in a
// transition of the deployment's status to running.
func TestHandle_runningPod(t *testing.T) {
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusPending)
	var updatedDeployment *kapi.ReplicationController

	controller := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return deployment, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
	}

	err := controller.Handle(runningPod(deployment))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatalf("expected deployment update")
	}

	if e, a := deployapi.DeploymentStatusRunning, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

// TestHandle_podTerminatedOk ensures that a successfully completed deployer
// pod results in a transition of the deployment's status to complete.
func TestHandle_podTerminatedOk(t *testing.T) {
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)
	var updatedDeployment *kapi.ReplicationController

	controller := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return deployment, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
	}

	err := controller.Handle(succeededPod(deployment))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatalf("expected deployment update")
	}

	if e, a := deployapi.DeploymentStatusComplete, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

// TestHandle_podTerminatedFailNoContainerStatus ensures that a failed
// deployer pod with no container status results in a transition of the
// deployment's status to failed.
func TestHandle_podTerminatedFailNoContainerStatus(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	// since we do not set the desired replicas annotation,
	// this also tests that the error is just logged and not result in a failure
	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)

	controller := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return deployment, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{Items: []kapi.ReplicationController{*deployment}}, nil
			},
		},
	}

	err := controller.Handle(terminatedPod(deployment))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatalf("expected deployment update")
	}

	if e, a := deployapi.DeploymentStatusFailed, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected updated deployment status %s, got %s", e, a)
	}
}

// TestHandle_cleanupDesiredReplicasAnnotation ensures that the desired replicas annotation
// will be cleaned up in a complete deployment and stay around in a failed deployment
func TestHandle_cleanupDesiredReplicasAnnotation(t *testing.T) {
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	tests := []struct {
		name     string
		pod      *kapi.Pod
		expected bool
	}{
		{
			name:     "complete deployment - cleaned up annotation",
			pod:      succeededPod(deployment),
			expected: false,
		},
		{
			name:     "failed deployment - annotation stays",
			pod:      terminatedPod(deployment),
			expected: true,
		},
	}

	for _, test := range tests {
		var updatedDeployment *kapi.ReplicationController
		deployment.Annotations[deployapi.DesiredReplicasAnnotation] = "1"

		controller := &DeployerPodController{
			deploymentClient: &deploymentClientImpl{
				getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
					return deployment, nil
				},
				updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
					updatedDeployment = deployment
					return deployment, nil
				},
				listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
					return &kapi.ReplicationControllerList{Items: []kapi.ReplicationController{*deployment}}, nil
				},
			},
		}

		if err := controller.Handle(test.pod); err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		if updatedDeployment == nil {
			t.Errorf("%s: expected deployment update", test.name)
			continue
		}

		if _, got := updatedDeployment.Annotations[deployapi.DesiredReplicasAnnotation]; got != test.expected {
			t.Errorf("%s: expected annotation: %t, got %t", test.name, test.expected, got)
		}
	}
}

func okPod(deployment *kapi.ReplicationController) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: deployutil.DeployerPodNameForDeployment(deployment.Name),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
		},
		Status: kapi.PodStatus{
			ContainerStatuses: []kapi.ContainerStatus{
				{},
			},
		},
	}
}

func succeededPod(deployment *kapi.ReplicationController) *kapi.Pod {
	p := okPod(deployment)
	p.Status.Phase = kapi.PodSucceeded
	return p
}

func failedPod(deployment *kapi.ReplicationController) *kapi.Pod {
	p := okPod(deployment)
	p.Status.Phase = kapi.PodFailed
	p.Status.ContainerStatuses = []kapi.ContainerStatus{
		{
			State: kapi.ContainerState{
				Terminated: &kapi.ContainerStateTerminated{
					ExitCode: 1,
				},
			},
		},
	}
	return p
}

func terminatedPod(deployment *kapi.ReplicationController) *kapi.Pod {
	p := okPod(deployment)
	p.Status.Phase = kapi.PodFailed
	return p
}

func runningPod(deployment *kapi.ReplicationController) *kapi.Pod {
	p := okPod(deployment)
	p.Status.Phase = kapi.PodRunning
	return p
}
