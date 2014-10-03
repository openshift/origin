package strategy

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func TestSetupDockerSocketHostSocket(t *testing.T) {
	pod := api.Pod{
		DesiredState: api.PodState{
			Manifest: api.ContainerManifest{
				Containers: []api.Container{
					{},
				},
			},
		},
	}

	setupDockerSocket(&pod)

	if len(pod.DesiredState.Manifest.Volumes) != 1 {
		t.Fatalf("Expected 1 volume, got: %#v", pod.DesiredState.Manifest.Volumes)
	}
	volume := pod.DesiredState.Manifest.Volumes[0]
	if e, a := "docker-socket", volume.Name; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if volume.Source == nil {
		t.Fatalf("Unexpected nil volume source")
	}
	if volume.Source.HostDir == nil {
		t.Fatalf("Unexpected nil host directory")
	}
	if volume.Source.EmptyDir != nil {
		t.Errorf("Unexpected non-nil empty directory: %#v", volume.Source.EmptyDir)
	}
	if e, a := "/var/run/docker.sock", volume.Source.HostDir.Path; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}

	if len(pod.DesiredState.Manifest.Containers[0].VolumeMounts) != 1 {
		t.Fatalf("Expected 1 volume mount, got: %#v", pod.DesiredState.Manifest.Containers[0].VolumeMounts)
	}
	mount := pod.DesiredState.Manifest.Containers[0].VolumeMounts[0]
	if e, a := "docker-socket", mount.Name; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if e, a := "/var/run/docker.sock", mount.MountPath; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if pod.DesiredState.Manifest.Containers[0].Privileged {
		t.Error("Expected privileged to be false")
	}
}
