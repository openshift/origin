package buildlog

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
)

type podClient struct {
}

func (p *podClient) ListPods(ctx kapi.Context, selector labels.Selector) (*kapi.PodList, error) {
	return nil, nil
}

func (p *podClient) GetPod(ctx kapi.Context, id string) (*kapi.Pod, error) {
	pod := &kapi.Pod{
		TypeMeta: kapi.TypeMeta{ID: "foo"},
		DesiredState: kapi.PodState{
			Manifest: kapi.ContainerManifest{
				Version: "v1beta1",
				Containers: []kapi.Container{
					{
						Name: "foo-container",
					},
				},
			},
		},
		CurrentState: kapi.PodState{
			Host: "foo-host",
		},
	}
	return pod, nil
}

func (p *podClient) DeletePod(ctx kapi.Context, id string) error {
	return nil
}

func (p *podClient) CreatePod(ctx kapi.Context, pod *kapi.Pod) (*kapi.Pod, error) {
	return nil, nil
}

func (p *podClient) UpdatePod(ctx kapi.Context, pod *kapi.Pod) (*kapi.Pod, error) {
	return nil, nil
}

func TestRegistryResourceLocation(t *testing.T) {
	expectedLocations := map[api.BuildStatus]string{
		api.BuildStatusComplete: "/proxy/minion/foo-host/containerLogs/foo-pod/foo-container",
		api.BuildStatusRunning:  "/proxy/minion/foo-host/containerLogs/foo-pod/foo-container?follow=1",
	}

	ctx := kapi.NewDefaultContext()
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
		TypeMeta: kapi.TypeMeta{
			ID: "foo-build",
		},
		Status: buildStatus,
		PodID:  "foo-pod",
	}
}
