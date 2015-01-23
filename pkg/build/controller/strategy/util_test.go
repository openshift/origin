package strategy

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func TestSetupDockerSocketHostSocket(t *testing.T) {
	pod := kapi.Pod{
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{},
			},
		},
	}

	setupDockerSocket(&pod)

	if len(pod.Spec.Volumes) != 1 {
		t.Fatalf("Expected 1 volume, got: %#v", pod.Spec.Volumes)
	}
	volume := pod.Spec.Volumes[0]
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

	if len(pod.Spec.Containers[0].VolumeMounts) != 1 {
		t.Fatalf("Expected 1 volume mount, got: %#v", pod.Spec.Containers[0].VolumeMounts)
	}
	mount := pod.Spec.Containers[0].VolumeMounts[0]
	if e, a := "docker-socket", mount.Name; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if e, a := "/var/run/docker.sock", mount.MountPath; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if pod.Spec.Containers[0].Privileged {
		t.Error("Expected privileged to be false")
	}
}

func TestSetupBuildEnvFails(t *testing.T) {
	build := mockCustomBuild()
	containerEnv := []kapi.EnvVar{
		{Name: "BUILD", Value: ""},
		{Name: "SOURCE_REPOSITORY", Value: build.Parameters.Source.Git.URI},
	}
	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: build.PodName,
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  "custom-build",
					Image: build.Parameters.Strategy.CustomStrategy.Image,
					Env:   containerEnv,
					// TODO: run unprivileged https://github.com/openshift/origin/issues/662
					Privileged: true,
				},
			},
			RestartPolicy: kapi.RestartPolicy{
				Never: &kapi.RestartPolicyNever{},
			},
		},
	}
	if err := setupBuildEnv(build, pod); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	build.Parameters.Output.DockerImageReference = ""
	if err := setupBuildEnv(build, pod); err == nil {
		t.Errorf("unexpected non-error: %v", err)
	}
}
