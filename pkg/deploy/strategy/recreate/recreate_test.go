package recreate

import (
	"fmt"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	api "github.com/openshift/origin/pkg/api/latest"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestFirstDeployment(t *testing.T) {
	var updatedController *kapi.ReplicationController
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)

	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		client: &testControllerClient{
			getReplicationControllerFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return deployment, nil
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

func TestSecondDeploymentSuccessfulRetries(t *testing.T) {
	updatedControllers := make(map[string]*kapi.ReplicationController)
	oldDeployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	newConfig := deploytest.OkDeploymentConfig(2)
	newDeployment, _ := deployutil.MakeDeployment(newConfig, kapi.Codec)

	errorCounts := map[string]int{}

	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		client: &testControllerClient{
			getReplicationControllerFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				switch name {
				case oldDeployment.Name:
					return oldDeployment, nil
				case newDeployment.Name:
					return newDeployment, nil
				default:
					t.Fatalf("unexpected call to getReplicationController: %s/%s", namespace, name)
					return nil, nil
				}
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				if errorCounts[ctrl.Name] < 3 {
					errorCounts[ctrl.Name] = errorCounts[ctrl.Name] + 1
					return nil, fmt.Errorf("test error %d", errorCounts[ctrl.Name])
				}
				updatedControllers[ctrl.Name] = ctrl
				return ctrl, nil
			},
		},
	}

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

func TestSecondDeploymentFailedInitialRetries(t *testing.T) {
	updatedControllers := make(map[string]*kapi.ReplicationController)
	oldDeployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	newConfig := deploytest.OkDeploymentConfig(2)
	newDeployment, _ := deployutil.MakeDeployment(newConfig, kapi.Codec)

	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Millisecond,
		retryPeriod:  1 * time.Millisecond,
		client: &testControllerClient{
			getReplicationControllerFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				switch name {
				case oldDeployment.Name:
					return oldDeployment, nil
				case newDeployment.Name:
					return newDeployment, nil
				default:
					t.Fatalf("unexpected call to getReplicationController: %s/%s", namespace, name)
					return nil, nil
				}
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return nil, fmt.Errorf("update failure")
			},
		},
	}

	err := strategy.Deploy(newDeployment, []kapi.ObjectReference{
		{
			Namespace: oldDeployment.Namespace,
			Name:      oldDeployment.Name,
		},
	})

	if err == nil {
		t.Fatalf("expected a deploy error: %#v", err)
	}

	if len(updatedControllers) > 0 {
		t.Fatalf("unexpected controller updates: %v", updatedControllers)
	}
}

func TestSecondDeploymentFailedDisableRetries(t *testing.T) {
	updatedControllers := make(map[string]*kapi.ReplicationController)
	oldDeployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	newConfig := deploytest.OkDeploymentConfig(2)
	newDeployment, _ := deployutil.MakeDeployment(newConfig, kapi.Codec)

	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Millisecond,
		retryPeriod:  1 * time.Millisecond,
		client: &testControllerClient{
			getReplicationControllerFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				switch name {
				case oldDeployment.Name:
					return oldDeployment, nil
				case newDeployment.Name:
					return newDeployment, nil
				default:
					t.Fatalf("unexpected call to getReplicationController: %s/%s", namespace, name)
					return nil, nil
				}
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				switch ctrl.Name {
				case newDeployment.Name:
					return newDeployment, nil
				case oldDeployment.Name:
					return nil, fmt.Errorf("update error")
				default:
					t.Fatalf("unexpected call to getReplicationController: %s/%s", namespace, ctrl.Name)
					return nil, nil
				}
			},
		},
	}

	err := strategy.Deploy(newDeployment, []kapi.ObjectReference{
		{
			Namespace: oldDeployment.Namespace,
			Name:      oldDeployment.Name,
		},
	})

	if err == nil {
		t.Fatalf("expcted a deploy error: %#v", err)
	}

	if len(updatedControllers) > 0 {
		t.Fatalf("unexpected controller updates: %v", updatedControllers)
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
