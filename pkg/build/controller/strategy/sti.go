package strategy

import (
	"fmt"
	"strings"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/security/apis/security"
	securityinternalversion "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
)

// SourceBuildStrategy creates STI(source to image) builds
type SourceBuildStrategy struct {
	Image string
	// Codec is the codec to use for encoding the output pod.
	// IMPORTANT: This may break backwards compatibility when
	// it changes.
	Codec          runtime.Codec
	SecurityClient securityinternalversion.SecurityInterface
}

// DefaultDropCaps is the list of capabilities to drop if the current user cannot run as root
var DefaultDropCaps = []string{
	"KILL",
	"MKNOD",
	"SETGID",
	"SETUID",
}

// CreateBuildPod creates a pod that will execute the STI build
// TODO: Make the Pod definition configurable
func (bs *SourceBuildStrategy) CreateBuildPod(build *buildapi.Build) (*v1.Pod, error) {
	data, err := runtime.Encode(bs.Codec, build)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the Build %s/%s: %v", build.Namespace, build.Name, err)
	}

	containerEnv := []v1.EnvVar{
		{Name: "BUILD", Value: string(data)},
	}

	addSourceEnvVars(build.Spec.Source, &containerEnv)
	addOriginVersionVar(&containerEnv)

	strategy := build.Spec.Strategy.SourceStrategy
	if len(strategy.Env) > 0 {
		buildutil.MergeTrustedEnvWithoutDuplicates(buildutil.CopyApiEnvVarToV1EnvVar(strategy.Env), &containerEnv, true)
	}

	// check if can run container as root
	if !bs.canRunAsRoot(build) {
		// TODO: both AllowedUIDs and DropCapabilities should
		// be controlled via the SCC that's in effect for the build service account
		// For now, both are hard-coded based on whether the build service account can
		// run as root.
		containerEnv = append(containerEnv, v1.EnvVar{Name: buildapi.AllowedUIDs, Value: "1-"})
		containerEnv = append(containerEnv, v1.EnvVar{Name: buildapi.DropCapabilities, Value: strings.Join(DefaultDropCaps, ",")})
	}

	serviceAccount := build.Spec.ServiceAccount
	if len(serviceAccount) == 0 {
		serviceAccount = buildutil.BuilderServiceAccountName
	}

	privileged := true
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildapi.GetBuildPodName(build),
			Namespace: build.Namespace,
			Labels:    getPodLabels(build),
		},
		Spec: v1.PodSpec{
			ServiceAccountName: serviceAccount,
			Containers: []v1.Container{
				{
					Name:    stiBuild,
					Image:   bs.Image,
					Command: []string{"openshift-sti-build"},
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
					},
					ImagePullPolicy: v1.PullIfNotPresent,
					Resources:       buildutil.CopyApiResourcesToV1Resources(&build.Spec.Resources),
				},
			},
			Volumes: []v1.Volume{
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

	if build.Spec.Source.Git != nil || build.Spec.Source.Binary != nil {
		gitCloneContainer := v1.Container{
			Name:    GitCloneContainer,
			Image:   bs.Image,
			Command: []string{"openshift-git-clone"},
			Env:     copyEnvVarSlice(containerEnv),
			TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "buildworkdir",
					MountPath: buildutil.BuildWorkDirMount,
				},
			},
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources:       buildutil.CopyApiResourcesToV1Resources(&build.Spec.Resources),
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
			},
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources:       buildutil.CopyApiResourcesToV1Resources(&build.Spec.Resources),
		}
		setupDockerSecrets(pod, &extractImageContentContainer, build.Spec.Output.PushSecret, strategy.PullSecret, build.Spec.Source.Images)
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, extractImageContentContainer)
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers,
		v1.Container{
			Name:    "manage-dockerfile",
			Image:   bs.Image,
			Command: []string{"openshift-manage-dockerfile"},
			Env:     copyEnvVarSlice(containerEnv),
			TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "buildworkdir",
					MountPath: buildutil.BuildWorkDirMount,
				},
			},
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources:       buildutil.CopyApiResourcesToV1Resources(&build.Spec.Resources),
		},
	)

	if build.Spec.CompletionDeadlineSeconds != nil {
		pod.Spec.ActiveDeadlineSeconds = build.Spec.CompletionDeadlineSeconds
	}

	setOwnerReference(pod, build)
	setupDockerSocket(pod)
	setupCrioSocket(pod)
	setupDockerSecrets(pod, &pod.Spec.Containers[0], build.Spec.Output.PushSecret, strategy.PullSecret, build.Spec.Source.Images)
	// For any secrets the user wants to reference from their Assemble script or Dockerfile, mount those
	// secrets into the main container.  The main container includes logic to copy them from the mounted
	// location into the working directory.
	// TODO: consider moving this into the git-clone container and doing the secret copying there instead.
	setupInputSecrets(pod, &pod.Spec.Containers[0], build.Spec.Source.Secrets)
	return pod, nil
}

func (bs *SourceBuildStrategy) canRunAsRoot(build *buildapi.Build) bool {
	rootUser := int64(0)

	review, err := bs.SecurityClient.PodSecurityPolicySubjectReviews(build.Namespace).Create(
		&security.PodSecurityPolicySubjectReview{
			Spec: security.PodSecurityPolicySubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						ServiceAccountName: build.Spec.ServiceAccount,
						Containers: []kapi.Container{
							{
								Name:  "fake",
								Image: "fake",
								SecurityContext: &kapi.SecurityContext{
									RunAsUser: &rootUser,
								},
							},
						},
					},
				},
			},
		},
	)
	if err != nil {
		utilruntime.HandleError(err)
		return false
	}
	return review.Status.AllowedBy != nil
}
