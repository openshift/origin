package buildlog

import (
	"fmt"
	"net/http"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
)

type podControl struct{}

func (p *podControl) getPod(namespace, podName string) (*kapi.Pod, error) {
	pod := &kapi.Pod{}
	switch podName {
	case "pending":
		pod = mockPod(kapi.PodPending)
	case "running":
		pod = mockPod(kapi.PodRunning)
	case "succeeded":
		pod = mockPod(kapi.PodSucceeded)
	case "failed":
		pod = mockPod(kapi.PodFailed)
	case "unknown":
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
		api.BuildStatusComplete:  fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running/foo-container", kapi.NamespaceDefault),
		api.BuildStatusFailed:    fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running/foo-container", kapi.NamespaceDefault),
		api.BuildStatusRunning:   fmt.Sprintf("https://foo-host:12345/containerLogs/%s/running/foo-container?follow=1", kapi.NamespaceDefault),
		api.BuildStatusNew:       "",
		api.BuildStatusPending:   "",
		api.BuildStatusError:     "",
		api.BuildStatusCancelled: "",
	}

	ctx := kapi.NewDefaultContext()

	for buildStatus, expectedLocation := range expectedLocations {
		location, err := resourceLocationHelper(buildStatus, "running", ctx)
		switch buildStatus {
		case api.BuildStatusNew, api.BuildStatusPending, api.BuildStatusError, api.BuildStatusCancelled:
			if err == nil {
				t.Errorf("Expected error when Build is in %s state, got nothing", buildStatus)
			}
		default:
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}

		if location != expectedLocation {
			t.Errorf("Expected: %s, Got %s", expectedLocation, location)
		}
	}
}

// TestRegistryResourceLocationPodPhases tests if ResourceLocation methods
// returns error, based on pod phase
// Note: For this test, the mocked build is set to "Running" state, so the test
// is evaluating the outcome based only on pod phase.
func TestRegistryResourceLocationPodPhases(t *testing.T) {
	expectedPodPhases := map[string]bool{
		"pending":   true,
		"running":   false,
		"succeeded": false,
		"failed":    false,
		"unknown":   true,
	}

	ctx := kapi.NewDefaultContext()

	for podPhase, expectedError := range expectedPodPhases {
		_, err := resourceLocationHelper(api.BuildStatusRunning, podPhase, ctx)
		switch expectedError {
		case true:
			if err == nil {
				t.Errorf("Expected error when Pod is in %s phase, got nothing", podPhase)
			}
		default:
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}
	}
}

func resourceLocationHelper(buildStatus api.BuildStatus, podPhase string, ctx kapi.Context) (string, error) {
	expectedBuild := mockBuild(buildStatus, podPhase)
	buildRegistry := test.BuildRegistry{Build: expectedBuild}

	storage := REST{&buildRegistry, &podControl{}, &kclient.HTTPKubeletClient{EnableHttps: true, Port: 12345, Client: &http.Client{}}}
	redirector := rest.Redirector(&storage)
	location, _, err := redirector.ResourceLocation(ctx, "foo-build")
	if err != nil {
		return "", err
	}
	return location.String(), err
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
		},
		Status: kapi.PodStatus{
			Host:  "foo-host",
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
