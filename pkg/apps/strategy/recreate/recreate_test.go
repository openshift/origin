package recreate

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	scalefake "k8s.io/client-go/scale/fake"
	clientgotesting "k8s.io/client-go/testing"

	appsv1 "github.com/openshift/api/apps/v1"
	appsstrategy "github.com/openshift/origin/pkg/apps/strategy"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	appstest "github.com/openshift/origin/pkg/apps/util/test"
)

func getUpdateAcceptor(timeout time.Duration, minReadySeconds int32) appsstrategy.UpdateAcceptor {
	return &testAcceptor{
		acceptFn: func(deployment *corev1.ReplicationController) error {
			return nil
		},
	}
}

func recreateParams(timeout int64, preFailurePolicy, midFailurePolicy, postFailurePolicy appsv1.LifecycleHookFailurePolicy) appsv1.DeploymentStrategy {
	var pre, mid, post *appsv1.LifecycleHook
	if len(preFailurePolicy) > 0 {
		pre = &appsv1.LifecycleHook{
			FailurePolicy: preFailurePolicy,
			ExecNewPod:    &appsv1.ExecNewPodHook{},
		}
	}
	if len(midFailurePolicy) > 0 {
		mid = &appsv1.LifecycleHook{
			FailurePolicy: midFailurePolicy,
			ExecNewPod:    &appsv1.ExecNewPodHook{},
		}
	}
	if len(postFailurePolicy) > 0 {
		post = &appsv1.LifecycleHook{
			FailurePolicy: postFailurePolicy,
			ExecNewPod:    &appsv1.ExecNewPodHook{},
		}
	}
	return appsv1.DeploymentStrategy{
		Type: appsv1.DeploymentStrategyTypeRecreate,
		RecreateParams: &appsv1.RecreateDeploymentStrategyParams{
			TimeoutSeconds: &timeout,
			Pre:            pre,
			Mid:            mid,
			Post:           post,
		},
	}
}

type testAcceptor struct {
	acceptFn func(*corev1.ReplicationController) error
}

func (t *testAcceptor) Accept(deployment *corev1.ReplicationController) error {
	return t.acceptFn(deployment)
}

type fakeControllerClient struct {
	deployment *corev1.ReplicationController
	fakeClient *fake.Clientset

	scaleEvents []*autoscalingv1.Scale
}

func (c *fakeControllerClient) ReplicationControllers(ns string) kcoreclient.ReplicationControllerInterface {
	return c.fakeClient.Core().ReplicationControllers(ns)
}

func (c *fakeControllerClient) scaledOnce() bool {
	return len(c.scaleEvents) == 1
}

func (c *fakeControllerClient) fakeScaleClient() *scalefake.FakeScaleClient {
	scaleFakeClient := &scalefake.FakeScaleClient{}
	scaleFakeClient.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		obj := &autoscalingv1.Scale{}
		obj.Status.Replicas = c.deployment.Status.Replicas
		return true, obj, nil
	})
	scaleFakeClient.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		updateAction := action.(clientgotesting.UpdateAction)
		scaleObj := updateAction.GetObject().(*autoscalingv1.Scale)
		c.scaleEvents = append(c.scaleEvents, scaleObj)
		c.deployment.Spec.Replicas = &scaleObj.Spec.Replicas
		c.deployment.Status.Replicas = scaleObj.Spec.Replicas
		return true, scaleObj, nil
	})
	return scaleFakeClient
}

func newFakeControllerClient(deployment *corev1.ReplicationController) *fakeControllerClient {
	c := &fakeControllerClient{deployment: deployment}
	c.fakeClient = fake.NewSimpleClientset(c.deployment)
	return c
}

type fakePodClient struct {
	deployerName string
}

func (c *fakePodClient) Pods(ns string) kcoreclient.PodInterface {
	deployerPod := &corev1.Pod{}
	deployerPod.Name = c.deployerName
	deployerPod.Namespace = ns
	deployerPod.Status = corev1.PodStatus{}
	return fake.NewSimpleClientset(deployerPod).CoreV1().Pods(ns)
}

type hookExecutorImpl struct {
	executeFunc func(hook *appsv1.LifecycleHook, deployment *corev1.ReplicationController, suffix, label string) error
}

func (h *hookExecutorImpl) Execute(hook *appsv1.LifecycleHook, rc *corev1.ReplicationController, suffix, label string) error {
	return h.executeFunc(hook, rc, suffix, label)
}

func TestRecreate_initialDeployment(t *testing.T) {
	var deployment *corev1.ReplicationController
	strategy := &RecreateDeploymentStrategy{
		out:               &bytes.Buffer{},
		errOut:            &bytes.Buffer{},
		getUpdateAcceptor: getUpdateAcceptor,
		eventClient:       fake.NewSimpleClientset().Core(),
	}

	config := appstest.OkDeploymentConfig(1)
	config.Spec.Strategy = recreateParams(30, "", "", "")
	deployment, _ = appsutil.MakeDeployment(config)

	controllerClient := newFakeControllerClient(deployment)
	strategy.rcClient = controllerClient
	strategy.scaleClient = controllerClient.fakeScaleClient()
	strategy.podClient = &fakePodClient{deployerName: appsutil.DeployerPodNameForDeployment(deployment.Name)}

	err := strategy.Deploy(nil, deployment, 3)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if !controllerClient.scaledOnce() {
		t.Fatalf("expected 1 scale calls, got %d", len(controllerClient.scaleEvents))
	}
}

func TestRecreate_deploymentPreHookSuccess(t *testing.T) {
	config := appstest.OkDeploymentConfig(1)
	config.Spec.Strategy = recreateParams(30, appsv1.LifecycleHookFailurePolicyAbort, "", "")
	deployment, _ := appsutil.MakeDeployment(config)
	controllerClient := newFakeControllerClient(deployment)

	hookExecuted := false
	strategy := &RecreateDeploymentStrategy{
		out:               &bytes.Buffer{},
		errOut:            &bytes.Buffer{},
		getUpdateAcceptor: getUpdateAcceptor,
		eventClient:       fake.NewSimpleClientset().Core(),
		rcClient:          controllerClient,
		scaleClient:       controllerClient.fakeScaleClient(),
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *appsv1.LifecycleHook, deployment *corev1.ReplicationController, suffix, label string) error {
				hookExecuted = true
				return nil
			},
		},
	}
	strategy.podClient = &fakePodClient{deployerName: appsutil.DeployerPodNameForDeployment(deployment.Name)}

	err := strategy.Deploy(nil, deployment, 2)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}
	if !hookExecuted {
		t.Fatalf("expected hook execution")
	}
}

func TestRecreate_deploymentPreHookFail(t *testing.T) {
	config := appstest.OkDeploymentConfig(1)
	config.Spec.Strategy = recreateParams(30, appsv1.LifecycleHookFailurePolicyAbort, "", "")
	deployment, _ := appsutil.MakeDeployment(config)
	controllerClient := newFakeControllerClient(deployment)

	strategy := &RecreateDeploymentStrategy{
		out:               &bytes.Buffer{},
		errOut:            &bytes.Buffer{},
		getUpdateAcceptor: getUpdateAcceptor,
		eventClient:       fake.NewSimpleClientset().Core(),
		rcClient:          controllerClient,
		scaleClient:       controllerClient.fakeScaleClient(),
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *appsv1.LifecycleHook, deployment *corev1.ReplicationController, suffix, label string) error {
				return fmt.Errorf("hook execution failure")
			},
		},
	}
	strategy.podClient = &fakePodClient{deployerName: appsutil.DeployerPodNameForDeployment(deployment.Name)}

	err := strategy.Deploy(nil, deployment, 2)
	if err == nil {
		t.Fatalf("expected a deploy error")
	}

	if len(controllerClient.scaleEvents) > 0 {
		t.Fatalf("unexpected scaling events: %v", controllerClient.scaleEvents)
	}
}

func TestRecreate_deploymentMidHookSuccess(t *testing.T) {
	config := appstest.OkDeploymentConfig(1)
	config.Spec.Strategy = recreateParams(30, "", appsv1.LifecycleHookFailurePolicyAbort, "")
	deployment, _ := appsutil.MakeDeployment(config)
	controllerClient := newFakeControllerClient(deployment)

	strategy := &RecreateDeploymentStrategy{
		out:               &bytes.Buffer{},
		errOut:            &bytes.Buffer{},
		rcClient:          controllerClient,
		scaleClient:       controllerClient.fakeScaleClient(),
		eventClient:       fake.NewSimpleClientset().Core(),
		getUpdateAcceptor: getUpdateAcceptor,
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *appsv1.LifecycleHook, deployment *corev1.ReplicationController, suffix, label string) error {
				return fmt.Errorf("hook execution failure")
			},
		},
	}
	strategy.podClient = &fakePodClient{deployerName: appsutil.DeployerPodNameForDeployment(deployment.Name)}

	err := strategy.Deploy(nil, deployment, 2)
	if err == nil {
		t.Fatalf("expected a deploy error")
	}

	if len(controllerClient.scaleEvents) > 0 {
		t.Fatalf("unexpected scaling events: %v", controllerClient.scaleEvents)
	}
}

func TestRecreate_deploymentPostHookSuccess(t *testing.T) {
	config := appstest.OkDeploymentConfig(1)
	config.Spec.Strategy = recreateParams(30, "", "", appsv1.LifecycleHookFailurePolicyAbort)
	deployment, _ := appsutil.MakeDeployment(config)
	controllerClient := newFakeControllerClient(deployment)

	hookExecuted := false
	strategy := &RecreateDeploymentStrategy{
		out:               &bytes.Buffer{},
		errOut:            &bytes.Buffer{},
		rcClient:          controllerClient,
		scaleClient:       controllerClient.fakeScaleClient(),
		eventClient:       fake.NewSimpleClientset().Core(),
		getUpdateAcceptor: getUpdateAcceptor,
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *appsv1.LifecycleHook, deployment *corev1.ReplicationController, suffix, label string) error {
				hookExecuted = true
				return nil
			},
		},
	}
	strategy.podClient = &fakePodClient{deployerName: appsutil.DeployerPodNameForDeployment(deployment.Name)}

	err := strategy.Deploy(nil, deployment, 2)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}
	if !hookExecuted {
		t.Fatalf("expected hook execution")
	}
}

func TestRecreate_deploymentPostHookFail(t *testing.T) {
	config := appstest.OkDeploymentConfig(1)
	config.Spec.Strategy = recreateParams(30, "", "", appsv1.LifecycleHookFailurePolicyAbort)
	deployment, _ := appsutil.MakeDeployment(config)
	controllerClient := newFakeControllerClient(deployment)

	hookExecuted := false
	strategy := &RecreateDeploymentStrategy{
		out:               &bytes.Buffer{},
		errOut:            &bytes.Buffer{},
		rcClient:          controllerClient,
		scaleClient:       controllerClient.fakeScaleClient(),
		eventClient:       fake.NewSimpleClientset().Core(),
		getUpdateAcceptor: getUpdateAcceptor,
		hookExecutor: &hookExecutorImpl{
			executeFunc: func(hook *appsv1.LifecycleHook, deployment *corev1.ReplicationController, suffix, label string) error {
				hookExecuted = true
				return fmt.Errorf("post hook failure")
			},
		},
	}
	strategy.podClient = &fakePodClient{deployerName: appsutil.DeployerPodNameForDeployment(deployment.Name)}

	err := strategy.Deploy(nil, deployment, 2)
	if err == nil {
		t.Fatalf("unexpected non deploy error: %#v", err)
	}
	if !hookExecuted {
		t.Fatalf("expected hook execution")
	}
}

func TestRecreate_acceptorSuccess(t *testing.T) {
	var deployment *corev1.ReplicationController
	strategy := &RecreateDeploymentStrategy{
		out:         &bytes.Buffer{},
		errOut:      &bytes.Buffer{},
		eventClient: fake.NewSimpleClientset().CoreV1(),
	}

	acceptorCalled := false
	acceptor := &testAcceptor{
		acceptFn: func(deployment *corev1.ReplicationController) error {
			acceptorCalled = true
			return nil
		},
	}

	oldDeployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1))
	deployment, _ = appsutil.MakeDeployment(appstest.OkDeploymentConfig(2))
	controllerClient := newFakeControllerClient(deployment)
	strategy.rcClient = controllerClient
	strategy.scaleClient = controllerClient.fakeScaleClient()
	strategy.podClient = &fakePodClient{deployerName: appsutil.DeployerPodNameForDeployment(deployment.Name)}

	err := strategy.DeployWithAcceptor(oldDeployment, deployment, 2, acceptor)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if !acceptorCalled {
		t.Fatalf("expected acceptor to be called")
	}

	if len(controllerClient.scaleEvents) != 2 {
		t.Fatalf("expected 2 scale calls, got %d", len(controllerClient.scaleEvents))
	}
	if r := controllerClient.scaleEvents[0].Spec.Replicas; r != 1 {
		t.Fatalf("expected first scale event to be 1 replica, got %d", r)
	}

	if r := controllerClient.scaleEvents[1].Spec.Replicas; r != 2 {
		t.Fatalf("expected second scale event to be 2 replica, got %d", r)
	}
}

func TestRecreate_acceptorSuccessWithColdCaches(t *testing.T) {
	var deployment *corev1.ReplicationController
	strategy := &RecreateDeploymentStrategy{
		out:         &bytes.Buffer{},
		errOut:      &bytes.Buffer{},
		eventClient: fake.NewSimpleClientset().Core(),
	}

	acceptorCalled := false
	acceptor := &testAcceptor{
		acceptFn: func(deployment *corev1.ReplicationController) error {
			acceptorCalled = true
			return nil
		},
	}

	oldDeployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1))
	deployment, _ = appsutil.MakeDeployment(appstest.OkDeploymentConfig(2))
	controllerClient := newFakeControllerClient(deployment)

	strategy.rcClient = controllerClient
	strategy.scaleClient = controllerClient.fakeScaleClient()
	strategy.podClient = &fakePodClient{deployerName: appsutil.DeployerPodNameForDeployment(deployment.Name)}

	err := strategy.DeployWithAcceptor(oldDeployment, deployment, 2, acceptor)
	if err != nil {
		t.Fatalf("unexpected deploy error: %#v", err)
	}

	if !acceptorCalled {
		t.Fatalf("expected acceptor to be called")
	}

	if len(controllerClient.scaleEvents) != 2 {
		t.Fatalf("expected 2 scale calls, got %d", len(controllerClient.scaleEvents))
	}
	if r := controllerClient.scaleEvents[0]; r.Spec.Replicas != 1 {
		t.Errorf("expected first scale event to be 1 replica, got %v", r)
	}
	if r := controllerClient.scaleEvents[1]; r.Spec.Replicas != 2 {
		t.Errorf("expected second scale event to be 2 replica, got %v", r)
	}
}

func TestRecreate_acceptorFail(t *testing.T) {
	var deployment *corev1.ReplicationController

	strategy := &RecreateDeploymentStrategy{
		out:         &bytes.Buffer{},
		errOut:      &bytes.Buffer{},
		eventClient: fake.NewSimpleClientset().Core(),
	}

	acceptor := &testAcceptor{
		acceptFn: func(deployment *corev1.ReplicationController) error {
			return fmt.Errorf("rejected")
		},
	}

	oldDeployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1))
	deployment, _ = appsutil.MakeDeployment(appstest.OkDeploymentConfig(2))
	rcClient := newFakeControllerClient(deployment)
	strategy.rcClient = rcClient
	strategy.scaleClient = rcClient.fakeScaleClient()
	strategy.podClient = &fakePodClient{deployerName: appsutil.DeployerPodNameForDeployment(deployment.Name)}
	err := strategy.DeployWithAcceptor(oldDeployment, deployment, 2, acceptor)
	if err == nil {
		t.Fatalf("expected a deployment failure")
	}
	t.Logf("got expected error: %v", err)

	if len(rcClient.scaleEvents) != 1 {
		t.Fatalf("expected 1 scale calls, got %d", len(rcClient.scaleEvents))
	}
	if r := rcClient.scaleEvents[0]; r.Spec.Replicas != 1 {
		t.Errorf("expected first scale event to be 1 replica, got %v", r)
	}
}
