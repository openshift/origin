package strategy

import (
	"path/filepath"

	"github.com/golang/glog"
	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util/namer"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
)

const (
	// dockerSocketPath is the default path for the Docker socket inside the builder container
	dockerSocketPath          = "/var/run/docker.sock"
	DockerPushSecretMountPath = "/var/run/secrets/openshift.io/push"
	DockerPullSecretMountPath = "/var/run/secrets/openshift.io/pull"
	sourceSecretMountPath     = "/var/run/secrets/openshift.io/source"
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

	switch build.Spec.Source.Type {
	case buildapi.BuildSourceGit:
		vars = append(vars, kapi.EnvVar{Name: "SOURCE_URI", Value: build.Spec.Source.Git.URI})
		vars = append(vars, kapi.EnvVar{Name: "SOURCE_REF", Value: build.Spec.Source.Git.Ref})
	default:
		// Do nothing for unknown source types
	}

	if build.Spec.Output.To != nil {
		// output much always be a DockerImage type reference at this point.
		ref, err := imageapi.ParseDockerImageReference(build.Spec.Output.To.Name)
		if err != nil {
			return err
		}
		vars = append(vars, kapi.EnvVar{Name: "OUTPUT_REGISTRY", Value: ref.Registry})
		ref.Registry = ""
		vars = append(vars, kapi.EnvVar{Name: "OUTPUT_IMAGE", Value: ref.String()})
	}

	if len(pod.Spec.Containers) > 0 {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, vars...)
	}
	return nil
}

// mountSecretVolume is a helper method responsible for actual mounting secret
// volumes into a pod.
func mountSecretVolume(pod *kapi.Pod, secretName, mountPath, volumePrefix string) {
	volumeName := namer.GetName(secretName, volumePrefix, util.DNS1123SubdomainMaxLength)
	volume := kapi.Volume{
		Name: volumeName,
		VolumeSource: kapi.VolumeSource{
			Secret: &kapi.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
	volumeMount := kapi.VolumeMount{
		Name:      volumeName,
		MountPath: mountPath,
		ReadOnly:  true,
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, volumeMount)
}

// setupDockerSecrets mounts Docker Registry secrets into Pod running the build,
// allowing Docker to authenticate against private registries or Docker Hub.
func setupDockerSecrets(pod *kapi.Pod, pushSecret, pullSecret *kapi.LocalObjectReference) {
	if pushSecret != nil {
		mountSecretVolume(pod, pushSecret.Name, DockerPushSecretMountPath, "push")
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []kapi.EnvVar{
			{Name: "PUSH_DOCKERCFG_PATH", Value: filepath.Join(DockerPushSecretMountPath, kapi.DockerConfigKey)},
		}...)
		glog.V(3).Infof("%s be used for docker push in %s", DockerPullSecretMountPath, pod.Name)
	}

	if pullSecret != nil {
		mountSecretVolume(pod, pullSecret.Name, DockerPullSecretMountPath, "pull")
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []kapi.EnvVar{
			{Name: "PULL_DOCKERCFG_PATH", Value: filepath.Join(DockerPullSecretMountPath, kapi.DockerConfigKey)},
		}...)
		glog.V(3).Infof("%s be used for docker pull in %s", DockerPullSecretMountPath, pod.Name)
	}
}

// setupSourceSecrets mounts SSH key used for accessing private SCM to clone
// application source code during build.
func setupSourceSecrets(pod *kapi.Pod, sourceSecret *kapi.LocalObjectReference) {
	if sourceSecret == nil {
		return
	}

	mountSecretVolume(pod, sourceSecret.Name, sourceSecretMountPath, "source")
	glog.V(3).Infof("Installed source secrets in %s, in Pod %s/%s", sourceSecretMountPath, pod.Namespace, pod.Name)
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []kapi.EnvVar{
		{Name: "SOURCE_SECRET_PATH", Value: sourceSecretMountPath},
	}...)
}

// addSourceEnvVars adds environment variables related to the source code
// repository to builder container
func addSourceEnvVars(source buildapi.BuildSource, output *[]kapi.EnvVar) {
	sourceVars := []kapi.EnvVar{}
	if source.Git != nil {
		sourceVars = append(sourceVars, kapi.EnvVar{Name: "SOURCE_REPOSITORY", Value: source.Git.URI})
	}
	if len(source.ContextDir) > 0 {
		sourceVars = append(sourceVars, kapi.EnvVar{Name: "SOURCE_CONTEXT_DIR", Value: source.ContextDir})
	}
	if source.Git != nil && len(source.Git.Ref) > 0 {
		sourceVars = append(sourceVars, kapi.EnvVar{Name: "SOURCE_REF", Value: source.Git.Ref})
	}
	*output = append(*output, sourceVars...)
}

// setupAdditionalSecrets creates secret volume mounts in the given pod for the given list of secrets
func setupAdditionalSecrets(pod *kapi.Pod, secrets []buildapi.SecretSpec) {
	for _, secretSpec := range secrets {
		mountSecretVolume(pod, secretSpec.SecretSource.Name, secretSpec.MountPath, "secret")
		glog.V(3).Infof("Installed additional secret in %s, in Pod %s/%s", secretSpec.MountPath, pod.Namespace, pod.Name)
	}
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

// getPodLabels creates labels for the Build Pod
func getPodLabels(build *buildapi.Build) map[string]string {
	labels := make(map[string]string)
	labels[buildapi.BuildLabel] = build.Name
	append(build.ObjectMeta.Labels, labels)

	return labels
}

// append adds elements from one map to another
func append(source *map[string]string, dest *map[string]string) {
	for key, value := range source {
		dest[key] = value
	}
}
