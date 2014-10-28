package strategy

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

// DockerBuildStrategy creates Docker build using a docker builder image
type DockerBuildStrategy struct {
	dockerBuilderImage string
	useLocalImage      bool
}

// NewDockerBuildStrategy creates a new DockerBuildStrategy
func NewDockerBuildStrategy(dockerBuilderImage string, useLocalImage bool) *DockerBuildStrategy {
	return &DockerBuildStrategy{dockerBuilderImage, useLocalImage}
}

// CreateBuildPod creates the pod to be used for the Docker build
// TODO: Make the Pod definition configurable
func (bs *DockerBuildStrategy) CreateBuildPod(build *buildapi.Build) (*api.Pod, error) {
	contextDir := ""
	if build.Input.DockerInput != nil {
		contextDir = build.Input.DockerInput.ContextDir
	}

	pod := &api.Pod{
		TypeMeta: api.TypeMeta{
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
							{Name: "SOURCE_URI", Value: build.Input.SourceURI},
							{Name: "SOURCE_REF", Value: build.Input.SourceRef},
							{Name: "REGISTRY", Value: build.Input.Registry},
							{Name: "CONTEXT_DIR", Value: contextDir},
						},
					},
				},
				RestartPolicy: api.RestartPolicy{
					Never: &api.RestartPolicyNever{},
				},
			},
		},
	}
	if bs.useLocalImage {
		pod.DesiredState.Manifest.Containers[0].ImagePullPolicy = api.PullIfNotPresent
	}

	setupDockerSocket(pod)
	setupDockerConfig(pod)
	return pod, nil
}
