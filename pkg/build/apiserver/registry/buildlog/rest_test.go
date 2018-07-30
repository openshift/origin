package buildlog

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildfakeclient "github.com/openshift/origin/pkg/build/generated/internalclientset/fake"
)

func newPodClient() *fake.Clientset {
	return fake.NewSimpleClientset(
		mockPod(kapi.PodPending, "pending-build"),
		mockPod(kapi.PodRunning, "running-build"),
		mockPod(kapi.PodSucceeded, "succeeded-build"),
		mockPod(kapi.PodFailed, "failed-build"),
		mockPod(kapi.PodUnknown, "unknown-build"),
	)
}

func anotherNewPodClient() *fake.Clientset {
	return fake.NewSimpleClientset(
		mockPod(kapi.PodSucceeded, "bc-1-build"),
		mockPod(kapi.PodSucceeded, "bc-2-build"),
		mockPod(kapi.PodSucceeded, "bc-3-build"),
	)
}

// TestRegistryResourceLocation tests if proper resource location URL is returned
// for different build states.
// Note: For this test, the mocked pod is set to "Running" phase, so the test
// is evaluating the outcome based only on build state.
func TestRegistryResourceLocation(t *testing.T) {
	expectedLocations := map[buildapi.BuildPhase]struct {
		namespace string
		name      string
		container string
	}{
		buildapi.BuildPhaseComplete:  {namespace: "default", name: "running-build", container: ""},
		buildapi.BuildPhaseFailed:    {namespace: "default", name: "running-build", container: ""},
		buildapi.BuildPhaseRunning:   {namespace: "default", name: "running-build", container: ""},
		buildapi.BuildPhaseNew:       {},
		buildapi.BuildPhasePending:   {},
		buildapi.BuildPhaseError:     {},
		buildapi.BuildPhaseCancelled: {},
	}

	ctx := apirequest.NewDefaultContext()

	for BuildPhase, expectedLocation := range expectedLocations {
		actualNamespace, actualPodName, actualContainer, err := resourceLocationHelper(BuildPhase, "running", ctx, 1)
		switch BuildPhase {
		case buildapi.BuildPhaseError, buildapi.BuildPhaseCancelled:
			if err == nil {
				t.Errorf("Expected error when Build is in %s state, got nothing", BuildPhase)
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
		status      []buildapi.BuildPhase
		expectError bool
	}{
		{
			name:        "New -> Running",
			status:      []buildapi.BuildPhase{buildapi.BuildPhaseNew, buildapi.BuildPhaseRunning},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Complete",
			status:      []buildapi.BuildPhase{buildapi.BuildPhaseNew, buildapi.BuildPhasePending, buildapi.BuildPhaseComplete},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Failed",
			status:      []buildapi.BuildPhase{buildapi.BuildPhaseNew, buildapi.BuildPhasePending, buildapi.BuildPhaseFailed},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Cancelled",
			status:      []buildapi.BuildPhase{buildapi.BuildPhaseNew, buildapi.BuildPhasePending, buildapi.BuildPhaseCancelled},
			expectError: true,
		},
		{
			name:        "New -> Pending -> Error",
			status:      []buildapi.BuildPhase{buildapi.BuildPhaseNew, buildapi.BuildPhasePending, buildapi.BuildPhaseError},
			expectError: true,
		},
		{
			name:        "Pending -> Cancelled",
			status:      []buildapi.BuildPhase{buildapi.BuildPhasePending, buildapi.BuildPhaseCancelled},
			expectError: true,
		},
		{
			name:        "Error",
			status:      []buildapi.BuildPhase{buildapi.BuildPhaseError},
			expectError: true,
		},
	}

	for _, tt := range tests {
		build := mockBuild(buildapi.BuildPhasePending, "running", 1)
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
	build := mockBuild(buildapi.BuildPhasePending, "running", 1)
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

func resourceLocationHelper(BuildPhase buildapi.BuildPhase, podPhase string, ctx context.Context, version int) (string, string, string, error) {
	expectedBuild := mockBuild(BuildPhase, podPhase, version)
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

func mockPod(podPhase kapi.PodPhase, podName string) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name: "foo-container",
				},
			},
			NodeName: "foo-host",
		},
		Status: kapi.PodStatus{
			Phase: podPhase,
		},
	}
}

func mockBuild(status buildapi.BuildPhase, podName string, version int) *buildapi.Build {
	return &buildapi.Build{
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
		Status: buildapi.BuildStatus{
			Phase: status,
		},
	}
}

func TestPreviousBuildLogs(t *testing.T) {
	ctx := apirequest.NewDefaultContext()
	first := mockBuild(buildapi.BuildPhaseComplete, "bc-1", 1)
	second := mockBuild(buildapi.BuildPhaseComplete, "bc-2", 2)
	third := mockBuild(buildapi.BuildPhaseComplete, "bc-3", 3)
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
