package strategy

import (
	"os"
	"path"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// dockerSocketPath is the default path for the Docker socket inside the builder
// container
const dockerSocketPath = "/var/run/docker.sock"

// setupDockerSocket configures the pod to support the host's Docker socket
func setupDockerSocket(podSpec *kapi.Pod) {
	dockerSocketVolume := kapi.Volume{
		Name: "docker-socket",
		VolumeSource: kapi.VolumeSource{
			HostPath: &kapi.HostPathVolumeSource{
				Path: dockerSocketPath,
			},
		},
	}

	dockerSocketVolumeMount := kapi.VolumeMount{
		Name:      "docker-socket",
		MountPath: dockerSocketPath,
	}

	podSpec.Spec.Volumes = append(podSpec.Spec.Volumes,
		dockerSocketVolume)
	podSpec.Spec.Containers[0].VolumeMounts =
		append(podSpec.Spec.Containers[0].VolumeMounts,
			dockerSocketVolumeMount)
}

// setupDockerConfig configures the path to .dockercfg which contains registry credentials
func setupDockerConfig(podSpec *kapi.Pod) {
	dockerConfig := path.Join(os.Getenv("HOME"), ".dockercfg")
	if _, err := os.Stat(dockerConfig); os.IsNotExist(err) {
		return
	}
	dockerConfigVolume := kapi.Volume{
		Name: "docker-cfg",
		VolumeSource: kapi.VolumeSource{
			HostPath: &kapi.HostPathVolumeSource{
				Path: dockerConfig,
			},
		},
	}

	dockerConfigVolumeMount := kapi.VolumeMount{
		Name:      "docker-cfg",
		ReadOnly:  true,
		MountPath: "/root/.dockercfg",
	}

	podSpec.Spec.Volumes = append(podSpec.Spec.Volumes,
		dockerConfigVolume)
	podSpec.Spec.Containers[0].VolumeMounts =
		append(podSpec.Spec.Containers[0].VolumeMounts,
			dockerConfigVolumeMount)
}

// setupBuildEnv injects human-friendly environment variables which provides
// useful information about the current build.
func setupBuildEnv(build *buildapi.Build, pod *kapi.Pod) error {
	vars := []kapi.EnvVar{}

	switch build.Parameters.Source.Type {
	case buildapi.BuildSourceGit:
		vars = append(vars, kapi.EnvVar{"SOURCE_URI", build.Parameters.Source.Git.URI})
		vars = append(vars, kapi.EnvVar{"SOURCE_REF", build.Parameters.Source.Git.Ref})
	default:
		// Do nothing for unknown source types
	}

	ref, err := imageapi.ParseDockerImageReference(build.Parameters.Output.DockerImageReference)
	if err != nil {
		return err
	}
	vars = append(vars, kapi.EnvVar{"OUTPUT_REGISTRY", ref.Registry})
	ref.Registry = ""
	vars = append(vars, kapi.EnvVar{"OUTPUT_IMAGE", ref.String()})

	if len(pod.Spec.Containers) > 0 {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, vars...)
	}
	return nil
}
