package strategy

import (
	"encoding/json"
	"errors"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// CustomBuildStrategy creates a build using a custom builder image.
type CustomBuildStrategy struct {
	UseLocalImages bool
}

// CreateBuildPod creates the pod to be used for the Custom build
func (bs *CustomBuildStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	buildJSON, err := json.Marshal(build)
	if err != nil {
		return nil, err
	}

	strategy := build.Parameters.Strategy.CustomStrategy
	containerEnv := []kapi.EnvVar{
		{Name: "BUILD", Value: string(buildJSON)},
		{Name: "SOURCE_REPOSITORY", Value: build.Parameters.Source.Git.URI},
	}

	if strategy == nil || (strategy != nil && len(strategy.BuilderImage) == 0) {
		return nil, errors.New("CustomBuildStrategy cannot be executed without builderImage")
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
					Image: strategy.BuilderImage,
					Env:   containerEnv,
				},
			},
			RestartPolicy: kapi.RestartPolicy{
				Never: &kapi.RestartPolicyNever{},
			},
		},
	}

	setupBuildEnv(build, pod)

	if bs.UseLocalImages {
		pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	}

	if strategy.ExposeDockerSocket {
		setupDockerSocket(pod)
		setupDockerConfig(pod)
	}
	return pod, nil
}
