package strategy

import (
	"path/filepath"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"
	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// dockerSocketPath is the default path for the Docker socket inside the builder
// container
const (
	dockerSocketPath          = "/var/run/docker.sock"
	dockerPushSecretMountPath = "/var/run/secrets/push"
	// TODO: The pull secrets is the same as push secret for now.
	//       This will be replaced using Service Account.
	dockerPullSecretMountPath = dockerPushSecretMountPath
)

var whitelistEnvVarNames = []string{"BUILD_LOGLEVEL"}

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

// setupBuildEnv injects human-friendly environment variables which provides
// useful information about the current build.
func setupBuildEnv(build *buildapi.Build, pod *kapi.Pod) error {
	vars := []kapi.EnvVar{}

	switch build.Parameters.Source.Type {
	case buildapi.BuildSourceGit:
		vars = append(vars, kapi.EnvVar{Name: "SOURCE_URI", Value: build.Parameters.Source.Git.URI})
		vars = append(vars, kapi.EnvVar{Name: "SOURCE_REF", Value: build.Parameters.Source.Git.Ref})
	default:
		// Do nothing for unknown source types
	}

	ref, err := imageapi.ParseDockerImageReference(build.Parameters.Output.DockerImageReference)
	if err != nil {
		return err
	}
	vars = append(vars, kapi.EnvVar{Name: "OUTPUT_REGISTRY", Value: ref.Registry})
	ref.Registry = ""
	vars = append(vars, kapi.EnvVar{Name: "OUTPUT_IMAGE", Value: ref.String()})

	if len(pod.Spec.Containers) > 0 {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, vars...)
	}
	return nil
}

// setupDockerSecrets mounts Docker Registry secrets into Pod running the build,
// allowing Docker to authenticate against private registries or Docker Hub.
func setupDockerSecrets(pod *kapi.Pod, pushSecret string) {
	if len(pushSecret) == 0 {
		return
	}

	volume := kapi.Volume{
		Name: pushSecret,
		VolumeSource: kapi.VolumeSource{
			Secret: &kapi.SecretVolumeSource{
				SecretName: pushSecret,
			},
		},
	}
	volumeMount := kapi.VolumeMount{
		Name:      pushSecret,
		MountPath: dockerPushSecretMountPath,
		ReadOnly:  true,
	}

	glog.V(3).Infof("Installed %s as docker push secret in Pod %s", volumeMount.MountPath, pod.Name)
	pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, volumeMount)
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []kapi.EnvVar{
		{Name: "PUSH_DOCKERCFG_PATH", Value: filepath.Join(dockerPushSecretMountPath, "dockercfg")},
		{Name: "PULL_DOCKERCFG_PATH", Value: filepath.Join(dockerPullSecretMountPath, "dockercfg")},
	}...)
}

// mergeTrustedEnvWithoutDuplicates merges two environment lists without having
// duplicate items in the output list.  Only trusted environment variables
// will be merged.
func mergeTrustedEnvWithoutDuplicates(source []kapi.EnvVar, output *[]kapi.EnvVar) {

	// filter out all environment variables except trusted/well known
	// values, because we do not want random environment variables being
	// fed into the privileged STI container via the BuildConfig definition.
	filteredSource := []kapi.EnvVar{}
	for _, env := range source {
		trusted := false
		for _, acceptable := range whitelistEnvVarNames {
			if env.Name == acceptable {
				trusted = true
				break
			}
		}
		if !trusted {
			continue
		}
		filteredSource = append(filteredSource, env)
	}

	type sourceMapItem struct {
		index int
		value string
	}
	// Convert source to Map for faster access
	sourceMap := make(map[string]sourceMapItem)
	for i, env := range filteredSource {
		sourceMap[env.Name] = sourceMapItem{i, env.Value}
	}
	result := *output
	for i, env := range result {
		// If the value exists in output, override it and remove it
		// from the source list
		if v, found := sourceMap[env.Name]; found {
			result[i].Value = v.value
			filteredSource = append(filteredSource[:v.index], filteredSource[v.index+1:]...)
		}
	}
	*output = append(result, filteredSource...)
}

// getContainerVerbosity returns the defined BUILD_LOGLEVEL value
func getContainerVerbosity(containerEnv []kapi.EnvVar) (verbosity string) {
	for _, env := range containerEnv {
		if env.Name == "BUILD_LOGLEVEL" {
			verbosity = env.Value
			break
		}
	}
	return
}

// getPodLabels copies build labels and adds additional one with build name itself
func getPodLabels(build *buildapi.Build) map[string]string {
	podLabels := make(map[string]string)
	for k, v := range build.Labels {
		podLabels[k] = v
	}
	podLabels[buildapi.BuildLabel] = build.Name
	return podLabels
}
