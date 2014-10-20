package buildlog

import (
	"testing"

	kubeapi	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
)

type podClient struct {
}

func (p *podClient) ListPods(ctx kubeapi.Context, selector labels.Selector) (*kubeapi.PodList, error) {
	return nil, nil
}

func (p *podClient) GetPod(ctx kubeapi.Context, id string) (*kubeapi.Pod, error) {
	pod := &kubeapi.Pod{
		JSONBase:     kubeapi.JSONBase{ID: "foo"},
		DesiredState: kubeapi.PodState{
			Manifest: kubeapi.ContainerManifest{
				Version: "v1beta1",
				Containers: []kubeapi.Container{
					{
						Name: "foo-container",
					},
				},
			},
		},
		CurrentState: kubeapi.PodState{
			Host: "foo-host",
		},
	}
	return pod, nil
}

func (p *podClient) DeletePod(ctx kubeapi.Context, id string) error {
	return nil
}

func (p *podClient) CreatePod(ctx kubeapi.Context, pod *kubeapi.Pod) (*kubeapi.Pod, error) {
	return nil, nil
}

func (p *podClient) UpdatePod(ctx kubeapi.Context, pod *kubeapi.Pod) (*kubeapi.Pod, error) {
	return nil, nil
}

func TestRegistryResourceLocation(t *testing.T) {
	expectedLocations := map[api.BuildStatus]string{
		api.BuildComplete: "/proxy/minion/foo-host/containerLogs/foo-pod/foo-container",
		api.BuildRunning: "/proxy/minion/foo-host/containerLogs/foo-pod/foo-container?follow=1",
	}

	ctx := kubeapi.NewDefaultContext()
	proxyPrefix := "/proxy/minion"

	for buildStatus, expectedLocation := range expectedLocations {
		expectedBuild := mockBuild(buildStatus)
		buildRegistry := test.BuildRegistry{Build: expectedBuild}
		storage := REST{&buildRegistry, &podClient{}, proxyPrefix}
		redirector := apiserver.Redirector(&storage)
		location, err := redirector.ResourceLocation(ctx, "foo")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if location != expectedLocation {
			t.Errorf("Expected: %s, Got %s", expectedLocation, location)
		}
	}
}

func mockBuild(buildStatus api.BuildStatus) *api.Build {
	return &api.Build{
		JSONBase: kubeapi.JSONBase{
			ID: "foo-build",
		},
		Status: buildStatus,
		PodID:  "foo-pod",
	}
}
