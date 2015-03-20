package strategy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// DockerBuildStrategy creates a Docker build using a Docker builder image.
type DockerBuildStrategy struct {
	Image string
	// Codec is the codec to use for encoding the output pod.
	// IMPORTANT: This may break backwards compatibility when
	// it changes.
	Codec runtime.Codec
}

// CreateBuildPod creates the pod to be used for the Docker build
// TODO: Make the Pod definition configurable
func (bs *DockerBuildStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	data, err := bs.Codec.Encode(build)
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
					Name:  "docker-build",
					Image: bs.Image,
					Env: []kapi.EnvVar{
						{Name: "BUILD", Value: string(data)},
					},
					// TODO: run unprivileged https://github.com/openshift/origin/issues/662
					Privileged: true,
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}

	pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent

	setupDockerSocket(pod)
	setupDockerConfig(pod)
	return pod, nil
}
