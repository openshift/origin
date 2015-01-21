package buildlog

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
)

type podControl struct{}

func (p *podControl) getPod(namespace, podName string) (*kapi.Pod, error) {
	pod := &kapi.Pod{}
	switch podName {
	case "pendingPod":
		pod = mockPod(kapi.PodPending)
	case "runningPod":
		pod = mockPod(kapi.PodRunning)
	case "succeededPod":
		pod = mockPod(kapi.PodSucceeded)
	case "failedPod":
		pod = mockPod(kapi.PodFailed)
	case "unknownPod":
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
		api.BuildStatusComplete: fmt.Sprintf("%s://foo-host:%d/containerLogs/%s/runningPod/foo-container",
			kubernetes.NodeScheme, kubernetes.NodePort, kapi.NamespaceDefault),
		api.BuildStatusFailed: fmt.Sprintf("%s://foo-host:%d/containerLogs/%s/runningPod/foo-container",
			kubernetes.NodeScheme, kubernetes.NodePort, kapi.NamespaceDefault),
		api.BuildStatusRunning: fmt.Sprintf("%s://foo-host:%d/containerLogs/%s/runningPod/foo-container?follow=1",
			kubernetes.NodeScheme, kubernetes.NodePort, kapi.NamespaceDefault),
		api.BuildStatusNew:       "",
		api.BuildStatusPending:   "",
		api.BuildStatusError:     "",
		api.BuildStatusCancelled: "",
	}

	ctx := kapi.NewDefaultContext()

	for buildStatus, expectedLocation := range expectedLocations {
		location, err := resourceLocationHelper(buildStatus, "runningPod", ctx)
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
		"pendingPod":   true,
		"runningPod":   false,
		"succeededPod": false,
		"failedPod":    false,
		"unknownPod":   true,
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
	storage := REST{&buildRegistry, &podControl{}}
	redirector := apiserver.Redirector(&storage)
	location, err := redirector.ResourceLocation(ctx, "foo-build")
	return location, err
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
			Name: "foo-build",
		},
		Status:  buildStatus,
		PodName: podName,
	}
}
