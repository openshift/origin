package strategy

import (
	"errors"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/golang/glog"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// CustomBuildStrategy creates a build using a custom builder image.
type CustomBuildStrategy struct {
	// Codec is the codec to use for encoding the output pod.
	// IMPORTANT: This may break backwards compatibility when
	// it changes.
	Codec runtime.Codec
}

// CreateBuildPod creates the pod to be used for the Custom build
func (bs *CustomBuildStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	data, err := bs.Codec.Encode(build)
	if err != nil {
		return nil, err
	}

	strategy := build.Parameters.Strategy.CustomStrategy
	containerEnv := []kapi.EnvVar{
		{Name: "BUILD", Value: string(data)},
		{Name: "SOURCE_REPOSITORY", Value: build.Parameters.Source.Git.URI},
	}

	if strategy == nil || (strategy != nil && len(strategy.Image) == 0) {
		return nil, errors.New("CustomBuildStrategy cannot be executed without image")
	}

	if len(strategy.Env) > 0 {
		containerEnv = append(containerEnv, strategy.Env...)
	}

	if strategy.ExposeDockerSocket {
		glog.V(2).Infof("ExposeDockerSocket is enabled for %s build", build.PodName)
		containerEnv = append(containerEnv, kapi.EnvVar{"DOCKER_SOCKET", dockerSocketPath})
	}

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: build.PodName,
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  "custom-build",
					Image: strategy.Image,
					Env:   containerEnv,
					// TODO: run unprivileged https://github.com/openshift/origin/issues/662
					Privileged: true,
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}

	if err := setupBuildEnv(build, pod); err != nil {
		return nil, err
	}

	pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	if strategy.ExposeDockerSocket {
		setupDockerSocket(pod)
		setupDockerConfig(pod)
	}
	return pod, nil
}
