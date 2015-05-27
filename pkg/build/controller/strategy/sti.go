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

// SourceBuildStrategy creates STI(source to image) builds
type SourceBuildStrategy struct {
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
func (bs *SourceBuildStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	data, err := bs.Codec.Encode(build)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the Build %s/%s: %v", build.Namespace, build.Name, err)
	}

	containerEnv := []kapi.EnvVar{
		{Name: "BUILD", Value: string(data)},
		{Name: "SOURCE_REPOSITORY", Value: build.Parameters.Source.Git.URI},
		{Name: "BUILD_LOGLEVEL", Value: fmt.Sprintf("%d", cmdutil.GetLogLevel())},
	}

	strategy := build.Parameters.Strategy.SourceStrategy
	if len(strategy.Env) > 0 {
		mergeTrustedEnvWithoutDuplicates(strategy.Env, &containerEnv)
	}

	privileged := true
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
					SecurityContext: &kapi.SecurityContext{
						Privileged: &privileged,
					},
					Args: []string{"--loglevel=" + getContainerVerbosity(containerEnv)},
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
	pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	pod.Spec.Containers[0].Resources = build.Parameters.Resources

	setupDockerSocket(pod)
	setupDockerSecrets(pod, build.Parameters.Output.PushSecret, strategy.PullSecret)
	setupSourceSecrets(pod, build.Parameters.Source.SourceSecret)
	return pod, nil
}
