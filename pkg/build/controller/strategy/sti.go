package strategy

import (
	"fmt"
	"io/ioutil"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// STIBuildStrategy creates STI(source to image) builds
type STIBuildStrategy struct {
	Image                string
	TempDirectoryCreator TempDirectoryCreator
	// Codec is the codec to use for encoding the output pod.
	// IMPORTANT: This may break backwards compatibility when
	// it changes.
	Codec runtime.Codec
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
	data, err := bs.Codec.Encode(build)
	if err != nil {
		return nil, err
	}

	containerEnv := []kapi.EnvVar{
		{Name: "BUILD", Value: string(data)},
		{Name: "SOURCE_REPOSITORY", Value: build.Parameters.Source.Git.URI},
		{Name: "BUILD_LOGLEVEL", Value: fmt.Sprintf("%d", cmdutil.GetLogLevel())},
	}

	strategy := build.Parameters.Strategy.STIStrategy
	if len(strategy.Env) > 0 {
		mergeEnvWithoutDuplicates(strategy.Env, &containerEnv)
	}

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:      buildutil.GetBuildPodName(build),
			Namespace: build.Namespace,
			Labels:    getPodLabels(build),
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  "sti-build",
					Image: bs.Image,
					Env:   containerEnv,
					// TODO: run unprivileged https://github.com/openshift/origin/issues/662
					Privileged: true,
					Command:    []string{"--loglevel=" + getContainerVerbosity(containerEnv)},
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
	pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent

	setupDockerSocket(pod)
	setupDockerSecrets(pod, build.Parameters.Output.PushSecretName)
	return pod, nil
}
