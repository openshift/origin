package kubelet

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/fsouza/go-dockerclient"
)

type SecurityContextProvider interface {
	SecureContainer(pod *api.BoundPod, container *api.Container, createOptions *docker.CreateContainerOptions) error
	SecureHostConfig(pod *api.BoundPod, container *api.Container, hostConfig *docker.HostConfig) error
}
