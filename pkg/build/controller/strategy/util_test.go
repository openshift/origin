package strategy

import (
	"reflect"
	"testing"
	"unsafe"

	"k8s.io/api/core/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

func TestSetupDockerSocketHostSocket(t *testing.T) {
	pod := v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
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

func isVolumeSourceEmpty(volumeSource v1.VolumeSource) bool {
	if volumeSource.EmptyDir == nil &&
		volumeSource.HostPath == nil &&
		volumeSource.GCEPersistentDisk == nil &&
		volumeSource.GitRepo == nil {
		return true
	}

	return false
}

func TestSetupDockerSecrets(t *testing.T) {
	pod := v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{},
			},
		},
	}

	pushSecret := &kapi.LocalObjectReference{
		Name: "my.pushSecret.with.full.stops.and.longer.than.sixty.three.characters",
	}
	pullSecret := &kapi.LocalObjectReference{
		Name: "pullSecret",
	}
	imageSources := []buildapi.ImageSource{
		{PullSecret: &kapi.LocalObjectReference{Name: "imageSourceSecret1"}},
		// this is a duplicate value on purpose, don't change it.
		{PullSecret: &kapi.LocalObjectReference{Name: "imageSourceSecret1"}},
	}

	setupDockerSecrets(&pod, &pod.Spec.Containers[0], pushSecret, pullSecret, imageSources)

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

	if !seenName["my-pushSecret-with-full-stops-and-longer-than-six-c6eb4d75-push"] {
		t.Errorf("volume my-pushSecret-with-full-stops-and-longer-than-six-c6eb4d75-push was not seen")
	}
	if !seenName["pullSecret-pull"] {
		t.Errorf("volume pullSecret-pull was not seen")
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

	if !seenMount["my-pushSecret-with-full-stops-and-longer-than-six-c6eb4d75-push"] {
		t.Errorf("volumemount my-pushSecret-with-full-stops-and-longer-than-six-c6eb4d75-push was not seen")
	}
	if !seenMount["pullSecret-pull"] {
		t.Errorf("volumemount pullSecret-pull was not seen")
	}
}

func TestCopyEnvVarSlice(t *testing.T) {
	s1 := []v1.EnvVar{{Name: "FOO", Value: "bar"}, {Name: "BAZ", Value: "qux"}}
	s2 := copyEnvVarSlice(s1)

	if !reflect.DeepEqual(s1, s2) {
		t.Error(s2)
	}

	if (*reflect.SliceHeader)(unsafe.Pointer(&s1)).Data == (*reflect.SliceHeader)(unsafe.Pointer(&s2)).Data {
		t.Error("copyEnvVarSlice didn't copy backing store")
	}
}

func checkAliasing(t *testing.T, pod *v1.Pod) {
	m := map[uintptr]bool{}
	for _, c := range pod.Spec.Containers {
		p := (*reflect.SliceHeader)(unsafe.Pointer(&c.Env)).Data
		if m[p] {
			t.Error("pod Env slices are aliased")
			return
		}
		m[p] = true
	}
	for _, c := range pod.Spec.InitContainers {
		p := (*reflect.SliceHeader)(unsafe.Pointer(&c.Env)).Data
		if m[p] {
			t.Error("pod Env slices are aliased")
			return
		}
		m[p] = true
	}
}
