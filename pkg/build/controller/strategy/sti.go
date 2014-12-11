package strategy

import (
	"encoding/json"
	"io/ioutil"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// STIBuildStrategy creates STI(source to image) builds
type STIBuildStrategy struct {
	Image                string
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
	buildJSON, err := json.Marshal(build)
	if err != nil {
		return nil, err
	}
	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: build.PodName,
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  "sti-build",
					Image: bs.Image,
					Env: []kapi.EnvVar{
						{Name: "BUILD", Value: string(buildJSON)},
					},
				},
			},
			RestartPolicy: kapi.RestartPolicy{
				Never: &kapi.RestartPolicyNever{},
			},
		},
	}

	if bs.UseLocalImages {
		pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	}

	setupDockerSocket(pod)
	setupDockerConfig(pod)
	return pod, nil
}
