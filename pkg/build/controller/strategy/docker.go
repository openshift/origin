package strategy

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// DockerBuildStrategy creates a Docker build using a Docker builder image.
type DockerBuildStrategy struct {
	Image string
	// Codec is the codec to use for encoding the output pod.
	// IMPORTANT: This may break backwards compatibility when
	// it changes.
	Codec runtime.Codec
}

// CreateBuildPod creates the pod to be used for the Docker build
// TODO: Make the Pod definition configurable
func (bs *DockerBuildStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	data, err := bs.Codec.Encode(build)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the build: %v", err)
	}

	privileged := true
	strategy := build.Spec.Strategy.DockerStrategy

	containerEnv := []kapi.EnvVar{
		{Name: "BUILD", Value: string(data)},
		{Name: "BUILD_LOGLEVEL", Value: fmt.Sprintf("%d", cmdutil.GetLogLevel())},
	}

	addSourceEnvVars(build.Spec.Source, &containerEnv)

	if len(strategy.Env) > 0 {
		mergeTrustedEnvWithoutDuplicates(strategy.Env, &containerEnv)
	}

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:      buildutil.GetBuildPodName(build),
			Namespace: build.Namespace,
			Labels:    getPodLabels(build),
		},
		Spec: kapi.PodSpec{
			ServiceAccountName: build.Spec.ServiceAccount,
			Containers: []kapi.Container{
				{
					Name:  "docker-build",
					Image: bs.Image,
					Env:   containerEnv,
					Args:  []string{"--loglevel=" + getContainerVerbosity(containerEnv)},
					// TODO: run unprivileged https://github.com/openshift/origin/issues/662
					SecurityContext: &kapi.SecurityContext{
						Privileged: &privileged,
					},
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
	pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	pod.Spec.Containers[0].Resources = build.Spec.Resources
	if build.Spec.CompletionDeadlineSeconds != nil {
		pod.Spec.ActiveDeadlineSeconds = build.Spec.CompletionDeadlineSeconds
	}

	setupDockerSocket(pod)
	setupDockerSecrets(pod, build.Spec.Output.PushSecret, strategy.PullSecret)
	setupSourceSecrets(pod, build.Spec.Source.SourceSecret)
	return pod, nil
}
