package strategy

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildv1 "github.com/openshift/api/build/v1"
	buildutil "github.com/openshift/origin/pkg/build/buildutil"
)

var (
	buildEncodingScheme       = runtime.NewScheme()
	buildEncodingCodecFactory = serializer.NewCodecFactory(buildEncodingScheme)
	buildJSONCodec            runtime.Encoder
)

func init() {
	utilruntime.Must(buildv1.Install(buildEncodingScheme))
	buildJSONCodec = buildEncodingCodecFactory.LegacyCodec(buildv1.GroupVersion)
}

// DockerBuildStrategy creates a Docker build using a Docker builder image.
type DockerBuildStrategy struct {
	Image string
}

// CreateBuildPod creates the pod to be used for the Docker build
// TODO: Make the Pod definition configurable
func (bs *DockerBuildStrategy) CreateBuildPod(build *buildv1.Build, additionalCAs map[string]string, internalRegistryHost string) (*v1.Pod, error) {
	data, err := runtime.Encode(buildJSONCodec, build)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the build: %v", err)
	}

	privileged := true
	strategy := build.Spec.Strategy.DockerStrategy

	containerEnv := []v1.EnvVar{
		{Name: "BUILD", Value: string(data)},
		{Name: "LANG", Value: "en_US.utf8"},
	}

	addSourceEnvVars(build.Spec.Source, &containerEnv)

	if len(strategy.Env) > 0 {
		buildutil.MergeTrustedEnvWithoutDuplicates(strategy.Env, &containerEnv, true)
	}

	serviceAccount := build.Spec.ServiceAccount
	if len(serviceAccount) == 0 {
		serviceAccount = buildutil.BuilderServiceAccountName
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildutil.GetBuildPodName(build),
			Namespace: build.Namespace,
			Labels:    getPodLabels(build),
		},
		Spec: v1.PodSpec{
			ServiceAccountName: serviceAccount,
			Containers: []v1.Container{
				{
					Name:    DockerBuild,
					Image:   bs.Image,
					Command: []string{"openshift-docker-build"},
					Env:     copyEnvVarSlice(containerEnv),
					// TODO: run unprivileged https://github.com/openshift/origin/issues/662
					SecurityContext: &v1.SecurityContext{
						Privileged: &privileged,
					},
					TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "buildworkdir",
							MountPath: buildutil.BuildWorkDirMount,
						},
						{
							Name:      "buildcachedir",
							MountPath: buildutil.BuildBlobsMetaCache,
						},
					},
					ImagePullPolicy: v1.PullIfNotPresent,
					Resources:       build.Spec.Resources,
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "buildcachedir",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{Path: buildutil.BuildBlobsMetaCache},
					},
				},
				{
					Name: "buildworkdir",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			NodeSelector:  build.Spec.NodeSelector,
		},
	}

	// can't conditionalize the manage-dockerfile init container because we don't
	// know until we've cloned, whether or not we've got a dockerfile to manage
	// (also if it's a docker type build, we should always have a dockerfile to manage)
	if build.Spec.Source.Git != nil || build.Spec.Source.Binary != nil {
		gitCloneContainer := v1.Container{
			Name:                     GitCloneContainer,
			Image:                    bs.Image,
			Command:                  []string{"openshift-git-clone"},
			Env:                      copyEnvVarSlice(containerEnv),
			TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "buildworkdir",
					MountPath: buildutil.BuildWorkDirMount,
				},
			},
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources:       build.Spec.Resources,
		}
		if build.Spec.Source.Binary != nil {
			gitCloneContainer.Stdin = true
			gitCloneContainer.StdinOnce = true
		}
		setupSourceSecrets(pod, &gitCloneContainer, build.Spec.Source.SourceSecret)
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, gitCloneContainer)
	}
	if len(build.Spec.Source.Images) > 0 {
		extractImageContentContainer := v1.Container{
			Name:    ExtractImageContentContainer,
			Image:   bs.Image,
			Command: []string{"openshift-extract-image-content"},
			Env:     copyEnvVarSlice(containerEnv),
			// TODO: run unprivileged https://github.com/openshift/origin/issues/662
			SecurityContext: &v1.SecurityContext{
				Privileged: &privileged,
			},
			TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "buildworkdir",
					MountPath: buildutil.BuildWorkDirMount,
				},
				{
					Name:      "buildcachedir",
					MountPath: buildutil.BuildBlobsMetaCache,
				},
			},
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources:       build.Spec.Resources,
		}
		setupDockerSecrets(pod, &extractImageContentContainer, build.Spec.Output.PushSecret, strategy.PullSecret, build.Spec.Source.Images)
		setupContainersStorage(pod, &extractImageContentContainer)
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, extractImageContentContainer)
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers,
		v1.Container{
			Name:                     "manage-dockerfile",
			Image:                    bs.Image,
			Command:                  []string{"openshift-manage-dockerfile"},
			Env:                      copyEnvVarSlice(containerEnv),
			TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "buildworkdir",
					MountPath: buildutil.BuildWorkDirMount,
				},
			},
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources:       build.Spec.Resources,
		},
	)

	if build.Spec.CompletionDeadlineSeconds != nil {
		pod.Spec.ActiveDeadlineSeconds = build.Spec.CompletionDeadlineSeconds
	}

	setOwnerReference(pod, build)
	setupDockerSecrets(pod, &pod.Spec.Containers[0], build.Spec.Output.PushSecret, strategy.PullSecret, build.Spec.Source.Images)
	// For any secrets the user wants to reference from their Assemble script or Dockerfile, mount those
	// secrets into the main container.  The main container includes logic to copy them from the mounted
	// location into the working directory.
	// TODO: consider moving this into the git-clone container and doing the secret copying there instead.
	setupInputSecrets(pod, &pod.Spec.Containers[0], build.Spec.Source.Secrets)
	setupInputConfigMaps(pod, &pod.Spec.Containers[0], build.Spec.Source.ConfigMaps)
	setupContainersConfigs(build, pod)
	setupBuildCAs(build, pod, additionalCAs, internalRegistryHost)
	setupContainersStorage(pod, &pod.Spec.Containers[0]) // for unprivileged builds
	// setupContainersNodeStorage(pod, &pod.Spec.Containers[0]) // for privileged builds
	setupBlobCache(pod)
	return pod, nil
}
