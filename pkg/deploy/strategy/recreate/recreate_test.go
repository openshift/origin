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
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return nil
			},
		},
	}

	err := strategy.Deploy(deployment, nil, []kapi.ObjectReference{})

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

func TestRecreate_secondDeploymentPreserveReplicas(t *testing.T) {
	var lastDeployment *kapi.ReplicationController
	oldDeployments := map[string]*kapi.ReplicationController{}

	expectedUpdates := map[string]int{}
	actualUpdates := map[string]int{}

	for i := 1; i <= 5; i++ {
		old, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(i), kapi.Codec)
		old.Spec.Replicas = i
		oldDeployments[old.Name] = old
		lastDeployment = old
		expectedUpdates[old.Name] = 0
	}

	newDeployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(6), kapi.Codec)
	expectedUpdates[newDeployment.Name] = 5

	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		client: &testControllerClient{
			getReplicationControllerFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				if name == newDeployment.Name {
					return newDeployment, nil
				}
				c, exists := oldDeployments[name]
				if !exists {
					t.Fatalf("unexpected call to getReplicationController: %s/%s", namespace, name)
				}
				return c, nil
			},
			updateReplicationControllerFunc: func(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				actualUpdates[ctrl.Name] = ctrl.Spec.Replicas
				return ctrl, nil
			},
		},
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return nil
			},
		},
	}

	old := []*kapi.ReplicationController{}
	for _, d := range oldDeployments {
		old = append(old, d)
	}

	err := strategy.Deploy(newDeployment, refFor(lastDeployment), refsFor(old...))
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	for name, expectedReplicas := range expectedUpdates {
		actualReplicas, updated := actualUpdates[name]
		if !updated {
			t.Errorf("expected an update for %s", name)
		}
		if expectedReplicas != actualReplicas {
			t.Errorf("expected replicas for %s: %d, got %d", name, expectedReplicas, actualReplicas)
		}
	}
}

func TestRecreate_secondDeploymentWithSuccessfulRetries(t *testing.T) {
	updatedControllers := make(map[string]*kapi.ReplicationController)
	oldDeployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	oldDeployment.Spec.Replicas = 1
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
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return nil
			},
		},
	}

	err := strategy.Deploy(newDeployment, refFor(oldDeployment), refsFor(oldDeployment))

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
	oldDeployment.Spec.Replicas = 1
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
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return nil
			},
		},
	}

	err := strategy.Deploy(newDeployment, refFor(oldDeployment), refsFor(oldDeployment))

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
	oldDeployment.Spec.Replicas = 1
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
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return nil
			},
		},
	}

	err := strategy.Deploy(newDeployment, refFor(oldDeployment), refsFor(oldDeployment))

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
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return nil
			},
		},
	}

	err := strategy.Deploy(deployment, nil, []kapi.ObjectReference{})

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

func TestRecreate_deploymentPreHookFailAbort(t *testing.T) {
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
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return fmt.Errorf("hook execution failure")
			},
		},
	}

	err := strategy.Deploy(deployment, nil, []kapi.ObjectReference{})
	if err == nil {
		t.Fatalf("expected a deploy error")
	}
	t.Logf("got expected error: %s", err)
}

func TestRecreate_deploymentPreHookFailureIgnored(t *testing.T) {
	var updatedController *kapi.ReplicationController
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams(deployapi.LifecycleHookFailurePolicyIgnore, "")
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
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return fmt.Errorf("hook execution failure")
			},
		},
	}

	err := strategy.Deploy(deployment, nil, []kapi.ObjectReference{})

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

func TestRecreate_deploymentPreHookFailureRetried(t *testing.T) {
	var updatedController *kapi.ReplicationController
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams(deployapi.LifecycleHookFailurePolicyRetry, "")
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)

	errorCount := 2
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
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				if errorCount == 0 {
					return nil
				}
				errorCount--
				return fmt.Errorf("hook execution failure")
			},
		},
	}

	err := strategy.Deploy(deployment, nil, []kapi.ObjectReference{})

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
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return nil
			},
		},
	}

	err := strategy.Deploy(deployment, nil, []kapi.ObjectReference{})

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

func TestRecreate_deploymentPostHookAbortUnsupported(t *testing.T) {
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
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return fmt.Errorf("hook execution failure")
			},
		},
	}

	err := strategy.Deploy(deployment, nil, []kapi.ObjectReference{})

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

func TestRecreate_deploymentPostHookFailIgnore(t *testing.T) {
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
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				return fmt.Errorf("hook execution failure")
			},
		},
	}

	err := strategy.Deploy(deployment, nil, []kapi.ObjectReference{})

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

func TestRecreate_deploymentPostHookFailureRetried(t *testing.T) {
	var updatedController *kapi.ReplicationController
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams("", deployapi.LifecycleHookFailurePolicyRetry)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)

	errorCount := 2
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
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
				if errorCount == 0 {
					return nil
				}
				errorCount--
				return fmt.Errorf("hook execution failure")
			},
		},
	}

	err := strategy.Deploy(deployment, nil, []kapi.ObjectReference{})

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

func refFor(deployment *kapi.ReplicationController) *kapi.ObjectReference {
	return &kapi.ObjectReference{
		Namespace: deployment.Namespace,
		Name:      deployment.Name,
	}
}

func refsFor(deployments ...*kapi.ReplicationController) []kapi.ObjectReference {
	refs := []kapi.ObjectReference{}
	for _, deployment := range deployments {
		refs = append(refs, *refFor(deployment))
	}
	return refs
}
