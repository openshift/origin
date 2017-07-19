package plugin

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
	dockertools "k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
)

func TestPodKillOnFailedUpdate(t *testing.T) {
	fakeDocker := dockertools.NewFakeDockerClient()
	id := "509383712c59ee328a78ae99d0f9411aa99f0bdf1ecf304aa83afb58f16f0768"
	name := "/k8s_nginx1_nginx1_default_379e14d9-562e-11e7-b251-0242ac110003_0"
	infraId := "0e7ff50ca5399654fe3b93a21dae1d264560bc018d5f0b13e79601c1a7948d6e"
	randomId := "71167588cc97636d2f269081579fb9668b4e42acdfdd1e1cea220f6de86a8b50"
	fakeDocker.SetFakeRunningContainers([]*dockertools.FakeContainer{
		{
			ID:   id,
			Name: name,
		},
		{
			// Infra container for the above container
			ID:   infraId,
			Name: "/k8s_POD_nginx1_default_379e14d9-562e-11e7-b251-0242ac110003_1",
		},
		{
			// Random container unrelated to first two
			ID:   randomId,
			Name: "/k8s_POD_blah_default_fef9db05-f5c2-4361-9244-2eb505bc61e7_1",
		},
	})

	pods := []kapi.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testpod1",
				Namespace: "namespace1",
			},
			Status: kapi.PodStatus{
				ContainerStatuses: []kapi.ContainerStatus{
					{
						Name:        "container1",
						ContainerID: fmt.Sprintf("docker://%s", id),
						State: kapi.ContainerState{
							Running: &kapi.ContainerStateRunning{},
						},
					},
				},
			},
		},
	}

	err := killUpdateFailedPods(fakeDocker, pods)
	if err != nil {
		t.Fatalf("Unexpected error killing update failed pods: %v", err)
	}

	// Infra container should be stopped
	result, err := fakeDocker.InspectContainer(infraId)
	if err != nil {
		t.Fatalf("Unexpected error inspecting container: %v", err)
	}
	if result.State.Running != false {
		t.Fatalf("Infra container was not stopped")
	}

	// Unrelated container should still be running
	result, err = fakeDocker.InspectContainer(randomId)
	if err != nil {
		t.Fatalf("Unexpected error inspecting container: %v", err)
	}
	if result.State.Running != true {
		t.Fatalf("Unrelated container was stopped")
	}
}
