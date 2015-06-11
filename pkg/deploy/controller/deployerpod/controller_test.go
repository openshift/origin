package deployerpod

import (
	"fmt"
	"strconv"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

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
	pod := runningPod()
	pod.Annotations = make(map[string]string)
	err := controller.Handle(pod)

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// TestHandle_orphanedPod ensures that deployer pods associated with a non-
// existent deployment result in an error.
func TestHandle_orphanedPod(t *testing.T) {
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
	}

	err := controller.Handle(runningPod())

	if err == nil {
		t.Fatalf("expected an error")
	}
}

// TestHandle_runningPod ensures that a running deployer pod results in a
// transition of the deployment's status to running.
func TestHandle_runningPod(t *testing.T) {
	var updatedDeployment *kapi.ReplicationController

	controller := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				config := deploytest.OkDeploymentConfig(1)
				deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
				deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusPending)
				return deployment, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
	}

	err := controller.Handle(runningPod())

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
	var updatedDeployment *kapi.ReplicationController

	controller := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				config := deploytest.OkDeploymentConfig(1)
				deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
				deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusRunning)
				return deployment, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployment = deployment
				return deployment, nil
			},
		},
	}

	err := controller.Handle(succeededPod())

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
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
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

	err := controller.Handle(terminatedPod())

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

	err := controller.Handle(terminatedPod())

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
		// No previous deployments
		{1, 3, []existing{
			{1, deployapi.DeploymentStatusRunning, 3, 0},
		}},
		// Multiple existing deployments - none in complete state
		{3, 2, []existing{
			{1, deployapi.DeploymentStatusFailed, 2, 2},
			{2, deployapi.DeploymentStatusFailed, 0, 0},
			{3, deployapi.DeploymentStatusRunning, 2, 0},
		}},
		// Failed deployment is already at 0 replicas
		{3, 2, []existing{
			{1, deployapi.DeploymentStatusFailed, 2, 2},
			{2, deployapi.DeploymentStatusFailed, 0, 0},
			{3, deployapi.DeploymentStatusRunning, 0, 0},
		}},
		// Multiple existing completed deployments
		{4, 2, []existing{
			{3, deployapi.DeploymentStatusComplete, 0, 2},
			{2, deployapi.DeploymentStatusComplete, 0, 0},
			{4, deployapi.DeploymentStatusRunning, 1, 0},
			{1, deployapi.DeploymentStatusFailed, 0, 0},
		}},
		// A deployment already exists after the current failed deployment
		// only the current deployment is marked as failed
		// the completed deployment is not scaled up
		{4, 2, []existing{
			{3, deployapi.DeploymentStatusComplete, 1, 1},
			{2, deployapi.DeploymentStatusComplete, 0, 0},
			{4, deployapi.DeploymentStatusRunning, 2, 0},
			{5, deployapi.DeploymentStatusNew, 0, 0},
			{1, deployapi.DeploymentStatusFailed, 0, 0},
		}},
	}

	for _, scenario := range scenarios {
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
		err := controller.Handle(terminatedPod())

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

func okPod() *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: "deploy-deploy1",
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: "1234",
			},
		},
		Status: kapi.PodStatus{
			ContainerStatuses: []kapi.ContainerStatus{
				{},
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
	p.Status.ContainerStatuses = []kapi.ContainerStatus{
		{
			State: kapi.ContainerState{
				Termination: &kapi.ContainerStateTerminated{
					ExitCode: 1,
				},
			},
		},
	}
	return p
}

func terminatedPod() *kapi.Pod {
	p := okPod()
	p.Status.Phase = kapi.PodFailed
	return p
}

func runningPod() *kapi.Pod {
	p := okPod()
	p.Status.Phase = kapi.PodRunning
	return p
}
