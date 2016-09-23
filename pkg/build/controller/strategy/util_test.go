package strategy

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
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
	if volume.Name == "" {
		t.Fatalf("Unexpected empty volume source name")
	}
	if isVolumeSourceEmpty(volume.VolumeSource) {
		t.Fatalf("Unexpected nil volume source")
	}
	if volume.HostPath == nil {
		t.Fatalf("Unexpected nil host directory")
	}
	if volume.EmptyDir != nil {
		t.Errorf("Unexpected non-nil empty directory: %#v", volume.EmptyDir)
	}
	if e, a := "/var/run/docker.sock", volume.HostPath.Path; e != a {
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
	if pod.Spec.Containers[0].SecurityContext != nil && pod.Spec.Containers[0].SecurityContext.Privileged != nil && *pod.Spec.Containers[0].SecurityContext.Privileged {
		t.Error("Expected privileged to be false")
	}
}

func isVolumeSourceEmpty(volumeSource kapi.VolumeSource) bool {
	if volumeSource.EmptyDir == nil &&
		volumeSource.HostPath == nil &&
		volumeSource.GCEPersistentDisk == nil &&
		volumeSource.GitRepo == nil {
		return true
	}

	return false
}

func TestTrustedMergeEnvWithoutDuplicates(t *testing.T) {
	input := []kapi.EnvVar{
		{Name: "foo", Value: "bar"},
		{Name: "input", Value: "inputVal"},
		{Name: "BUILD_LOGLEVEL", Value: "loglevel"},
	}
	output := []kapi.EnvVar{
		{Name: "foo", Value: "test"},
	}

	mergeTrustedEnvWithoutDuplicates(input, &output)

	if len(output) != 2 {
		t.Errorf("Expected output to contain input items len==2 (%d)", len(output))
	}

	if output[0].Name != "foo" {
		t.Errorf("Expected output to have env 'foo', got %+v", output[0])
	}
	if output[0].Value != "test" {
		t.Errorf("Expected output env 'foo' to have value 'test', got %+v", output[0])
	}
	if output[1].Name != "BUILD_LOGLEVEL" {
		t.Errorf("Expected output to have env 'BUILD_LOGLEVEL', got %+v", output[1])
	}
	if output[1].Value != "loglevel" {
		t.Errorf("Expected output env 'BUILD_LOGLEVEL' to have value 'loglevel', got %+v", output[1])
	}
}

func TestSetupDockerSecrets(t *testing.T) {
	pod := kapi.Pod{
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{},
			},
		},
	}

	pushSecret := &kapi.LocalObjectReference{
		Name: "pushSecret",
	}
	pullSecret := &kapi.LocalObjectReference{
		Name: "pullSecret",
	}
	imageSources := []buildapi.ImageSource{
		{PullSecret: &kapi.LocalObjectReference{Name: "imageSourceSecret1"}},
		// this is a duplicate value on purpose, don't change it.
		{PullSecret: &kapi.LocalObjectReference{Name: "imageSourceSecret1"}},
	}

	setupDockerSecrets(&pod, pushSecret, pullSecret, imageSources)

	if len(pod.Spec.Volumes) != 4 {
		t.Fatalf("Expected 4 volumes, got: %#v", pod.Spec.Volumes)
	}

	seenName := map[string]bool{}
	for _, v := range pod.Spec.Volumes {
		if seenName[v.Name] {
			t.Errorf("Duplicate volume name %s", v.Name)
		}
		seenName[v.Name] = true
	}

	seenMount := map[string]bool{}
	seenMountPath := map[string]bool{}
	for _, m := range pod.Spec.Containers[0].VolumeMounts {
		if seenMount[m.Name] {
			t.Errorf("Duplicate volume mount name %s", m.Name)
		}
		seenMount[m.Name] = true

		if seenMountPath[m.MountPath] {
			t.Errorf("Duplicate volume mount path %s", m.MountPath)
		}
		seenMountPath[m.Name] = true
	}
}
