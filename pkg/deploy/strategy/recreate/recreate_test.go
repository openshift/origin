package recreate

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	api "github.com/openshift/origin/pkg/api/latest"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestFirstDeployment(t *testing.T) {
	var updatedController *kapi.ReplicationController
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	strategy := &DeploymentStrategy{
		Codec: api.Codec,
		ReplicationController: &testControllerClient{
			getReplicationControllerFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to getReplicationController")
				return nil, nil
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedController = ctrl
				return ctrl, nil
			},
		},
	}

	err := strategy.Deploy(deployment, []kapi.ObjectReference{})

	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if updatedController == nil {
		t.Fatalf("expected a ReplicationController")
	}

	if e, a := 1, updatedController.Spec.Replicas; e != a {
		t.Fatalf("expected controller replicas to be %d, got %d", e, a)
	}
}

func TestSecondDeployment(t *testing.T) {
	updatedControllers := make(map[string]*kapi.ReplicationController)
	oldDeployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	strategy := &DeploymentStrategy{
		Codec: api.Codec,
		ReplicationController: &testControllerClient{
			getReplicationControllerFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				if e, a := oldDeployment.Namespace, namespace; e != a {
					t.Fatalf("expected getReplicationController call with %s, got %s", e, a)
				}

				if e, a := oldDeployment.Name, name; e != a {
					t.Fatalf("expected getReplicationController call with %s, got %s", e, a)
				}
				return oldDeployment, nil
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedControllers[ctrl.Name] = ctrl
				return ctrl, nil
			},
		},
	}

	newConfig := deploytest.OkDeploymentConfig(2)
	newDeployment, _ := deployutil.MakeDeployment(newConfig, kapi.Codec)

	err := strategy.Deploy(newDeployment, []kapi.ObjectReference{
		{
			Namespace: oldDeployment.Namespace,
			Name:      oldDeployment.Name,
		},
	})

	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if e, a := 0, updatedControllers[oldDeployment.Name].Spec.Replicas; e != a {
		t.Fatalf("expected old controller replicas to be %d, got %d", e, a)
	}

	if e, a := 1, updatedControllers[newDeployment.Name].Spec.Replicas; e != a {
		t.Fatalf("expected new controller replicas to be %d, got %d", e, a)
	}
}

type testControllerClient struct {
	getReplicationControllerFunc    func(namespace, name string) (*kapi.ReplicationController, error)
	updateReplicationControllerFunc func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (t *testControllerClient) getReplicationController(namespace, name string) (*kapi.ReplicationController, error) {
	return t.getReplicationControllerFunc(namespace, name)
}

func (t *testControllerClient) updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return t.updateReplicationControllerFunc(namespace, ctrl)
}
