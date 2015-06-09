package deployerpod

import (
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

// TestHandle_podTerminatedFail ensures that a failed deployer pod results in
// a transition of the deployment's status to failed.
func TestHandle_podTerminatedFail(t *testing.T) {
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

	err := controller.Handle(failedPod())

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

// TestHandle_podTerminatedFailNoContainerStatus ensures that a failed
// deployer pod with no container status results in a transition of the
// deployment's status to failed.
func TestHandle_podTerminatedFailNoContainerStatus(t *testing.T) {
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
