package support

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/api/apihelpers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	_ "github.com/openshift/origin/pkg/api/install"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

func init() {
	// Override the streaming handler for the lifecyclePodHook
	streamFn = func(client typedcorev1.PodInterface, hookPodName string) (io.ReadCloser, error) {
		return ioutil.NopCloser(strings.NewReader("sample log\n")), nil
	}
}

func TestRunHookSuccess(t *testing.T) {
	dc := appstest.OkDeploymentConfig(1)
	dc.Namespace = "test"
	rc, err := appsutil.MakeDeploymentV1(dc, legacyscheme.Codecs.LegacyCodec(legacyscheme.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	if err != nil {
		t.Fatalf("unable to make rc: %v", err)
	}
	deployerPod := makeDeployerPod(rc)
	deployerPod.Status.StartTime = &metav1.Time{time.Now()}

	fakeClient := fake.NewSimpleClientset(deployerPod, rc.DeepCopy())

	out := &bytes.Buffer{}
	controller := NewLifecycleHookController(fakeClient, nil, out)
	hookFinished := make(chan struct{})

	go func() {
		defer close(hookFinished)
		if err := controller.RunHook(makeHook(), rc, DeploymentHookTypePre, 10*time.Second); err != nil {
			t.Fatalf("unexpected hook run error: %v", err)
		}
	}()

	hookPodName := apihelpers.GetPodName(rc.Name, DeploymentHookTypePre)
	var hookPod *corev1.Pod

	pollErr := wait.PollImmediate(1*time.Second, 10*time.Second, func() (bool, error) {
		hookPod, err = fakeClient.CoreV1().Pods("test").Get(hookPodName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	})

	if pollErr != nil {
		t.Fatalf("unexpected error while waiting for hook pod: %v", err)
	}

	// Start the hook pod
	updatedPod := hookPod.DeepCopy()
	updatedPod.Status.StartTime = &metav1.Time{time.Now()}
	updatedPod.Status.Phase = corev1.PodRunning
	_, err = fakeClient.CoreV1().Pods("test").Update(updatedPod)
	if err != nil {
		t.Fatalf("unable to make update to pod: %v", err)
	}

	time.Sleep(1 * time.Second)

	// The hook succeeded
	finalPod := updatedPod.DeepCopy()
	finalPod.Status.Phase = corev1.PodSucceeded
	_, err = fakeClient.CoreV1().Pods("test").Update(finalPod)
	if err != nil {
		t.Fatalf("unable to make update to pod: %v", err)
	}

	select {
	case <-hookFinished:
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout while waiting for hook pod to finish")
	}

	hookConsoleOutput := out.String()
	if !strings.Contains(hookConsoleOutput, "sample log") {
		t.Errorf("expected to see 'sample log' in hook hook output, got:\n%s\n", hookConsoleOutput)
	}
}

func TestRunHookFailed(t *testing.T) {
	dc := appstest.OkDeploymentConfig(1)
	dc.Namespace = "test"
	rc, err := appsutil.MakeDeploymentV1(dc, legacyscheme.Codecs.LegacyCodec(legacyscheme.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	if err != nil {
		t.Fatalf("unable to make rc: %v", err)
	}
	deployerPod := makeDeployerPod(rc)
	deployerPod.Status.StartTime = &metav1.Time{time.Now()}

	fakeClient := fake.NewSimpleClientset(deployerPod, rc.DeepCopy())

	out := &bytes.Buffer{}
	controller := NewLifecycleHookController(fakeClient, nil, out)
	hookFinished := make(chan struct{})

	go func() {
		defer close(hookFinished)
		if err := controller.RunHook(makeHook(), rc, DeploymentHookTypePre, 10*time.Second); err == nil {
			t.Fatalf("expected hook run error, got none")
		}
	}()

	hookPodName := apihelpers.GetPodName(rc.Name, DeploymentHookTypePre)
	var hookPod *corev1.Pod

	pollErr := wait.PollImmediate(1*time.Second, 10*time.Second, func() (bool, error) {
		hookPod, err = fakeClient.CoreV1().Pods("test").Get(hookPodName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	})

	if pollErr != nil {
		t.Fatalf("unexpected error while waiting for hook pod: %v", err)
	}

	// Start the hook pod
	updatedPod := hookPod.DeepCopy()
	updatedPod.Status.StartTime = &metav1.Time{time.Now()}
	updatedPod.Status.Phase = corev1.PodRunning
	_, err = fakeClient.CoreV1().Pods("test").Update(updatedPod)
	if err != nil {
		t.Fatalf("unable to make update to pod: %v", err)
	}

	time.Sleep(1 * time.Second)

	// The hook succeeded
	finalPod := updatedPod.DeepCopy()
	finalPod.Status.Phase = corev1.PodFailed
	finalPod.Status.Message = "fake failure"
	_, err = fakeClient.CoreV1().Pods("test").Update(finalPod)
	if err != nil {
		t.Fatalf("unable to make update to pod: %v", err)
	}

	select {
	case <-hookFinished:
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout while waiting for hook pod to finish")
	}

	hookConsoleOutput := out.String()
	if !strings.Contains(hookConsoleOutput, "sample log") {
		t.Errorf("expected to see 'sample log' in hook hook output, got:\n%s\n", hookConsoleOutput)
	}
}
func TestRunHookContainerRestart(t *testing.T) {
	dc := appstest.OkDeploymentConfig(1)
	dc.Namespace = "test"
	rc, err := appsutil.MakeDeploymentV1(dc, legacyscheme.Codecs.LegacyCodec(legacyscheme.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	if err != nil {
		t.Fatalf("unable to make rc: %v", err)
	}
	deployerPod := makeDeployerPod(rc)
	deployerPod.Status.StartTime = &metav1.Time{time.Now()}

	fakeClient := fake.NewSimpleClientset(deployerPod, rc.DeepCopy())

	out := &bytes.Buffer{}
	controller := NewLifecycleHookController(fakeClient, nil, out)
	hookFinished := make(chan struct{})

	go func() {
		defer close(hookFinished)
		if err := controller.RunHook(makeHook(), rc, DeploymentHookTypePre, 10*time.Second); err == nil {
			t.Fatalf("expected hook run error, got none")
		}
	}()

	hookPodName := apihelpers.GetPodName(rc.Name, DeploymentHookTypePre)
	var hookPod *corev1.Pod

	pollErr := wait.PollImmediate(1*time.Second, 10*time.Second, func() (bool, error) {
		hookPod, err = fakeClient.CoreV1().Pods("test").Get(hookPodName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	})

	if pollErr != nil {
		t.Fatalf("unexpected error while waiting for hook pod: %v", err)
	}

	// Start the hook pod
	updatedPod := hookPod.DeepCopy()
	updatedPod.Status.StartTime = &metav1.Time{time.Now()}
	updatedPod.Status.Phase = corev1.PodRunning
	_, err = fakeClient.CoreV1().Pods("test").Update(updatedPod)
	if err != nil {
		t.Fatalf("unable to make update to pod: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Start the hook pod
	restartedPod := hookPod.DeepCopy()
	restartedPod.Status.ContainerStatuses = append(restartedPod.Status.ContainerStatuses, corev1.ContainerStatus{RestartCount: 1})
	_, err = fakeClient.CoreV1().Pods("test").Update(restartedPod)
	if err != nil {
		t.Fatalf("unable to make update to pod: %v", err)
	}

	time.Sleep(1 * time.Second)

	// The hook succeeded
	finalPod := updatedPod.DeepCopy()
	finalPod.Status.Phase = corev1.PodFailed
	finalPod.Status.Message = "fake failure"
	_, err = fakeClient.CoreV1().Pods("test").Update(finalPod)
	if err != nil {
		t.Fatalf("unable to make update to pod: %v", err)
	}

	select {
	case <-hookFinished:
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout while waiting for hook pod to finish")
	}

	hookConsoleOutput := out.String()

	if !strings.Contains(hookConsoleOutput, "sample log") {
		t.Errorf("expected to see 'sample log' in hook hook output, got:\n%s\n", hookConsoleOutput)
	}
	if !strings.Contains(hookConsoleOutput, "Retrying lifecycle hook pod") {
		t.Errorf("expected to see retry message in hook hook output, got:\n%s\n", hookConsoleOutput)
	}

	if c := len(strings.Split(hookConsoleOutput, "sample log")); c != 3 {
		t.Errorf("expected to see logs from two container in hook hook output, got (%d):\n%s\n", c, hookConsoleOutput)
	}
}
func TestRunHookPodExists(t *testing.T) {
	dc := appstest.OkDeploymentConfig(1)
	dc.Namespace = "test"
	rc, err := appsutil.MakeDeploymentV1(dc, legacyscheme.Codecs.LegacyCodec(legacyscheme.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0]))
	if err != nil {
		t.Fatalf("unable to make rc: %v", err)
	}
	deployerPod := makeDeployerPod(rc)
	deployerPod.Status.StartTime = &metav1.Time{time.Now()}

	hookPod := makeHookPod(rc)
	fakeClient := fake.NewSimpleClientset(deployerPod, hookPod, rc.DeepCopy())
	controller := NewLifecycleHookController(fakeClient, nil, &bytes.Buffer{})

	if err := controller.RunHook(makeHook(), rc, DeploymentHookTypePre, 10*time.Second); err != nil {
		t.Fatalf("unexpected hook run error, got: %v", err)
	}
}

func makeHook() *appsapi.LifecycleHook {
	return &appsapi.LifecycleHook{
		FailurePolicy: appsapi.LifecycleHookFailurePolicyAbort,
		ExecNewPod: &appsapi.ExecNewPodHook{
			Command: []string{"/bin/true"},
			Env: []kapi.EnvVar{
				{Name: "foo", Value: "bar"},
			},
			ContainerName: "test",
		},
	}
}

func makeHookPod(deployment *corev1.ReplicationController) *corev1.Pod {
	hookPodName := apihelpers.GetPodName(deployment.Name, "pre")
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hookPodName,
			Namespace: deployment.Namespace,
			Labels: map[string]string{
				appsapi.DeployerPodForDeploymentLabel: deployment.Name,
			},
			Annotations: map[string]string{
				appsapi.DeploymentAnnotation: deployment.Name,
			},
		},
	}
}

func makeDeployerPod(deployment *corev1.ReplicationController) *corev1.Pod {
	deployerPodName := appsutil.DeployerPodNameForDeployment(deployment.Name)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployerPodName,
			Namespace: deployment.Namespace,
			Labels: map[string]string{
				appsapi.DeployerPodForDeploymentLabel: deployment.Name,
			},
			Annotations: map[string]string{
				appsapi.DeploymentAnnotation: deployment.Name,
			},
		},
	}
}
