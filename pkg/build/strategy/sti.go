package strategy

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

// STIBuildStrategy creates STI(source to image) builds
// useHostDocker determines whether the minion Docker daemon is used for the build
// or a separate Docker daemon is run inside the container
type STIBuildStrategy struct {
	stiBuilderImage string
	useHostDocker   bool
}

// NewSTIBuildStrategy creates a new STIBuildStrategy with the given
// builder image
func NewSTIBuildStrategy(stiBuilderImage string, useHostDocker bool) *STIBuildStrategy {
	return &STIBuildStrategy{stiBuilderImage, useHostDocker}
}

// CreateBuildPod creates a pod that will execute the STI build
// TODO: Make the Pod definition configurable
func (bs *STIBuildStrategy) CreateBuildPod(build *buildapi.Build, dockerRegistry string) *api.Pod {
	pod := &api.Pod{
		JSONBase: api.JSONBase{
			ID: build.PodID,
		},
		DesiredState: api.PodState{
			Manifest: api.ContainerManifest{
				Version: "v1beta1",
				Containers: []api.Container{
					{
						Name:          "sti-build",
						Image:         bs.stiBuilderImage,
						RestartPolicy: "runOnce",
						Env: []api.EnvVar{
							{Name: "BUILD_TAG", Value: build.Input.ImageTag},
							{Name: "DOCKER_REGISTRY", Value: dockerRegistry},
							{Name: "SOURCE_REF", Value: build.Input.SourceURI},
							{Name: "BUILDER_IMAGE", Value: build.Input.BuilderImage},
						},
					},
				},
			},
		},
	}
	setupDockerSocket(bs.useHostDocker, pod)
	return pod
}
