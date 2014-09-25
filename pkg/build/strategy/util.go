package strategy

import (
	"os"
	"path"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// setupDockerSocket configures the pod to support the host's Docker socket
func setupDockerSocket(podSpec *api.Pod) {
	dockerSocketVolume := api.Volume{
		Name: "docker-socket",
		Source: &api.VolumeSource{
			HostDirectory: &api.HostDirectory{
				Path: "/var/run/docker.sock",
			},
		},
	}

	dockerSocketVolumeMount := api.VolumeMount{
		Name:      "docker-socket",
		MountPath: "/var/run/docker.sock",
	}

	podSpec.DesiredState.Manifest.Volumes = append(podSpec.DesiredState.Manifest.Volumes,
		dockerSocketVolume)
	podSpec.DesiredState.Manifest.Containers[0].VolumeMounts =
		append(podSpec.DesiredState.Manifest.Containers[0].VolumeMounts,
			dockerSocketVolumeMount)
}

// setupDockerConfig configures the path to .dockercfg which contains registry credentials
func setupDockerConfig(podSpec *api.Pod) {
	dockerConfig := path.Join(os.Getenv("HOME"), ".dockercfg")
	if _, err := os.Stat(dockerConfig); os.IsNotExist(err) {
		return
	}
	dockerConfigVolume := api.Volume{
		Name: "docker-cfg",
		Source: &api.VolumeSource{
			HostDirectory: &api.HostDirectory{
				Path: dockerConfig,
			},
		},
	}

	dockerConfigVolumeMount := api.VolumeMount{
		Name:      "docker-cfg",
		ReadOnly:  true,
		MountPath: "/.dockercfg",
	}

	podSpec.DesiredState.Manifest.Volumes = append(podSpec.DesiredState.Manifest.Volumes,
		dockerConfigVolume)
	podSpec.DesiredState.Manifest.Containers[0].VolumeMounts =
		append(podSpec.DesiredState.Manifest.Containers[0].VolumeMounts,
			dockerConfigVolumeMount)
}
