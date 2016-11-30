package strategy

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// DockerBuildStrategy creates a Docker build using a Docker builder image.
type DockerBuildStrategy struct {
	Image                    string
	GitCloneImage            string
	ManageDockerfileImage    string
	ExtractImageContentImage string
	// Codec is the codec to use for encoding the output pod.
	// IMPORTANT: This may break backwards compatibility when
	// it changes.
	Codec runtime.Codec
}

// CreateBuildPod creates the pod to be used for the Docker build
// TODO: Make the Pod definition configurable
func (bs *DockerBuildStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	data, err := runtime.Encode(bs.Codec, build)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the build: %v", err)
	}

	privileged := true
	strategy := build.Spec.Strategy.DockerStrategy

	containerEnv := []kapi.EnvVar{
		{Name: "BUILD", Value: string(data)},
	}

	addSourceEnvVars(build.Spec.Source, &containerEnv)
	addOriginVersionVar(&containerEnv)

	if len(strategy.Env) > 0 {
		mergeTrustedEnvWithoutDuplicates(strategy.Env, &containerEnv)
	}

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:      buildapi.GetBuildPodName(build),
			Namespace: build.Namespace,
			Labels:    getPodLabels(build),
		},
		Spec: kapi.PodSpec{
			ServiceAccountName: build.Spec.ServiceAccount,
			InitContainers: []kapi.Container{
				{
					Name:  "manage-dockerfile",
					Image: bs.ManageDockerfileImage,
					Env:   containerEnv,
					Args:  []string{},
					VolumeMounts: []kapi.VolumeMount{
						{
							Name:      "gitsource",
							MountPath: "/tmp/gitSource",
						},
					},
					ImagePullPolicy: kapi.PullIfNotPresent,
					Resources:       build.Spec.Resources,
				},
			},
			Containers: []kapi.Container{
				{
					Name:  "docker-build",
					Image: bs.Image,
					Env:   containerEnv,
					Args:  []string{},
					// TODO: run unprivileged https://github.com/openshift/origin/issues/662
					SecurityContext: &kapi.SecurityContext{
						Privileged: &privileged,
					},
					VolumeMounts: []kapi.VolumeMount{
						{
							Name:      "gitsource",
							MountPath: "/tmp/gitSource",
						},
					},
					ImagePullPolicy: kapi.PullIfNotPresent,
					Resources:       build.Spec.Resources,
				},
			},
			Volumes: []kapi.Volume{
				{
					Name: "gitsource",
					VolumeSource: kapi.VolumeSource{
						EmptyDir: &kapi.EmptyDirVolumeSource{},
					},
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
			NodeSelector:  build.Spec.NodeSelector,
		},
	}
	// can't conditionalize the manage-dockerfile init container because we don't
	// know until we've cloned, whether or not we've got a dockerfile to manage
	// (also if it's a docker type build, we should always have a dockerfile to manage)
	if build.Spec.Source.Git != nil || build.Spec.Source.Binary != nil {
		gitCloneContainer := kapi.Container{
			Name:  "git-clone",
			Image: bs.GitCloneImage,
			Env:   containerEnv,
			Args:  []string{},
			VolumeMounts: []kapi.VolumeMount{
				{
					Name:      "gitsource",
					MountPath: "/tmp/gitSource",
				},
			},
			ImagePullPolicy: kapi.PullIfNotPresent,
			Resources:       build.Spec.Resources,
		}
		if build.Spec.Source.Binary != nil {
			gitCloneContainer.Stdin = true
			gitCloneContainer.StdinOnce = true
		}
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, gitCloneContainer)
	}
	if len(build.Spec.Source.Images) > 0 {
		extractImageContentContainer := kapi.Container{
			Name:  "extract-image-content",
			Image: bs.ExtractImageContentImage,
			Env:   containerEnv,
			Args:  []string{},
			// TODO: run unprivileged https://github.com/openshift/origin/issues/662
			SecurityContext: &kapi.SecurityContext{
				Privileged: &privileged,
			},
			VolumeMounts: []kapi.VolumeMount{
				{
					Name:      "gitsource",
					MountPath: "/tmp/gitSource",
				},
			},
			ImagePullPolicy: kapi.PullIfNotPresent,
			Resources:       build.Spec.Resources,
		}
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, extractImageContentContainer)
	}

	if build.Spec.CompletionDeadlineSeconds != nil {
		pod.Spec.ActiveDeadlineSeconds = build.Spec.CompletionDeadlineSeconds
	}

	setupDockerSocket(pod)
	setupDockerSecrets(pod, build.Spec.Output.PushSecret, strategy.PullSecret, build.Spec.Source.Images)
	setupSourceSecrets(pod, build.Spec.Source.SourceSecret)
	setupSecrets(pod, build.Spec.Source.Secrets)

	return pod, nil
}
