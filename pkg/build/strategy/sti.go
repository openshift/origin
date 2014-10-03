package strategy

import (
	"io/ioutil"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

// STIBuildStrategy creates STI(source to image) builds
type STIBuildStrategy struct {
	stiBuilderImage      string
	tempDirectoryCreator TempDirectoryCreator
}

type TempDirectoryCreator interface {
	CreateTempDirectory() (string, error)
}

type tempDirectoryCreator struct{}

func (tc *tempDirectoryCreator) CreateTempDirectory() (string, error) {
	return ioutil.TempDir("", "stibuild")
}

var STITempDirectoryCreator = &tempDirectoryCreator{}

// NewSTIBuildStrategy creates a new STIBuildStrategy with the given
// builder image
func NewSTIBuildStrategy(stiBuilderImage string, tc TempDirectoryCreator) *STIBuildStrategy {
	return &STIBuildStrategy{stiBuilderImage, tc}
}

// CreateBuildPod creates a pod that will execute the STI build
// TODO: Make the Pod definition configurable
func (bs *STIBuildStrategy) CreateBuildPod(build *buildapi.Build) (*api.Pod, error) {
	pod := &api.Pod{
		JSONBase: api.JSONBase{
			ID: build.PodID,
		},
		DesiredState: api.PodState{
			Manifest: api.ContainerManifest{
				Version: "v1beta1",
				Containers: []api.Container{
					{
						Name:  "sti-build",
						Image: bs.stiBuilderImage,
						Env: []api.EnvVar{
							{Name: "BUILD_TAG", Value: build.Input.ImageTag},
							{Name: "DOCKER_REGISTRY", Value: build.Input.Registry},
							{Name: "SOURCE_URI", Value: build.Input.SourceURI},
							{Name: "SOURCE_REF", Value: build.Input.SourceRef},
							{Name: "BUILDER_IMAGE", Value: build.Input.BuilderImage},
						},
					},
				},
				RestartPolicy: api.RestartPolicy{
					Never: &api.RestartPolicyNever{},
				},
			},
		},
	}

	if err := bs.setupTempVolume(pod); err != nil {
		return nil, err
	}

	setupDockerSocket(pod)
	setupDockerConfig(pod)
	return pod, nil
}

func (bs *STIBuildStrategy) setupTempVolume(pod *api.Pod) error {
	tempDir, err := bs.tempDirectoryCreator.CreateTempDirectory()
	if err != nil {
		return err
	}
	tmpVolume := api.Volume{
		Name: "tmp",
		Source: &api.VolumeSource{
			HostDir: &api.HostDir{
				Path: tempDir,
			},
		},
	}
	tmpMount := api.VolumeMount{Name: "tmp", ReadOnly: false, MountPath: tempDir}
	pod.DesiredState.Manifest.Volumes = append(pod.DesiredState.Manifest.Volumes, tmpVolume)
	pod.DesiredState.Manifest.Containers[0].VolumeMounts =
		append(pod.DesiredState.Manifest.Containers[0].VolumeMounts, tmpMount)
	pod.DesiredState.Manifest.Containers[0].Env =
		append(pod.DesiredState.Manifest.Containers[0].Env, api.EnvVar{
			Name: "TEMP_DIR", Value: tempDir})

	return nil
}
