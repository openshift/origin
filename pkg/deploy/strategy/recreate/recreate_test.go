package recreate

import (
	"fmt"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apimachinery/registered"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	scalertest "github.com/openshift/origin/pkg/deploy/scaler/test"
	"github.com/openshift/origin/pkg/deploy/strategy"
	deployutil "github.com/openshift/origin/pkg/deploy/util"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestRecreate_initialDeployment(t *testing.T) {
	var deployment *kapi.ReplicationController
	scaler := &scalertest.FakeScaler{}

	strategy := &RecreateDeploymentStrategy{
		decoder:      kapi.Codecs.UniversalDecoder(),
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		getUpdateAcceptor: getUpdateAcceptor,
		scaler:            scaler,
	}

	config := deploytest.OkDeploymentConfig(1)
	config.Spec.Strategy = recreateParams(30, "", "", "")
	deployment, _ = deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	err := strategy.Deploy(nil, deployment, 3)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if e, a := 2, len(scaler.Events); e != a {
		t.Fatalf("expected %d scale calls, got %d", e, a)
	}
	if e, a := uint(1), scaler.Events[0].Size; e != a {
		t.Errorf("expected scale up to %d, got %d", e, a)
	}
	if e, a := uint(3), scaler.Events[1].Size; e != a {
		t.Errorf("expected scale up to %d, got %d", e, a)
	}
}

func TestRecreate_deploymentPreHookSuccess(t *testing.T) {
	config := deploytest.OkDeploymentConfig(1)
	config.Spec.Strategy = recreateParams(30, deployapi.LifecycleHookFailurePolicyAbort, "", "")
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	scaler := &scalertest.FakeScaler{}

	hookExecuted := false
	strategy := &RecreateDeploymentStrategy{
		decoder:      kapi.Codecs.UniversalDecoder(),
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		getUpdateAcceptor: getUpdateAcceptor,
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
	config.Spec.Strategy = recreateParams(30, deployapi.LifecycleHookFailurePolicyAbort, "", "")
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	scaler := &scalertest.FakeScaler{}

	strategy := &RecreateDeploymentStrategy{
		decoder:      kapi.Codecs.UniversalDecoder(),
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		getUpdateAcceptor: getUpdateAcceptor,
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

func TestRecreate_deploymentMidHookSuccess(t *testing.T) {
	config := deploytest.OkDeploymentConfig(1)
	config.Spec.Strategy = recreateParams(30, "", deployapi.LifecycleHookFailurePolicyAbort, "")
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
	scaler := &scalertest.FakeScaler{}

	hookExecuted := false
	strategy := &RecreateDeploymentStrategy{
		decoder:      kapi.Codecs.UniversalDecoder(),
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		getUpdateAcceptor: getUpdateAcceptor,
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

func TestRecreate_deploymentMidHookFail(t *testing.T) {
	config := deploytest.OkDeploymentConfig(1)
	config.Spec.Strategy = recreateParams(30, "", deployapi.LifecycleHookFailurePolicyAbort, "")
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
	scaler := &scalertest.FakeScaler{}

	strategy := &RecreateDeploymentStrategy{
		decoder:      kapi.Codecs.UniversalDecoder(),
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		getUpdateAcceptor: getUpdateAcceptor,
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
	config.Spec.Strategy = recreateParams(30, "", "", deployapi.LifecycleHookFailurePolicyAbort)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	scaler := &scalertest.FakeScaler{}

	hookExecuted := false
	strategy := &RecreateDeploymentStrategy{
		decoder:      kapi.Codecs.UniversalDecoder(),
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		getUpdateAcceptor: getUpdateAcceptor,
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
	config.Spec.Strategy = recreateParams(30, "", "", deployapi.LifecycleHookFailurePolicyAbort)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	scaler := &scalertest.FakeScaler{}

	hookExecuted := false
	strategy := &RecreateDeploymentStrategy{
		decoder:      kapi.Codecs.UniversalDecoder(),
		retryTimeout: 1 * time.Second,
		retryPeriod:  1 * time.Millisecond,
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return deployment, nil
		},
		getUpdateAcceptor: getUpdateAcceptor,
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
		decoder:      kapi.Codecs.UniversalDecoder(),
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

	deployment, _ = deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	err := strategy.DeployWithAcceptor(nil, deployment, 2, acceptor)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if !acceptorCalled {
		t.Fatalf("expected acceptor to be called")
	}

	if e, a := 2, len(scaler.Events); e != a {
		t.Fatalf("expected %d scale calls, got %d", e, a)
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
		decoder:      kapi.Codecs.UniversalDecoder(),
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

	deployment, _ = deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	err := strategy.DeployWithAcceptor(nil, deployment, 2, acceptor)
	if err == nil {
		t.Fatalf("expected a deployment failure")
	}
	t.Logf("got expected error: %v", err)

	if e, a := 1, len(scaler.Events); e != a {
		t.Fatalf("expected %d scale calls, got %d", e, a)
	}
	if e, a := uint(1), scaler.Events[0].Size; e != a {
		t.Errorf("expected scale up to %d, got %d", e, a)
	}
}

func recreateParams(timeout int64, preFailurePolicy, midFailurePolicy, postFailurePolicy deployapi.LifecycleHookFailurePolicy) deployapi.DeploymentStrategy {
	var pre, mid, post *deployapi.LifecycleHook
	if len(preFailurePolicy) > 0 {
		pre = &deployapi.LifecycleHook{
			FailurePolicy: preFailurePolicy,
			ExecNewPod:    &deployapi.ExecNewPodHook{},
		}
	}
	if len(midFailurePolicy) > 0 {
		mid = &deployapi.LifecycleHook{
			FailurePolicy: midFailurePolicy,
			ExecNewPod:    &deployapi.ExecNewPodHook{},
		}
	}
	if len(postFailurePolicy) > 0 {
		post = &deployapi.LifecycleHook{
			FailurePolicy: postFailurePolicy,
			ExecNewPod:    &deployapi.ExecNewPodHook{},
		}
	}
	return deployapi.DeploymentStrategy{
		Type: deployapi.DeploymentStrategyTypeRecreate,
		RecreateParams: &deployapi.RecreateDeploymentStrategyParams{
			TimeoutSeconds: &timeout,

			Pre:  pre,
			Mid:  mid,
			Post: post,
		},
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

func getUpdateAcceptor(timeout time.Duration) strategy.UpdateAcceptor {
	return &testAcceptor{
		acceptFn: func(deployment *kapi.ReplicationController) error {
			return nil
		},
	}
}

type testAcceptor struct {
	acceptFn func(*kapi.ReplicationController) error
}

func (t *testAcceptor) Accept(deployment *kapi.ReplicationController) error {
	return t.acceptFn(deployment)
}
