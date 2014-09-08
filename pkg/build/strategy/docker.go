package strategy

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

// DockerBuildStrategy creates Docker build using a docker builder image
// useHostDocker determines whether the minion Docker daemon is used for the build
// or a separate Docker daemon is run inside the container
type DockerBuildStrategy struct {
	dockerBuilderImage string
	useHostDocker      bool
}

// NewDockerBuildStrategy creates a new DockerBuildStrategy
func NewDockerBuildStrategy(dockerBuilderImage string, useHostDocker bool) *DockerBuildStrategy {
	return &DockerBuildStrategy{dockerBuilderImage, useHostDocker}
}

// CreateBuildPod creates the pod to be used for the Docker build
// TODO: Make the Pod definition configurable
func (bs *DockerBuildStrategy) CreateBuildPod(build *buildapi.Build, dockerRegistry string) *api.Pod {
	pod := &api.Pod{
		JSONBase: api.JSONBase{
			ID: build.PodID,
		},
		DesiredState: api.PodState{
			Manifest: api.ContainerManifest{
				Version: "v1beta1",
				Containers: []api.Container{
					{
						Name:          "docker-build",
						Image:         bs.dockerBuilderImage,
						RestartPolicy: "runOnce",
						Env: []api.EnvVar{
							{Name: "BUILD_TAG", Value: build.Input.ImageTag},
							{Name: "DOCKER_CONTEXT_URL", Value: build.Input.SourceURI},
							{Name: "DOCKER_REGISTRY", Value: dockerRegistry},
						},
					},
				},
			},
		},
	}

	setupDockerSocket(bs.useHostDocker, pod)
	return pod
}
