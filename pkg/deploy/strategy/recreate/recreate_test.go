package recreate

import (
	"fmt"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestRecreate_initialDeployment(t *testing.T) {
	var updatedController *kapi.ReplicationController
	var deployment *kapi.ReplicationController

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

	// Deployment replicas should follow the config as there's no explicit
	// desired annotation.
	deployment, _ = deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	err := strategy.Deploy(deployment, []*kapi.ReplicationController{})
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}
	if e, a := 1, updatedController.Spec.Replicas; e != a {
		t.Fatalf("expected controller replicas to be %d, got %d", e, a)
	}

	// Deployment replicas should follow the explicit annotation.
	deployment, _ = deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	deployment.Annotations[deployapi.DesiredReplicasAnnotation] = "2"
	err = strategy.Deploy(deployment, []*kapi.ReplicationController{})
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}
	if e, a := 2, updatedController.Spec.Replicas; e != a {
		t.Fatalf("expected controller replicas to be %d, got %d", e, a)
	}

	// Deployment replicas should follow the config as the explicit value is
	// invalid.
	deployment, _ = deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	deployment.Annotations[deployapi.DesiredReplicasAnnotation] = "invalid"
	err = strategy.Deploy(deployment, []*kapi.ReplicationController{})
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}
	if e, a := 1, updatedController.Spec.Replicas; e != a {
		t.Fatalf("expected controller replicas to be %d, got %d", e, a)
	}
}

func TestRecreate_secondDeploymentWithSuccessfulRetries(t *testing.T) {
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

	err := strategy.Deploy(newDeployment, []*kapi.ReplicationController{oldDeployment})

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

func TestRecreate_secondDeploymentScaleUpRetries(t *testing.T) {
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

	err := strategy.Deploy(newDeployment, []*kapi.ReplicationController{oldDeployment})

	if err == nil {
		t.Fatalf("expected a deploy error: %#v", err)
	}

	if len(updatedControllers) > 0 {
		t.Fatalf("unexpected controller updates: %v", updatedControllers)
	}
}

func TestRecreate_secondDeploymentScaleDownRetries(t *testing.T) {
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

	err := strategy.Deploy(newDeployment, []*kapi.ReplicationController{oldDeployment})

	if err == nil {
		t.Fatalf("expected a deploy error: %#v", err)
	}

	if len(updatedControllers) > 0 {
		t.Fatalf("unexpected controller updates: %v", updatedControllers)
	}
}

func TestRecreate_deploymentPreHookSuccess(t *testing.T) {
	var updatedController *kapi.ReplicationController
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams(deployapi.LifecycleHookFailurePolicyAbort, "")
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)

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
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
				return nil
			},
		},
	}

	err := strategy.Deploy(deployment, []*kapi.ReplicationController{})

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

func TestRecreate_deploymentPreHookFail(t *testing.T) {
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams(deployapi.LifecycleHookFailurePolicyAbort, "")
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)

	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		client: &testControllerClient{
			getReplicationControllerFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to getReplicationController")
				return deployment, nil
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to updateReplicationController")
				return ctrl, nil
			},
		},
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
				return fmt.Errorf("hook execution failure")
			},
		},
	}

	err := strategy.Deploy(deployment, []*kapi.ReplicationController{})
	if err == nil {
		t.Fatalf("expected deploy error: %v", err)
	}
}

func TestRecreate_deploymentPostHookSuccess(t *testing.T) {
	var updatedController *kapi.ReplicationController
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams("", deployapi.LifecycleHookFailurePolicyAbort)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)

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
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
				return nil
			},
		},
	}

	err := strategy.Deploy(deployment, []*kapi.ReplicationController{})

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

func TestRecreate_deploymentPostHookFailureIgnored(t *testing.T) {
	var updatedController *kapi.ReplicationController
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams("", deployapi.LifecycleHookFailurePolicyIgnore)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)

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
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
				return fmt.Errorf("hook execution failure")
			},
		},
	}

	err := strategy.Deploy(deployment, []*kapi.ReplicationController{})

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

func recreateParams(preFailurePolicy, postFailurePolicy deployapi.LifecycleHookFailurePolicy) *deployapi.RecreateDeploymentStrategyParams {
	var pre *deployapi.LifecycleHook
	var post *deployapi.LifecycleHook

	if len(preFailurePolicy) > 0 {
		pre = &deployapi.LifecycleHook{
			FailurePolicy: preFailurePolicy,
			ExecNewPod:    &deployapi.ExecNewPodHook{},
		}
	}
	if len(postFailurePolicy) > 0 {
		post = &deployapi.LifecycleHook{
			FailurePolicy: postFailurePolicy,
			ExecNewPod:    &deployapi.ExecNewPodHook{},
		}
	}
	return &deployapi.RecreateDeploymentStrategyParams{
		Pre:  pre,
		Post: post,
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
