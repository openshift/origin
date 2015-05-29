package buildlog

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	genericrest "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
)

type testPodGetter struct{}

func (p *testPodGetter) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	pod := &kapi.Pod{}
	switch name {
	case "pending-build":
		pod = mockPod(kapi.PodPending)
	case "running-build":
		pod = mockPod(kapi.PodRunning)
	case "succeeded-build":
		pod = mockPod(kapi.PodSucceeded)
	case "failed-build":
		pod = mockPod(kapi.PodFailed)
	case "unknown-build":
		pod = mockPod(kapi.PodUnknown)
	}
	return pod, nil
}

// TestRegistryResourceLocation tests if proper resource location URL is returner
// for different build states.
// Note: For this test, the mocked pod is set to "Running" phase, so the test
// is evaluating the outcome based only on build state.
func TestRegistryResourceLocation(t *testing.T) {
	expectedLocations := map[api.BuildStatus]string{
		api.BuildStatusComplete:  fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running-build/foo-container", kapi.NamespaceDefault),
		api.BuildStatusFailed:    fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running-build/foo-container", kapi.NamespaceDefault),
		api.BuildStatusRunning:   fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running-build/foo-container", kapi.NamespaceDefault),
		api.BuildStatusNew:       "",
		api.BuildStatusPending:   "",
		api.BuildStatusError:     "",
		api.BuildStatusCancelled: "",
	}

	ctx := kapi.NewDefaultContext()

	for buildStatus, expectedLocation := range expectedLocations {
		location, err := resourceLocationHelper(buildStatus, "running", ctx)
		switch buildStatus {
		case api.BuildStatusError, api.BuildStatusCancelled:
			if err == nil {
				t.Errorf("Expected error when Build is in %s state, got nothing", buildStatus)
			}
		default:
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}

		if location != expectedLocation {
			t.Errorf("Status: %s Expected Location: %s, Got %s", buildStatus, expectedLocation, location)
		}
	}
}

func TestWaitForBuild(t *testing.T) {
	ctx := kapi.NewDefaultContext()
	tests := []struct {
		name        string
		status      []api.BuildStatus
		expectError bool
	}{
		{
			name:        "New -> Running",
			status:      []api.BuildStatus{api.BuildStatusNew, api.BuildStatusRunning},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Complete",
			status:      []api.BuildStatus{api.BuildStatusNew, api.BuildStatusPending, api.BuildStatusComplete},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Failed",
			status:      []api.BuildStatus{api.BuildStatusNew, api.BuildStatusPending, api.BuildStatusFailed},
			expectError: false,
		},
		{
			name:        "New -> Pending -> Error",
			status:      []api.BuildStatus{api.BuildStatusNew, api.BuildStatusPending, api.BuildStatusError},
			expectError: true,
		},
		{
			name:        "Pending -> Cancelled",
			status:      []api.BuildStatus{api.BuildStatusPending, api.BuildStatusCancelled},
			expectError: true,
		},
		{
			name:        "Error",
			status:      []api.BuildStatus{api.BuildStatusError},
			expectError: true,
		},
	}

	for _, tt := range tests {
		build := mockBuild(api.BuildStatusPending, "running")
		ch := make(chan watch.Event)
		registry := &buildRegistryWithWatch{
			BuildRegistry: &test.BuildRegistry{},
			Watch: &fakeWatch{
				Channel: ch,
			},
		}
		storage := REST{
			BuildRegistry: registry,
			Timeout:       defaultTimeout,
		}
		go func() {
			for _, status := range tt.status {
				ch <- watch.Event{
					Type:   watch.Modified,
					Object: mockBuild(status, "running"),
				}
			}
		}()
		err := storage.waitForBuild(ctx, build)
		if tt.expectError && err == nil {
			t.Errorf("%s: Expected an error but got nil from waitFromBuild", tt.name)
		}
		if !tt.expectError && err != nil {
			t.Errorf("%s: Unexpected error from watchBuild: %v", tt.name, err)
		}
	}
}

func TestWaitForBuildTimeout(t *testing.T) {
	ctx := kapi.NewDefaultContext()
	build := mockBuild(api.BuildStatusPending, "running")
	ch := make(chan watch.Event)
	registry := &buildRegistryWithWatch{
		BuildRegistry: &test.BuildRegistry{},
		Watch: &fakeWatch{
			Channel: ch,
		},
	}
	storage := REST{
		BuildRegistry: registry,
		Timeout:       100 * time.Millisecond,
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		ch <- watch.Event{
			Type:   watch.Modified,
			Object: mockBuild(api.BuildStatusRunning, "running"),
		}
	}()
	err := storage.waitForBuild(ctx, build)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("Unexpected error result from waitForBuild: %v\n", err)
	}
}

type buildRegistryWithWatch struct {
	*test.BuildRegistry
	Watch watch.Interface
}

func (r *buildRegistryWithWatch) WatchBuilds(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return r.Watch, r.Err
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

func resourceLocationHelper(buildStatus api.BuildStatus, podPhase string, ctx kapi.Context) (string, error) {
	expectedBuild := mockBuild(buildStatus, podPhase)
	buildRegistry := test.BuildRegistry{Build: expectedBuild}

	storage := REST{
		BuildRegistry:  &buildRegistry,
		PodGetter:      &testPodGetter{},
		ConnectionInfo: &kclient.HTTPKubeletClient{EnableHttps: true, Port: 12345, Client: &http.Client{}},
		Timeout:        defaultTimeout,
	}
	getter := rest.GetterWithOptions(&storage)
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

func mockPod(podPhase kapi.PodPhase) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "foo-pod",
			Namespace: kapi.NamespaceDefault,
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name: "foo-container",
				},
			},
			Host: "foo-host",
		},
		Status: kapi.PodStatus{
			Phase: podPhase,
		},
	}
}

func mockBuild(buildStatus api.BuildStatus, podName string) *api.Build {
	return &api.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: podName,
		},
		Status: buildStatus,
	}
}
