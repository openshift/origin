package recreate

import (
	"fmt"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	scalertest "github.com/openshift/origin/pkg/deploy/scaler/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestRecreate_initialDeployment(t *testing.T) {
	var deployment *kapi.ReplicationController
	scaler := &scalertest.FakeScaler{}

	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		scaler: scaler,
	}

	deployment, _ = deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	err := strategy.Deploy(nil, deployment, 2)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if e, a := 1, len(scaler.Events); e != a {
		t.Fatalf("expected %s scale calls, got %d", e, a)
	}
	if e, a := uint(2), scaler.Events[0].Size; e != a {
		t.Errorf("expected scale up to %d, got %d", e, a)
	}
}

func TestRecreate_deploymentPreHookSuccess(t *testing.T) {
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams(deployapi.LifecycleHookFailurePolicyAbort, "")
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	scaler := &scalertest.FakeScaler{}

	hookExecuted := false
	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
				hookExecuted = true
				return nil
			},
		},
		scaler: scaler,
	}

	err := strategy.Deploy(nil, deployment, 2)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}
	if !hookExecuted {
		t.Fatalf("expected hook execution")
	}
}

func TestRecreate_deploymentPreHookFail(t *testing.T) {
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams(deployapi.LifecycleHookFailurePolicyAbort, "")
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	scaler := &scalertest.FakeScaler{}

	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
				return fmt.Errorf("hook execution failure")
			},
		},
		scaler: scaler,
	}

	err := strategy.Deploy(nil, deployment, 2)
	if err == nil {
		t.Fatalf("expected a deploy error")
	}
	if len(scaler.Events) > 0 {
		t.Fatalf("unexpected scaling events: %v", scaler.Events)
	}
}

func TestRecreate_deploymentPostHookSuccess(t *testing.T) {
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams("", deployapi.LifecycleHookFailurePolicyAbort)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	scaler := &scalertest.FakeScaler{}

	hookExecuted := false
	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
				hookExecuted = true
				return nil
			},
		},
		scaler: scaler,
	}

	err := strategy.Deploy(nil, deployment, 2)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}
	if !hookExecuted {
		t.Fatalf("expected hook execution")
	}
}

func TestRecreate_deploymentPostHookFail(t *testing.T) {
	config := deploytest.OkDeploymentConfig(1)
	config.Template.Strategy.RecreateParams = recreateParams("", deployapi.LifecycleHookFailurePolicyAbort)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	scaler := &scalertest.FakeScaler{}

	hookExecuted := false
	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
				hookExecuted = true
				return fmt.Errorf("post hook failure")
			},
		},
		scaler: scaler,
	}

	err := strategy.Deploy(nil, deployment, 2)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}
	if !hookExecuted {
		t.Fatalf("expected hook execution")
	}
}

func TestRecreate_acceptorSuccess(t *testing.T) {
	var deployment *kapi.ReplicationController
	scaler := &scalertest.FakeScaler{}

	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		scaler: scaler,
	}

	acceptorCalled := false
	acceptor := &testAcceptor{
		acceptFn: func(deployment *kapi.ReplicationController) error {
			acceptorCalled = true
			return nil
		},
	}

	deployment, _ = deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	err := strategy.DeployWithAcceptor(nil, deployment, 2, acceptor)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if !acceptorCalled {
		t.Fatalf("expected acceptor to be called")
	}

	if e, a := 2, len(scaler.Events); e != a {
		t.Fatalf("expected %s scale calls, got %d", e, a)
	}
	if e, a := uint(1), scaler.Events[0].Size; e != a {
		t.Errorf("expected scale up to %d, got %d", e, a)
	}
	if e, a := uint(2), scaler.Events[1].Size; e != a {
		t.Errorf("expected scale up to %d, got %d", e, a)
	}
}

func TestRecreate_acceptorFail(t *testing.T) {
	var deployment *kapi.ReplicationController
	scaler := &scalertest.FakeScaler{}

	strategy := &RecreateDeploymentStrategy{
		codec:        api.Codec,
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		scaler: scaler,
	}

	acceptor := &testAcceptor{
		acceptFn: func(deployment *kapi.ReplicationController) error {
			return fmt.Errorf("rejected")
		},
	}

	deployment, _ = deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
	err := strategy.DeployWithAcceptor(nil, deployment, 2, acceptor)
	if err == nil {
		t.Fatalf("expected a deployment failure")
	}
	t.Logf("got expected error: %v", err)

	if e, a := 1, len(scaler.Events); e != a {
		t.Fatalf("expected %s scale calls, got %d", e, a)
	}
	if e, a := uint(1), scaler.Events[0].Size; e != a {
		t.Errorf("expected scale up to %d, got %d", e, a)
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

type testAcceptor struct {
	acceptFn func(*kapi.ReplicationController) error
}

func (t *testAcceptor) Accept(deployment *kapi.ReplicationController) error {
	return t.acceptFn(deployment)
}
