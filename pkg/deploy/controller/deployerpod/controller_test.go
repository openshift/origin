package deployerpod

import (
	"fmt"
	"strconv"
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

// TestHandle_deploymentCleanupTransientError ensures that a failure
// to clean up a failed deployment results in a transient error
// and the deployment status is not set to Failed.
func TestHandle_deploymentCleanupTransientError(t *testing.T) {
	completedDeployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	completedDeployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
	currentDeployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(2), kapi.Codec)
	currentDeployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)
	currentDeployment.Annotations[deployapi.DesiredReplicasAnnotation] = "2"

	controller := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return currentDeployment, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				// simulate failure ONLY for the completed deployment
				if deployutil.DeploymentStatusFor(deployment) == deployapi.DeploymentStatusComplete {
					return nil, fmt.Errorf("test failure in updating completed deployment")
				}
				return deployment, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{Items: []kapi.ReplicationController{*currentDeployment, *completedDeployment}}, nil
			},
		},
	}

	err := controller.Handle(terminatedPod(currentDeployment))

	if err == nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, isTransient := err.(transientError); !isTransient {
		t.Fatalf("expected transientError on failure to update deployment")
	}

	if e, a := deployapi.DeploymentStatusRunning, deployutil.DeploymentStatusFor(currentDeployment); e != a {
		t.Fatalf("expected updated deployment status to remain %s, got %s", e, a)
	}
}

// TestHandle_cleanupDeploymentFailure ensures that clean up happens
// for the deployment if the deployer pod fails.
//  - failed deployment is scaled down
//  - the last completed deployment is scaled back up
func TestHandle_cleanupDeploymentFailure(t *testing.T) {
	var existingDeployments *kapi.ReplicationControllerList
	var failedDeployment *kapi.ReplicationController
	// map of deployment-version to updated replicas
	var updatedDeployments map[int]*kapi.ReplicationController

	controller := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return failedDeployment, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				if _, found := updatedDeployments[deployutil.DeploymentVersionFor(deployment)]; found {
					t.Fatalf("unexpected multiple updates for deployment #%d", deployutil.DeploymentVersionFor(deployment))
				}
				updatedDeployments[deployutil.DeploymentVersionFor(deployment)] = deployment
				return deployment, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return existingDeployments, nil
			},
		},
	}

	type existing struct {
		version         int
		status          deployapi.DeploymentStatus
		initialReplicas int
		updatedReplicas int
	}

	type scenario struct {
		name string
		// this is the deployment that is passed to Handle
		version int
		// this is the target replicas for the deployment that failed
		desiredReplicas int
		// existing deployments also include the one being handled currently
		existing []existing
	}

	// existing deployments intentionally placed un-ordered
	// in order to verify sorting
	scenarios := []scenario{
		{"No previous deployments",
			1, 3, []existing{
				{1, deployapi.DeploymentStatusRunning, 3, 0},
			}},
		{"Multiple existing deployments - none in complete state",
			3, 2, []existing{
				{1, deployapi.DeploymentStatusFailed, 2, 2},
				{2, deployapi.DeploymentStatusFailed, 0, 0},
				{3, deployapi.DeploymentStatusRunning, 2, 0},
			}},
		{"Failed deployment is already at 0 replicas",
			3, 2, []existing{
				{1, deployapi.DeploymentStatusFailed, 2, 2},
				{2, deployapi.DeploymentStatusFailed, 0, 0},
				{3, deployapi.DeploymentStatusRunning, 0, 0},
			}},
		{"Multiple existing completed deployments",
			4, 2, []existing{
				{3, deployapi.DeploymentStatusComplete, 0, 2},
				{2, deployapi.DeploymentStatusComplete, 0, 0},
				{4, deployapi.DeploymentStatusRunning, 1, 0},
				{1, deployapi.DeploymentStatusFailed, 0, 0},
			}},
		// A deployment already exists after the current failed deployment
		// only the current deployment is marked as failed
		// the completed deployment is not scaled up
		{"Deployment exists after current failed",
			4, 2, []existing{
				{3, deployapi.DeploymentStatusComplete, 1, 1},
				{2, deployapi.DeploymentStatusComplete, 0, 0},
				{4, deployapi.DeploymentStatusRunning, 2, 0},
				{5, deployapi.DeploymentStatusNew, 0, 0},
				{1, deployapi.DeploymentStatusFailed, 0, 0},
			}},
	}

	for _, scenario := range scenarios {
		t.Logf("running scenario: %s", scenario.name)
		updatedDeployments = make(map[int]*kapi.ReplicationController)
		failedDeployment = nil
		existingDeployments = &kapi.ReplicationControllerList{}

		for _, e := range scenario.existing {
			d, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(e.version), kapi.Codec)
			d.Annotations[deployapi.DeploymentStatusAnnotation] = string(e.status)
			d.Spec.Replicas = e.initialReplicas
			// if this is the deployment passed to Handle, set the desired replica annotation
			if e.version == scenario.version {
				d.Annotations[deployapi.DesiredReplicasAnnotation] = strconv.Itoa(scenario.desiredReplicas)
				failedDeployment = d
			}
			existingDeployments.Items = append(existingDeployments.Items, *d)
		}
		associatedDeployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(scenario.version), kapi.Codec)
		err := controller.Handle(terminatedPod(associatedDeployment))

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// only the failed and the last completed deployment should be updated
		if len(updatedDeployments) > 2 {
			t.Fatalf("expected to update only the failed and last completed deployment")
		}

		for _, existing := range scenario.existing {
			updatedDeployment, ok := updatedDeployments[existing.version]
			if existing.initialReplicas != existing.updatedReplicas {
				if !ok {
					t.Fatalf("expected deployment #%d to be updated", existing.version)
				}
				if e, a := existing.updatedReplicas, updatedDeployment.Spec.Replicas; e != a {
					t.Fatalf("expected deployment #%d to be scaled to %d, got %d", existing.version, e, a)
				}
			} else if ok && existing.version != scenario.version {
				t.Fatalf("unexpected update for deployment #%d; replicas %d; status: %s", existing.version, updatedDeployment.Spec.Replicas, deployutil.DeploymentStatusFor(updatedDeployment))
			}
		}
		if deployutil.DeploymentStatusFor(updatedDeployments[scenario.version]) != deployapi.DeploymentStatusFailed {
			t.Fatalf("status for deployment #%d expected to be updated to failed; got %s", scenario.version, deployutil.DeploymentStatusFor(updatedDeployments[scenario.version]))
		}
		if updatedDeployments[scenario.version].Spec.Replicas != 0 {
			t.Fatalf("deployment #%d expected to be scaled down to 0; got %d", scenario.version, updatedDeployments[scenario.version].Spec.Replicas)
		}
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
