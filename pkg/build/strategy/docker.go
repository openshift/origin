package strategy

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

// DockerBuildStrategy creates Docker build using a docker builder image
type DockerBuildStrategy struct {
	dockerBuilderImage string
}

// NewDockerBuildStrategy creates a new DockerBuildStrategy
func NewDockerBuildStrategy(dockerBuilderImage string) *DockerBuildStrategy {
	return &DockerBuildStrategy{dockerBuilderImage}
}

// CreateBuildPod creates the pod to be used for the Docker build
// TODO: Make the Pod definition configurable
func (bs *DockerBuildStrategy) CreateBuildPod(build *buildapi.Build) (*api.Pod, error) {
	pod := &api.Pod{
		JSONBase: api.JSONBase{
			ID: build.PodID,
		},
		DesiredState: api.PodState{
			Manifest: api.ContainerManifest{
				Version: "v1beta1",
				Containers: []api.Container{
					{
						Name:  "docker-build",
						Image: bs.dockerBuilderImage,
						Env: []api.EnvVar{
							{Name: "BUILD_TAG", Value: build.Input.ImageTag},
							{Name: "DOCKER_CONTEXT_URL", Value: build.Input.SourceURI},
							{Name: "DOCKER_REGISTRY", Value: build.Input.Registry},
						},
					},
				},
				RestartPolicy: api.RestartPolicy{
					Never: &api.RestartPolicyNever{},
				},
			},
		},
	}

	setupDockerSocket(pod)
	setupDockerConfig(pod)
	return pod, nil
}
