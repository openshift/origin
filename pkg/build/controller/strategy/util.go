package strategy

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapiv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/version"
)

const (
	// dockerSocketPath is the default path for the Docker socket inside the builder container
	dockerSocketPath      = "/var/run/docker.sock"
	sourceSecretMountPath = "/var/run/secrets/openshift.io/source"

	DockerPushSecretMountPath      = "/var/run/secrets/openshift.io/push"
	DockerPullSecretMountPath      = "/var/run/secrets/openshift.io/pull"
	SecretBuildSourceBaseMountPath = "/var/run/secrets/openshift.io/build"
	SourceImagePullSecretMountPath = "/var/run/secrets/openshift.io/source-image"

	// ExtractImageContentContainer is the name of the container that will
	// pull down input images and extract their content for input to the build.
	ExtractImageContentContainer = "extract-image-content"

	// GitCloneContainer is the name of the container that will clone the
	// build source repository and also handle binary input content.
	GitCloneContainer = "git-clone"
)

const (
	CustomBuild = "custom-build"
	DockerBuild = "docker-build"
	StiBuild    = "sti-build"
)

var BuildContainerNames = []string{CustomBuild, StiBuild, DockerBuild}

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
func setupDockerSocket(pod *v1.Pod) {
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

	pod.Spec.Volumes = append(pod.Spec.Volumes,
		dockerSocketVolume)
	pod.Spec.Containers[0].VolumeMounts =
		append(pod.Spec.Containers[0].VolumeMounts,
			dockerSocketVolumeMount)
	for i, initContainer := range pod.Spec.InitContainers {
		if initContainer.Name == ExtractImageContentContainer {
			pod.Spec.InitContainers[i].VolumeMounts = append(pod.Spec.InitContainers[i].VolumeMounts, dockerSocketVolumeMount)
			break
		}
	}
}

// setupCrioSocket configures the pod to support the host's Crio socket
func setupCrioSocket(pod *v1.Pod) {
	crioSocketVolume := v1.Volume{
		Name: "crio-socket",
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/var/run/crio/crio.sock",
			},
		},
	}

	crioSocketVolumeMount := v1.VolumeMount{
		Name:      "crio-socket",
		MountPath: "/var/run/crio/crio.sock",
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes,
		crioSocketVolume)
	pod.Spec.Containers[0].VolumeMounts =
		append(pod.Spec.Containers[0].VolumeMounts,
			crioSocketVolumeMount)
}

// mountSecretVolume is a helper method responsible for actual mounting secret
// volumes into a pod.
func mountSecretVolume(pod *v1.Pod, container *v1.Container, secretName, mountPath, volumeSuffix string) {
	volumeName := apihelpers.GetName(secretName, volumeSuffix, kvalidation.DNS1123LabelMaxLength)

	// coerce from RFC1123 subdomain to RFC1123 label.
	volumeName = strings.Replace(volumeName, ".", "-", -1)

	volumeExists := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == volumeName {
			volumeExists = true
			break
		}
	}
	mode := int32(0600)
	if !volumeExists {
		volume := v1.Volume{
			Name: volumeName,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName:  secretName,
					DefaultMode: &mode,
				},
			},
		}
		pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	}

	volumeMount := v1.VolumeMount{
		Name:      volumeName,
		MountPath: mountPath,
		ReadOnly:  true,
	}
	container.VolumeMounts = append(container.VolumeMounts, volumeMount)
}

// setupDockerSecrets mounts Docker Registry secrets into Pod running the build,
// allowing Docker to authenticate against private registries or Docker Hub.
func setupDockerSecrets(pod *v1.Pod, container *v1.Container, pushSecret, pullSecret *kapi.LocalObjectReference, imageSources []buildapi.ImageSource) {
	if pushSecret != nil {
		mountSecretVolume(pod, container, pushSecret.Name, DockerPushSecretMountPath, "push")
		container.Env = append(container.Env, []v1.EnvVar{
			{Name: "PUSH_DOCKERCFG_PATH", Value: DockerPushSecretMountPath},
		}...)
		glog.V(3).Infof("%s will be used for docker push in %s", DockerPushSecretMountPath, pod.Name)
	}

	if pullSecret != nil {
		mountSecretVolume(pod, container, pullSecret.Name, DockerPullSecretMountPath, "pull")
		container.Env = append(container.Env, []v1.EnvVar{
			{Name: "PULL_DOCKERCFG_PATH", Value: DockerPullSecretMountPath},
		}...)
		glog.V(3).Infof("%s will be used for docker pull in %s", DockerPullSecretMountPath, pod.Name)
	}

	for i, imageSource := range imageSources {
		if imageSource.PullSecret == nil {
			continue
		}
		mountPath := filepath.Join(SourceImagePullSecretMountPath, strconv.Itoa(i))
		mountSecretVolume(pod, container, imageSource.PullSecret.Name, mountPath, fmt.Sprintf("%s%d", "source-image", i))
		container.Env = append(container.Env, []v1.EnvVar{
			{Name: fmt.Sprintf("%s%d", "PULL_SOURCE_DOCKERCFG_PATH_", i), Value: mountPath},
		}...)
		glog.V(3).Infof("%s will be used for docker pull in %s", mountPath, pod.Name)
	}
}

// setupSourceSecrets mounts SSH key used for accessing private SCM to clone
// application source code during build.
func setupSourceSecrets(pod *v1.Pod, container *v1.Container, sourceSecret *kapi.LocalObjectReference) {
	if sourceSecret == nil {
		return
	}

	mountSecretVolume(pod, container, sourceSecret.Name, sourceSecretMountPath, "source")
	glog.V(3).Infof("Installed source secrets in %s, in Pod %s/%s", sourceSecretMountPath, pod.Namespace, pod.Name)
	container.Env = append(container.Env, []v1.EnvVar{
		{Name: "SOURCE_SECRET_PATH", Value: sourceSecretMountPath},
	}...)
}

// setupInputSecrets mounts the secrets referenced by the SecretBuildSource
// into a builder container.
func setupInputSecrets(pod *v1.Pod, container *v1.Container, secrets []buildapi.SecretBuildSource) {
	for _, s := range secrets {
		mountSecretVolume(pod, container, s.Secret.Name, filepath.Join(SecretBuildSourceBaseMountPath, s.Secret.Name), "build")
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
func setupAdditionalSecrets(pod *v1.Pod, container *v1.Container, secrets []buildapi.SecretSpec) {
	for _, secretSpec := range secrets {
		mountSecretVolume(pod, container, secretSpec.SecretSource.Name, secretSpec.MountPath, "secret")
		glog.V(3).Infof("Installed additional secret in %s, in Pod %s/%s", secretSpec.MountPath, pod.Namespace, pod.Name)
	}
}

// getPodLabels creates labels for the Build Pod
func getPodLabels(build *buildapi.Build) map[string]string {
	return map[string]string{buildapi.BuildLabel: buildapi.LabelValue(build.Name)}
}

func makeOwnerReference(build *buildapi.Build) metav1.OwnerReference {
	t := true
	return metav1.OwnerReference{
		APIVersion: BuildControllerRefKind.GroupVersion().String(),
		Kind:       BuildControllerRefKind.Kind,
		Name:       build.Name,
		UID:        build.UID,
		Controller: &t,
	}
}

func setOwnerReference(pod *v1.Pod, build *buildapi.Build) {
	pod.OwnerReferences = []metav1.OwnerReference{makeOwnerReference(build)}
}

// HasOwnerReference returns true if the build pod has an OwnerReference to the
// build.
func HasOwnerReference(pod *v1.Pod, build *buildapi.Build) bool {
	ref := makeOwnerReference(build)

	for _, r := range pod.OwnerReferences {
		if reflect.DeepEqual(r, ref) {
			return true
		}
	}

	return false
}

// copyEnvVarSlice returns a copy of an []v1.EnvVar
func copyEnvVarSlice(in []v1.EnvVar) []v1.EnvVar {
	out := make([]v1.EnvVar, len(in))
	copy(out, in)
	return out
}
