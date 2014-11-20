package recreate

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func TestFirstDeployment(t *testing.T) {
	var (
		createdController *kapi.ReplicationController
		deployment        = okDeployment()
	)

	strategy := &RecreateDeploymentStrategy{
		ReplicationController: &testControllerClient{
			listReplicationControllersFunc: func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{}, nil
			},
			createReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				createdController = ctrl
				return ctrl, nil
			},
			deleteReplicationControllerFunc: func(namespaceBravo, id string) error {
				t.Fatalf("unexpected call to DeleteReplicationController")
				return nil
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to UpdateReplicationController")
				return nil, nil
			},
		},
	}

	err := strategy.Deploy(deployment)

	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if createdController == nil {
		t.Fatalf("expected a ReplicationController")
	}

	if e, a := "deploy1", createdController.Annotations[deployapi.DeploymentAnnotation]; e != a {
		t.Fatalf("expected controller deployment annotation %s, git %s", e, a)
	}

	if e, a := "deploymentConfig1", createdController.Labels[deployapi.DeploymentConfigLabel]; e != a {
		t.Fatalf("expected controller with label %s, got %s", e, a)
	}

	if e, a := "deploymentConfig1", createdController.DesiredState.PodTemplate.Labels[deployapi.DeploymentConfigLabel]; e != a {
		t.Fatalf("expected controller podtemplate label %s, got %s", e, a)
	}

	if e, a := 2, createdController.DesiredState.Replicas; e != a {
		t.Fatalf("expected controller replicas to be %d, got %d", e, a)
	}
}

func TestSecondDeployment(t *testing.T) {
	var (
		createdController   *kapi.ReplicationController
		updatedController   *kapi.ReplicationController
		deletedControllerID string
		deployment          = okDeployment()
	)

	strategy := &RecreateDeploymentStrategy{
		ReplicationController: &testControllerClient{
			listReplicationControllersFunc: func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{
					Items: []kapi.ReplicationController{
						okReplicationController(),
					},
				}, nil
			},
			createReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				createdController = ctrl
				return ctrl, nil
			},
			deleteReplicationControllerFunc: func(namespace, id string) error {
				deletedControllerID = id
				return nil
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedController = ctrl
				return ctrl, nil
			},
		},
	}

	deployment.Name = "deploy2"
	err := strategy.Deploy(deployment)

	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if createdController == nil {
		t.Fatalf("expected a ReplicationController")
	}

	if e, a := "deploy2", createdController.Annotations[deployapi.DeploymentAnnotation]; e != a {
		t.Fatalf("expected controller deployment annotation %s, git %s", e, a)
	}

	if e, a := "deploymentConfig1", createdController.Labels[deployapi.DeploymentConfigLabel]; e != a {
		t.Fatalf("expected controller with label %s, got %s", e, a)
	}

	if e, a := "deploymentConfig1", createdController.DesiredState.PodTemplate.Labels[deployapi.DeploymentConfigLabel]; e != a {
		t.Fatalf("expected controller podtemplate label %s, got %s", e, a)
	}

	if e, a := 0, updatedController.DesiredState.Replicas; e != a {
		t.Fatalf("expected old controller replicas to be %d, got %d", e, a)
	}

	if e, a := "controller1", deletedControllerID; e != a {
		t.Fatalf("expected deletion of controller %s, got %s", e, a)
	}
}

type testControllerClient struct {
	listReplicationControllersFunc  func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error)
	getReplicationControllerFunc    func(namespace, id string) (*kapi.ReplicationController, error)
	createReplicationControllerFunc func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
	updateReplicationControllerFunc func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
	deleteReplicationControllerFunc func(namespace, id string) error
}

func (t *testControllerClient) listReplicationControllers(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
	return t.listReplicationControllersFunc(namespace, selector)
}

func (t *testControllerClient) getReplicationController(namespace, id string) (*kapi.ReplicationController, error) {
	return t.getReplicationControllerFunc(namespace, id)
}

func (t *testControllerClient) createReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return t.createReplicationControllerFunc(namespace, ctrl)
}

func (t *testControllerClient) updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return t.updateReplicationControllerFunc(namespace, ctrl)
}

func (t *testControllerClient) deleteReplicationController(namespace, id string) error {
	return t.deleteReplicationControllerFunc(namespace, id)
}

func okDeployment() *deployapi.Deployment {
	return &deployapi.Deployment{
		ObjectMeta: kapi.ObjectMeta{
			Name: "deploy1",
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation: "deploymentConfig1",
			},
		},
		Status: deployapi.DeploymentStatusNew,
		Strategy: deployapi.DeploymentStrategy{
			Type: deployapi.DeploymentStrategyTypeRecreate,
		},
		ControllerTemplate: kapi.ReplicationControllerState{
			Replicas: 2,
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

func okReplicationController() kapi.ReplicationController {
	return kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name:        "controller1",
			Annotations: map[string]string{deployapi.DeploymentAnnotation: "deploy1"},
			Labels: map[string]string{
				deployapi.DeploymentConfigLabel: "deploymentConfig1",
			},
		},
		DesiredState: kapi.ReplicationControllerState{
			Replicas: 1,
			PodTemplate: kapi.PodTemplate{
				Labels: map[string]string{
					deployapi.DeploymentConfigLabel: "deploymentConfig1",
				},
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
