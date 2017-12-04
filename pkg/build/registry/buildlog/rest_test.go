package buildlog

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericrest "k8s.io/apiserver/pkg/registry/generic/rest"
	"k8s.io/apiserver/pkg/registry/rest"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildfakeclient "github.com/openshift/origin/pkg/build/generated/internalclientset/fake"
)

type testPodGetter struct{}

func (p *testPodGetter) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	pod := &kapi.Pod{}
	switch name {
	case "pending-build":
		pod = mockPod(kapi.PodPending, name)
	case "running-build":
		pod = mockPod(kapi.PodRunning, name)
	case "succeeded-build":
		pod = mockPod(kapi.PodSucceeded, name)
	case "failed-build":
		pod = mockPod(kapi.PodFailed, name)
	case "unknown-build":
		pod = mockPod(kapi.PodUnknown, name)
	}
	return pod, nil
}

type fakeConnectionInfoGetter struct{}

func (*fakeConnectionInfoGetter) GetConnectionInfo(nodeName types.NodeName) (*kubeletclient.ConnectionInfo, error) {
	rt, err := kubeletclient.MakeTransport(&kubeletclient.KubeletClientConfig{})
	if err != nil {
		return nil, err
	}
	return &kubeletclient.ConnectionInfo{
		Scheme:    "https",
		Hostname:  "foo-host",
		Port:      "12345",
		Transport: rt,
	}, nil
}

// TestRegistryResourceLocation tests if proper resource location URL is returned
// for different build states.
// Note: For this test, the mocked pod is set to "Running" phase, so the test
// is evaluating the outcome based only on build state.
func TestRegistryResourceLocation(t *testing.T) {
	expectedLocations := map[buildapi.BuildPhase]string{
		buildapi.BuildPhaseComplete:  fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running-build/foo-container", metav1.NamespaceDefault),
		buildapi.BuildPhaseFailed:    fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running-build/foo-container", metav1.NamespaceDefault),
		buildapi.BuildPhaseRunning:   fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running-build/foo-container", metav1.NamespaceDefault),
		buildapi.BuildPhaseNew:       "",
		buildapi.BuildPhasePending:   "",
		buildapi.BuildPhaseError:     "",
		buildapi.BuildPhaseCancelled: "",
	}

	ctx := apirequest.NewDefaultContext()

	for BuildPhase, expectedLocation := range expectedLocations {
		location, err := resourceLocationHelper(BuildPhase, "running", ctx, 1)
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

		if location != expectedLocation {
			t.Errorf("Status: %s Expected Location: %s, Got %s", BuildPhase, expectedLocation, location)
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
			BuildClient:    buildClient.Build(),
			PodGetter:      &testPodGetter{},
			ConnectionInfo: &fakeConnectionInfoGetter{},
			Timeout:        defaultTimeout,
		}
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
		BuildClient:    buildClient.Build(),
		PodGetter:      &testPodGetter{},
		ConnectionInfo: &fakeConnectionInfoGetter{},
		Timeout:        100 * time.Millisecond,
	}
	_, err := storage.Get(ctx, build.Name, &buildapi.BuildLogOptions{})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("Unexpected error result from waitForBuild: %v\n", err)
	}
}

func resourceLocationHelper(BuildPhase buildapi.BuildPhase, podPhase string, ctx apirequest.Context, version int) (string, error) {
	expectedBuild := mockBuild(BuildPhase, podPhase, version)
	buildClient := buildfakeclient.NewSimpleClientset(expectedBuild)

	storage := &REST{
		BuildClient:    buildClient.Build(),
		PodGetter:      &testPodGetter{},
		ConnectionInfo: &fakeConnectionInfoGetter{},
		Timeout:        defaultTimeout,
	}
	getter := rest.GetterWithOptions(storage)
	obj, err := getter.Get(ctx, expectedBuild.Name, &buildapi.BuildLogOptions{NoWait: true})
	if err != nil {
		return "", err
	}
	streamer, ok := obj.(*genericrest.LocationStreamer)
	if !ok {
		return "", fmt.Errorf("Result of get not LocationStreamer")
	}
	if streamer.Location != nil {
		return streamer.Location.String(), nil
	}
	return "", nil

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

type anotherTestPodGetter struct{}

func (p *anotherTestPodGetter) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	pod := &kapi.Pod{}
	switch name {
	case "bc-1-build":
		pod = mockPod(kapi.PodSucceeded, name)
	case "bc-2-build":
		pod = mockPod(kapi.PodSucceeded, name)
	case "bc-3-build":
		pod = mockPod(kapi.PodSucceeded, name)
	}
	return pod, nil
}

func TestPreviousBuildLogs(t *testing.T) {
	ctx := apirequest.NewDefaultContext()
	first := mockBuild(buildapi.BuildPhaseComplete, "bc-1", 1)
	second := mockBuild(buildapi.BuildPhaseComplete, "bc-2", 2)
	third := mockBuild(buildapi.BuildPhaseComplete, "bc-3", 3)
	buildClient := buildfakeclient.NewSimpleClientset(first, second, third)

	storage := &REST{
		BuildClient:    buildClient.Build(),
		PodGetter:      &anotherTestPodGetter{},
		ConnectionInfo: &fakeConnectionInfoGetter{},
		Timeout:        defaultTimeout,
	}
	getter := rest.GetterWithOptions(storage)
	// Will expect the previous from bc-3 aka bc-2
	obj, err := getter.Get(ctx, "bc-3", &buildapi.BuildLogOptions{NoWait: true, Previous: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	streamer, ok := obj.(*genericrest.LocationStreamer)
	if !ok {
		t.Fatalf("unexpected object: %#v", obj)
	}

	expected := &url.URL{
		Scheme: "https",
		Host:   "foo-host:12345",
		Path:   "/containerLogs/default/bc-2-build/foo-container",
	}

	if exp, got := expected.String(), streamer.Location.String(); exp != got {
		t.Fatalf("expected location:\n\t%s\ngot location:\n\t%s\n", exp, got)
	}
}
