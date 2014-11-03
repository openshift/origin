package buildlog

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
)

type podClient struct {
	kclient.Fake
}

func (p *podClient) GetPod(ctx kapi.Context, id string) (*kapi.Pod, error) {
	pod := &kapi.Pod{
		TypeMeta: kapi.TypeMeta{
			ID:        "foo",
			Namespace: kapi.NamespaceDefault,
		},
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

func TestRegistryResourceLocation(t *testing.T) {
	expectedLocations := map[api.BuildStatus]string{
		api.BuildStatusComplete: fmt.Sprintf("//foo-host:%d/containerLogs/%s/foo-pod/foo-container",
			kubernetes.NodePort, kapi.NamespaceDefault),
		api.BuildStatusRunning: fmt.Sprintf("//foo-host:%d/containerLogs/%s/foo-pod/foo-container?follow=1",
			kubernetes.NodePort, kapi.NamespaceDefault),
	}

	ctx := kapi.NewDefaultContext()

	for buildStatus, expectedLocation := range expectedLocations {
		expectedBuild := mockBuild(buildStatus)
		buildRegistry := test.BuildRegistry{Build: expectedBuild}
		storage := REST{&buildRegistry, &podClient{}}
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
