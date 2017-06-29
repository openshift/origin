package strategy

import (
	"fmt"
	"path/filepath"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"

	"github.com/golang/glog"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/util/namer"
	"github.com/openshift/origin/pkg/version"
)

const (
	// dockerSocketPath is the default path for the Docker socket inside the builder container
	dockerSocketPath               = "/var/run/docker.sock"
	DockerPushSecretMountPath      = "/var/run/secrets/openshift.io/push"
	DockerPullSecretMountPath      = "/var/run/secrets/openshift.io/pull"
	SecretBuildSourceBaseMountPath = "/var/run/secrets/openshift.io/build"
	SourceImagePullSecretMountPath = "/var/run/secrets/openshift.io/source-image"
	sourceSecretMountPath          = "/var/run/secrets/openshift.io/source"
)

var (
	// BuildControllerRefKind contains the schema.GroupVersionKind for builds.
	// This is used in the ownerRef of builder pods.
	BuildControllerRefKind = buildapiv1.SchemeGroupVersion.WithKind("Build")
)

// FatalError is an error which can't be retried.
type FatalError struct {
	// Reason the fatal error occurred
	Reason string
}

// Error implements the error interface.
func (e *FatalError) Error() string {
	return fmt.Sprintf("fatal error: %s", e.Reason)
}

// IsFatal returns true if the error is fatal
func IsFatal(err error) bool {
	_, isFatal := err.(*FatalError)
	return isFatal
}

// setupDockerSocket configures the pod to support the host's Docker socket
func setupDockerSocket(podSpec *v1.Pod) {
	dockerSocketVolume := v1.Volume{
		Name: "docker-socket",
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: dockerSocketPath,
			},
		},
	}

	dockerSocketVolumeMount := v1.VolumeMount{
		Name:      "docker-socket",
		MountPath: dockerSocketPath,
	}

	podSpec.Spec.Volumes = append(podSpec.Spec.Volumes,
		dockerSocketVolume)
	podSpec.Spec.Containers[0].VolumeMounts =
		append(podSpec.Spec.Containers[0].VolumeMounts,
			dockerSocketVolumeMount)
}

// mountSecretVolume is a helper method responsible for actual mounting secret
// volumes into a pod.
func mountSecretVolume(pod *v1.Pod, secretName, mountPath, volumeSuffix string) {
	volumeName := namer.GetName(secretName, volumeSuffix, kvalidation.DNS1123SubdomainMaxLength)
	volume := v1.Volume{
		Name: volumeName,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
	volumeMount := v1.VolumeMount{
		Name:      volumeName,
		MountPath: mountPath,
		ReadOnly:  true,
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, volumeMount)
}

// setupDockerSecrets mounts Docker Registry secrets into Pod running the build,
// allowing Docker to authenticate against private registries or Docker Hub.
func setupDockerSecrets(pod *v1.Pod, pushSecret, pullSecret *kapi.LocalObjectReference, imageSources []buildapi.ImageSource) {
	if pushSecret != nil {
		mountSecretVolume(pod, pushSecret.Name, DockerPushSecretMountPath, "push")
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []v1.EnvVar{
			{Name: dockercfg.PushAuthType, Value: DockerPushSecretMountPath},
		}...)
		glog.V(3).Infof("%s will be used for docker push in %s", DockerPushSecretMountPath, pod.Name)
	}

	if pullSecret != nil {
		mountSecretVolume(pod, pullSecret.Name, DockerPullSecretMountPath, "pull")
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []v1.EnvVar{
			{Name: dockercfg.PullAuthType, Value: DockerPullSecretMountPath},
		}...)
		glog.V(3).Infof("%s will be used for docker pull in %s", DockerPullSecretMountPath, pod.Name)
	}

	for i, imageSource := range imageSources {
		if imageSource.PullSecret == nil {
			continue
		}
		mountPath := filepath.Join(SourceImagePullSecretMountPath, strconv.Itoa(i))
		mountSecretVolume(pod, imageSource.PullSecret.Name, mountPath, fmt.Sprintf("%s%d", "source-image", i))
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []v1.EnvVar{
			{Name: fmt.Sprintf("%s%d", dockercfg.PullSourceAuthType, i), Value: mountPath},
		}...)
		glog.V(3).Infof("%s will be used for docker pull in %s", mountPath, pod.Name)

	}
}

// setupSourceSecrets mounts SSH key used for accessing private SCM to clone
// application source code during build.
func setupSourceSecrets(pod *v1.Pod, sourceSecret *kapi.LocalObjectReference) {
	if sourceSecret == nil {
		return
	}

	mountSecretVolume(pod, sourceSecret.Name, sourceSecretMountPath, "source")
	glog.V(3).Infof("Installed source secrets in %s, in Pod %s/%s", sourceSecretMountPath, pod.Namespace, pod.Name)
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []v1.EnvVar{
		{Name: "SOURCE_SECRET_PATH", Value: sourceSecretMountPath},
	}...)
}

// setupSecrets mounts the secrets referenced by the SecretBuildSource
// into a builder container. It also sets an environment variable that contains
// a name of the secret and the destination directory.
func setupSecrets(pod *v1.Pod, secrets []buildapi.SecretBuildSource) {
	for _, s := range secrets {
		mountSecretVolume(pod, s.Secret.Name, filepath.Join(SecretBuildSourceBaseMountPath, s.Secret.Name), "build")
		glog.V(3).Infof("%s will be used as a build secret in %s", s.Secret.Name, SecretBuildSourceBaseMountPath)
	}
}

// addSourceEnvVars adds environment variables related to the source code
// repository to builder container
func addSourceEnvVars(source buildapi.BuildSource, output *[]v1.EnvVar) {
	sourceVars := []v1.EnvVar{}
	if source.Git != nil {
		sourceVars = append(sourceVars, v1.EnvVar{Name: "SOURCE_REPOSITORY", Value: source.Git.URI})
		sourceVars = append(sourceVars, v1.EnvVar{Name: "SOURCE_URI", Value: source.Git.URI})
	}
	if len(source.ContextDir) > 0 {
		sourceVars = append(sourceVars, v1.EnvVar{Name: "SOURCE_CONTEXT_DIR", Value: source.ContextDir})
	}
	if source.Git != nil && len(source.Git.Ref) > 0 {
		sourceVars = append(sourceVars, v1.EnvVar{Name: "SOURCE_REF", Value: source.Git.Ref})
	}
	*output = append(*output, sourceVars...)
}

func addOriginVersionVar(output *[]v1.EnvVar) {
	version := v1.EnvVar{Name: buildapi.OriginVersion, Value: version.Get().String()}
	*output = append(*output, version)
}

// addOutputEnvVars adds env variables that provide information about the output
// target for the build
func addOutputEnvVars(buildOutput *kapi.ObjectReference, output *[]v1.EnvVar) error {
	if buildOutput == nil {
		return nil
	}

	// output must always be a DockerImage type reference at this point.
	if buildOutput.Kind != "DockerImage" {
		return fmt.Errorf("invalid build output kind %s, must be DockerImage", buildOutput.Kind)
	}
	ref, err := imageapi.ParseDockerImageReference(buildOutput.Name)
	if err != nil {
		return err
	}
	registry := ref.Registry
	ref.Registry = ""
	image := ref.String()

	outputVars := []v1.EnvVar{
		{Name: "OUTPUT_REGISTRY", Value: registry},
		{Name: "OUTPUT_IMAGE", Value: image},
	}

	*output = append(*output, outputVars...)
	return nil
}

// setupAdditionalSecrets creates secret volume mounts in the given pod for the given list of secrets
func setupAdditionalSecrets(pod *v1.Pod, secrets []buildapi.SecretSpec) {
	for _, secretSpec := range secrets {
		mountSecretVolume(pod, secretSpec.SecretSource.Name, secretSpec.MountPath, "secret")
		glog.V(3).Infof("Installed additional secret in %s, in Pod %s/%s", secretSpec.MountPath, pod.Namespace, pod.Name)
	}
}

// getContainerVerbosity returns the defined BUILD_LOGLEVEL value
func getContainerVerbosity(containerEnv []v1.EnvVar) (verbosity string) {
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
	return map[string]string{buildapi.BuildLabel: buildapi.LabelValue(build.Name)}
}

func setOwnerReference(pod *v1.Pod, build *buildapi.Build) {
	t := true
	pod.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: BuildControllerRefKind.GroupVersion().String(),
			Kind:       BuildControllerRefKind.Kind,
			Name:       build.Name,
			UID:        build.UID,
			Controller: &t,
		},
	}
}
