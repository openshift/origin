package recreate

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestFirstDeployment(t *testing.T) {
	var (
		updatedController *kapi.ReplicationController
		deployment        = okDeployment(okDeploymentConfig())
	)

	strategy := &DeploymentStrategy{
		Codec: api.Codec,
		ReplicationController: &testControllerClient{
			listReplicationControllersFunc: func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{}, nil
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedController = ctrl
				return ctrl, nil
			},
			deleteReplicationControllerFunc: func(namespaceBravo, id string) error {
				t.Fatalf("unexpected call to DeleteReplicationController")
				return nil
			},
		},
	}

	err := strategy.Deploy(deployment)

	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if updatedController == nil {
		t.Fatalf("expected a ReplicationController")
	}

	if e, a := 2, updatedController.Spec.Replicas; e != a {
		t.Fatalf("expected controller replicas to be %d, got %d", e, a)
	}
}

func TestSecondDeployment(t *testing.T) {
	var deletedControllerID string
	updatedControllers := make(map[string]*kapi.ReplicationController)
	oldDeployment := okDeployment(okDeploymentConfig())

	strategy := &DeploymentStrategy{
		Codec: api.Codec,
		ReplicationController: &testControllerClient{
			listReplicationControllersFunc: func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{
					Items: []kapi.ReplicationController{
						*oldDeployment,
					},
				}, nil
			},
			deleteReplicationControllerFunc: func(namespace, id string) error {
				deletedControllerID = id
				return nil
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedControllers[ctrl.Name] = ctrl
				return ctrl, nil
			},
		},
	}

	newConfig := okDeploymentConfig()
	newConfig.LatestVersion = 2
	newDeployment := okDeployment(newConfig)

	err := strategy.Deploy(newDeployment)

	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if e, a := 0, updatedControllers[oldDeployment.Name].Spec.Replicas; e != a {
		t.Fatalf("expected old controller replicas to be %d, got %d", e, a)
	}

	if e, a := oldDeployment.Name, deletedControllerID; e != a {
		t.Fatalf("expected deletion of controller %s, got %s", e, a)
	}

	if e, a := 2, updatedControllers[newDeployment.Name].Spec.Replicas; e != a {
		t.Fatalf("expected new controller replicas to be %d, got %d", e, a)
	}
}

type testControllerClient struct {
	listReplicationControllersFunc  func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error)
	getReplicationControllerFunc    func(namespace, id string) (*kapi.ReplicationController, error)
	updateReplicationControllerFunc func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
	deleteReplicationControllerFunc func(namespace, id string) error
}

func (t *testControllerClient) listReplicationControllers(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
	return t.listReplicationControllersFunc(namespace, selector)
}

func (t *testControllerClient) getReplicationController(namespace, id string) (*kapi.ReplicationController, error) {
	return t.getReplicationControllerFunc(namespace, id)
}

func (t *testControllerClient) updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return t.updateReplicationControllerFunc(namespace, ctrl)
}

func (t *testControllerClient) deleteReplicationController(namespace, id string) error {
	return t.deleteReplicationControllerFunc(namespace, id)
}

func okDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta:    kapi.ObjectMeta{Name: "deploymentConfig"},
		LatestVersion: 1,
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 2,
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
		},
	}
}

func okDeployment(config *deployapi.DeploymentConfig) *kapi.ReplicationController {
	encodedConfig, _ := deployutil.EncodeDeploymentConfig(config, api.Codec)
	controller := &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: deployutil.LatestDeploymentIDForConfig(config),
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation:        config.Name,
				deployapi.DeploymentStatusAnnotation:        string(deployapi.DeploymentStatusNew),
				deployapi.DeploymentEncodedConfigAnnotation: encodedConfig,
			},
			Labels: config.Labels,
		},
		Spec: config.Template.ControllerTemplate,
	}

	controller.Spec.Replicas = 0
	return controller
}
