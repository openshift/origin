package dockerhelper

import (
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

type Interface interface {
	Endpoint() string
	Info() (*types.Info, error)
	ServerVersion() (*types.Version, error)
	ContainerCreate(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, name string) (*container.ContainerCreateCreatedBody, error)
	ContainerList(options types.ContainerListOptions) ([]types.Container, error)
	ContainerInspect(container string) (*types.ContainerJSON, error)
	ContainerRemove(container string, options types.ContainerRemoveOptions) error
	ContainerLogs(container string, options types.ContainerLogsOptions, stdOut, stdErr io.Writer) error
	ContainerStart(container string) error
	ContainerStop(container string, timeout int) error
	ContainerWait(container string) (int, error)
	CopyToContainer(container string, dest string, src io.Reader, options types.CopyToContainerOptions) error
	CopyFromContainer(container string, src string) (io.ReadCloser, error)
	ContainerExecCreate(container string, config types.ExecConfig) (*types.IDResponse, error)
	ContainerExecAttach(execID string, stdIn io.Reader, stdOut, stdErr io.Writer) error
	ContainerExecInspect(execID string) (*types.ContainerExecInspect, error)
	ImageInspectWithRaw(imageID string, getSize bool) (*types.ImageInspect, []byte, error)
	ImagePull(ref string, options types.ImagePullOptions, writer io.Writer) error
}
