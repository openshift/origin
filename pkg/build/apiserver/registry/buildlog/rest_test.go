package buildlog

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildv1 "github.com/openshift/api/build/v1"
	buildfakeclient "github.com/openshift/client-go/build/clientset/versioned/fake"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

func newPodClient() *fake.Clientset {
	return fake.NewSimpleClientset(
		mockPod(corev1.PodPending, "pending-build"),
		mockPod(corev1.PodRunning, "running-build"),
		mockPod(corev1.PodSucceeded, "succeeded-build"),
		mockPod(corev1.PodFailed, "failed-build"),
		mockPod(corev1.PodUnknown, "unknown-build"),
	)
}

func anotherNewPodClient() *fake.Clientset {
	return fake.NewSimpleClientset(
		mockPod(corev1.PodSucceeded, "bc-1-build"),
		mockPod(corev1.PodSucceeded, "bc-2-build"),
		mockPod(corev1.PodSucceeded, "bc-3-build"),
	)
}

// TestRegistryResourceLocation tests if proper resource location URL is returned
// for different build states.
// Note: For this test, the mocked pod is set to "Running" phase, so the test
// is evaluating the outcome based only on build state.
func TestRegistryResourceLocation(t *testing.T) {
	expectedLocations := map[buildv1.BuildPhase]struct {
		namespace string
		name      string
		container string
	}{
		buildv1.BuildPhaseComplete:  {namespace: "default", name: "running-build", container: ""},
		buildv1.BuildPhaseFailed:    {namespace: "default", name: "running-build", container: ""},
		buildv1.BuildPhaseRunning:   {namespace: "default", name: "running-build", container: ""},
		buildv1.BuildPhaseNew:       {},
		buildv1.BuildPhasePending:   {},
		buildv1.BuildPhaseError:     {},
		buildv1.BuildPhaseCancelled: {},
	}

	ctx := apirequest.NewDefaultContext()

	for buildPhase, expectedLocation := range expectedLocations {
		actualNamespace, actualPodName, actualContainer, err := resourceLocationHelper(buildPhase, "running", ctx, 1)
		switch buildPhase {
		case buildv1.BuildPhaseError, buildv1.BuildPhaseCancelled:
			if err == nil {
				t.Errorf("Expected error when Build is in %s state, got nothing", buildPhase)
			}
		default:
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}

		if e, a := expectedLocation.namespace, actualNamespace; e != a {
			t.Errorf("expected %v, actual %v", e, a)
		}
		if e, a := expectedLocation.name, actualPodName; e != a {
			t.Errorf("expected %v, actual %v", e, a)
		}
		if e, a := expectedLocation.container, actualContainer; e != a {
			t.Errorf("expected %v, actual %v", e, a)
		}
	}
}

func TestWaitForBuild(t *testing.T) {
	ctx := apirequest.NewDefaultContext()
	tests := []struct {
		name        string
		status      []buildv1.BuildPhase
		expectError bool
	}{
		{
			name:        "New -> Running",
			status:      []buildv1.BuildPhase{buildv1.BuildPhaseNew, buildv1.BuildPhaseRunning},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Complete",
			status:      []buildv1.BuildPhase{buildv1.BuildPhaseNew, buildv1.BuildPhasePending, buildv1.BuildPhaseComplete},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Failed",
			status:      []buildv1.BuildPhase{buildv1.BuildPhaseNew, buildv1.BuildPhasePending, buildv1.BuildPhaseFailed},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Cancelled",
			status:      []buildv1.BuildPhase{buildv1.BuildPhaseNew, buildv1.BuildPhasePending, buildv1.BuildPhaseCancelled},
			expectError: true,
		},
		{
			name:        "New -> Pending -> Error",
			status:      []buildv1.BuildPhase{buildv1.BuildPhaseNew, buildv1.BuildPhasePending, buildv1.BuildPhaseError},
			expectError: true,
		},
		{
			name:        "Pending -> Cancelled",
			status:      []buildv1.BuildPhase{buildv1.BuildPhasePending, buildv1.BuildPhaseCancelled},
			expectError: true,
		},
		{
			name:        "Error",
			status:      []buildv1.BuildPhase{buildv1.BuildPhaseError},
			expectError: true,
		},
	}

	for _, tt := range tests {
		build := mockBuild(buildv1.BuildPhasePending, "running", 1)
		buildClient := buildfakeclient.NewSimpleClientset(build)
		fakeWatcher := watch.NewFake()
		buildClient.PrependWatchReactor("builds", func(action clientgotesting.Action) (handled bool, ret watch.Interface, err error) {
			return true, fakeWatcher, nil
		})
		storage := REST{
			BuildClient: buildClient.Build(),
			PodClient:   newPodClient().Core(),
			Timeout:     defaultTimeout,
		}
		getSimplePodLogsFn := func(podNamespace, podName string, logOpts *kapi.PodLogOptions) (runtime.Object, error) {
			return nil, nil
		}
		storage.getSimpleLogsFn = getSimplePodLogsFn

		go func() {
			for _, status := range tt.status {
				fakeWatcher.Modify(mockBuild(status, "running", 1))
			}
		}()

		_, err := storage.Get(ctx, build.Name, &buildapi.BuildLogOptions{})
		if tt.expectError && err == nil {
			t.Errorf("%s: Expected an error but got nil from waitFromBuild", tt.name)
		}
		if !tt.expectError && err != nil {
			t.Errorf("%s: Unexpected error from watchBuild: %v", tt.name, err)
		}
	}
}

func TestWaitForBuildTimeout(t *testing.T) {
	build := mockBuild(buildv1.BuildPhasePending, "running", 1)
	buildClient := buildfakeclient.NewSimpleClientset(build)
	ctx := apirequest.NewDefaultContext()
	storage := REST{
		BuildClient: buildClient.Build(),
		PodClient:   newPodClient().Core(),
		Timeout:     100 * time.Millisecond,
	}

	_, err := storage.Get(ctx, build.Name, &buildapi.BuildLogOptions{})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("Unexpected error result from waitForBuild: %v\n", err)
	}
}

func resourceLocationHelper(buildPhase buildv1.BuildPhase, podPhase string, ctx context.Context, version int) (string, string, string, error) {
	expectedBuild := mockBuild(buildPhase, podPhase, version)
	buildClient := buildfakeclient.NewSimpleClientset(expectedBuild)

	storage := &REST{
		BuildClient: buildClient.Build(),
		PodClient:   newPodClient().Core(),
		Timeout:     defaultTimeout,
	}
	actualPodNamespace := ""
	actualPodName := ""
	actualContainer := ""
	getSimplePodLogsFn := func(podNamespace, podName string, logOpts *kapi.PodLogOptions) (runtime.Object, error) {
		actualPodNamespace = podNamespace
		actualPodName = podName
		actualContainer = logOpts.Container
		return nil, nil
	}
	storage.getSimpleLogsFn = getSimplePodLogsFn

	getter := rest.GetterWithOptions(storage)
	_, err := getter.Get(ctx, expectedBuild.Name, &buildapi.BuildLogOptions{NoWait: true})
	if err != nil {
		return "", "", "", err
	}
	return actualPodNamespace, actualPodName, actualContainer, nil

}

func mockPod(podPhase corev1.PodPhase, podName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "foo-container",
				},
			},
			NodeName: "foo-host",
		},
		Status: corev1.PodStatus{
			Phase: podPhase,
		},
	}
}

func mockBuild(status buildv1.BuildPhase, podName string, version int) *buildv1.Build {
	return &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      podName,
			Annotations: map[string]string{
				buildapi.BuildNumberAnnotation: strconv.Itoa(version),
			},
			Labels: map[string]string{
				buildapi.BuildConfigLabel: "bc",
			},
		},
		Status: buildv1.BuildStatus{
			Phase: status,
		},
	}
}

func TestPreviousBuildLogs(t *testing.T) {
	ctx := apirequest.NewDefaultContext()
	first := mockBuild(buildv1.BuildPhaseComplete, "bc-1", 1)
	second := mockBuild(buildv1.BuildPhaseComplete, "bc-2", 2)
	third := mockBuild(buildv1.BuildPhaseComplete, "bc-3", 3)
	buildClient := buildfakeclient.NewSimpleClientset(first, second, third)

	storage := &REST{
		BuildClient: buildClient.Build(),
		PodClient:   anotherNewPodClient().Core(),
		Timeout:     defaultTimeout,
	}
	actualPodNamespace := ""
	actualPodName := ""
	actualContainer := ""
	getSimplePodLogsFn := func(podNamespace, podName string, logOpts *kapi.PodLogOptions) (runtime.Object, error) {
		actualPodNamespace = podNamespace
		actualPodName = podName
		actualContainer = logOpts.Container
		return nil, nil
	}
	storage.getSimpleLogsFn = getSimplePodLogsFn

	getter := rest.GetterWithOptions(storage)
	// Will expect the previous from bc-3 aka bc-2
	_, err := getter.Get(ctx, "bc-3", &buildapi.BuildLogOptions{NoWait: true, Previous: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e, a := "default", actualPodNamespace; e != a {
		t.Errorf("expected %v, actual %v", e, a)
	}
	if e, a := "bc-2-build", actualPodName; e != a {
		t.Errorf("expected %v, actual %v", e, a)
	}
	if e, a := "", actualContainer; e != a {
		t.Errorf("expected %v, actual %v", e, a)
	}
}
