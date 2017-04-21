package buildlog

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericrest "k8s.io/apiserver/pkg/registry/generic/rest"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
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

// TestRegistryResourceLocation tests if proper resource location URL is returner
// for different build states.
// Note: For this test, the mocked pod is set to "Running" phase, so the test
// is evaluating the outcome based only on build state.
func TestRegistryResourceLocation(t *testing.T) {
	expectedLocations := map[api.BuildPhase]string{
		api.BuildPhaseComplete:  fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running-build/foo-container", metav1.NamespaceDefault),
		api.BuildPhaseFailed:    fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running-build/foo-container", metav1.NamespaceDefault),
		api.BuildPhaseRunning:   fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running-build/foo-container", metav1.NamespaceDefault),
		api.BuildPhaseNew:       "",
		api.BuildPhasePending:   "",
		api.BuildPhaseError:     "",
		api.BuildPhaseCancelled: "",
	}

	ctx := apirequest.NewDefaultContext()

	for BuildPhase, expectedLocation := range expectedLocations {
		location, err := resourceLocationHelper(BuildPhase, "running", ctx, 1)
		switch BuildPhase {
		case api.BuildPhaseError, api.BuildPhaseCancelled:
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
		status      []api.BuildPhase
		expectError bool
	}{
		{
			name:        "New -> Running",
			status:      []api.BuildPhase{api.BuildPhaseNew, api.BuildPhaseRunning},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Complete",
			status:      []api.BuildPhase{api.BuildPhaseNew, api.BuildPhasePending, api.BuildPhaseComplete},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Failed",
			status:      []api.BuildPhase{api.BuildPhaseNew, api.BuildPhasePending, api.BuildPhaseFailed},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Cancelled",
			status:      []api.BuildPhase{api.BuildPhaseNew, api.BuildPhasePending, api.BuildPhaseCancelled},
			expectError: true,
		},
		{
			name:        "New -> Pending -> Error",
			status:      []api.BuildPhase{api.BuildPhaseNew, api.BuildPhasePending, api.BuildPhaseError},
			expectError: true,
		},
		{
			name:        "Pending -> Cancelled",
			status:      []api.BuildPhase{api.BuildPhasePending, api.BuildPhaseCancelled},
			expectError: true,
		},
		{
			name:        "Error",
			status:      []api.BuildPhase{api.BuildPhaseError},
			expectError: true,
		},
	}

	for _, tt := range tests {
		build := mockBuild(api.BuildPhasePending, "running", 1)
		ch := make(chan watch.Event)
		watcher := &buildWatcher{
			Build: build,
			Watcher: &fakeWatch{
				Channel: ch,
			},
		}
		storage := REST{
			Getter:         watcher,
			Watcher:        watcher,
			PodGetter:      &testPodGetter{},
			ConnectionInfo: &fakeConnectionInfoGetter{},
			Timeout:        defaultTimeout,
		}
		go func() {
			for _, status := range tt.status {
				ch <- watch.Event{
					Type:   watch.Modified,
					Object: mockBuild(status, "running", 1),
				}
			}
		}()
		_, err := storage.Get(ctx, build.Name, &api.BuildLogOptions{})
		if tt.expectError && err == nil {
			t.Errorf("%s: Expected an error but got nil from waitFromBuild", tt.name)
		}
		if !tt.expectError && err != nil {
			t.Errorf("%s: Unexpected error from watchBuild: %v", tt.name, err)
		}
	}
}

func TestWaitForBuildTimeout(t *testing.T) {
	ctx := apirequest.NewDefaultContext()
	build := mockBuild(api.BuildPhasePending, "running", 1)
	ch := make(chan watch.Event)
	watcher := &buildWatcher{
		Build: build,
		Watcher: &fakeWatch{
			Channel: ch,
		},
	}
	storage := REST{
		Getter:         watcher,
		Watcher:        watcher,
		PodGetter:      &testPodGetter{},
		ConnectionInfo: &fakeConnectionInfoGetter{},
		Timeout:        100 * time.Millisecond,
	}
	_, err := storage.Get(ctx, build.Name, &api.BuildLogOptions{})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("Unexpected error result from waitForBuild: %v\n", err)
	}
}

type buildWatcher struct {
	Build   *api.Build
	Watcher watch.Interface
	Err     error
}

func (r *buildWatcher) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.Build, nil
}

func (r *buildWatcher) Watch(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return r.Watcher, r.Err
}

type fakeWatch struct {
	Channel chan watch.Event
}

func (w *fakeWatch) Stop() {
	close(w.Channel)
}

func (w *fakeWatch) ResultChan() <-chan watch.Event {
	return w.Channel
}

func resourceLocationHelper(BuildPhase api.BuildPhase, podPhase string, ctx apirequest.Context, version int) (string, error) {
	expectedBuild := mockBuild(BuildPhase, podPhase, version)
	internal := &test.BuildStorage{Build: expectedBuild}

	storage := &REST{
		Getter:         internal,
		PodGetter:      &testPodGetter{},
		ConnectionInfo: &fakeConnectionInfoGetter{},
		Timeout:        defaultTimeout,
	}
	getter := rest.GetterWithOptions(storage)
	obj, err := getter.Get(ctx, "foo-build", &api.BuildLogOptions{NoWait: true})
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

func mockBuild(status api.BuildPhase, podName string, version int) *api.Build {
	return &api.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Annotations: map[string]string{
				api.BuildNumberAnnotation: strconv.Itoa(version),
			},
			Labels: map[string]string{
				api.BuildConfigLabel: "bc",
			},
		},
		Status: api.BuildStatus{
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
	first := mockBuild(api.BuildPhaseComplete, "bc-1", 1)
	second := mockBuild(api.BuildPhaseComplete, "bc-2", 2)
	third := mockBuild(api.BuildPhaseComplete, "bc-3", 3)
	internal := &test.BuildStorage{Builds: &api.BuildList{Items: []api.Build{*first, *second, *third}}}

	storage := &REST{
		Getter:         internal,
		PodGetter:      &anotherTestPodGetter{},
		ConnectionInfo: &fakeConnectionInfoGetter{},
		Timeout:        defaultTimeout,
	}
	getter := rest.GetterWithOptions(storage)
	// Will expect the previous from bc-3 aka bc-2
	obj, err := getter.Get(ctx, "bc-3", &api.BuildLogOptions{NoWait: true, Previous: true})
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
