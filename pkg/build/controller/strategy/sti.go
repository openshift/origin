package strategy

import (
	"encoding/json"
	"io/ioutil"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// STIBuildStrategy creates STI(source to image) builds
type STIBuildStrategy struct {
	BuilderImage         string
	TempDirectoryCreator TempDirectoryCreator
	UseLocalImages       bool
}

type TempDirectoryCreator interface {
	CreateTempDirectory() (string, error)
}

type tempDirectoryCreator struct{}

func (tc *tempDirectoryCreator) CreateTempDirectory() (string, error) {
	return ioutil.TempDir("", "stibuild")
}

var STITempDirectoryCreator = &tempDirectoryCreator{}

// CreateBuildPod creates a pod that will execute the STI build
// TODO: Make the Pod definition configurable
func (bs *STIBuildStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	buildJson, err := json.Marshal(build)
	if err != nil {
		return nil, err
	}
	pod := &kapi.Pod{
		TypeMeta: kapi.TypeMeta{
			ID: build.PodID,
		},
		DesiredState: kapi.PodState{
			Manifest: kapi.ContainerManifest{
				Version: "v1beta1",
				Containers: []kapi.Container{
					{
						Name:  "sti-build",
						Image: bs.BuilderImage,
						Env: []kapi.EnvVar{
							{Name: "SOURCE_URI", Value: build.Parameters.Source.Git.URI},
							{Name: "SOURCE_REF", Value: build.Parameters.Source.Git.Ref},
							{Name: "SOURCE_ID", Value: build.Parameters.Revision.Git.Commit},
							{Name: "BUILDER_IMAGE", Value: build.Parameters.Strategy.STIStrategy.BuilderImage},
							{Name: "BUILD_TAG", Value: build.Parameters.Output.ImageTag},
							{Name: "REGISTRY", Value: build.Parameters.Output.Registry},
							{Name: "BUILD", Value: string(buildJson)},
						},
					},
				},
				RestartPolicy: kapi.RestartPolicy{
					Never: &kapi.RestartPolicyNever{},
				},
			},
		},
	}

	if bs.UseLocalImages {
		pod.DesiredState.Manifest.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	}

	if err := bs.setupTempVolume(pod); err != nil {
		return nil, err
	}

	setupDockerSocket(pod)
	setupDockerConfig(pod)
	return pod, nil
}

func (bs *STIBuildStrategy) setupTempVolume(pod *kapi.Pod) error {
	tempDir, err := bs.TempDirectoryCreator.CreateTempDirectory()
	if err != nil {
		return err
	}
	tmpVolume := kapi.Volume{
		Name: "tmp",
		Source: &kapi.VolumeSource{
			HostDir: &kapi.HostDir{
				Path: tempDir,
			},
		},
	}
	tmpMount := kapi.VolumeMount{Name: "tmp", ReadOnly: false, MountPath: tempDir}
	pod.DesiredState.Manifest.Volumes = append(pod.DesiredState.Manifest.Volumes, tmpVolume)
	pod.DesiredState.Manifest.Containers[0].VolumeMounts =
		append(pod.DesiredState.Manifest.Containers[0].VolumeMounts, tmpMount)
	pod.DesiredState.Manifest.Containers[0].Env =
		append(pod.DesiredState.Manifest.Containers[0].Env, kapi.EnvVar{
			Name: "TEMP_DIR", Value: tempDir})

	return nil
}
