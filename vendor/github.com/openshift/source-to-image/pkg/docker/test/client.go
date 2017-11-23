package test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"time"

	dockertypes "github.com/docker/engine-api/types"
	dockercontainer "github.com/docker/engine-api/types/container"
	dockernetwork "github.com/docker/engine-api/types/network"
	"golang.org/x/net/context"
)

// FakeConn fakes a net.Conn
type FakeConn struct {
}

// Read reads bytes
func (c FakeConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

// Write writes bytes
func (c FakeConn) Write(b []byte) (n int, err error) {
	return 0, nil
}

// Close closes the connection
func (c FakeConn) Close() error {
	return nil
}

// LocalAddr returns the local address
func (c FakeConn) LocalAddr() net.Addr {
	ip, _ := net.ResolveIPAddr("ip4", "127.0.0.1")
	return ip
}

// RemoteAddr returns the remote address
func (c FakeConn) RemoteAddr() net.Addr {
	ip, _ := net.ResolveIPAddr("ip4", "127.0.0.1")
	return ip
}

// SetDeadline sets the deadline
func (c FakeConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline sets the read deadline
func (c FakeConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline sets the write deadline
func (c FakeConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// FakeDockerClient provides a Fake client for Docker testing
type FakeDockerClient struct {
	CopyToContainerID      string
	CopyToContainerPath    string
	CopyToContainerContent io.Reader

	CopyFromContainerID   string
	CopyFromContainerPath string
	CopyFromContainerErr  error

	WaitContainerID             string
	WaitContainerResult         int
	WaitContainerErr            error
	WaitContainerErrInspectJSON dockertypes.ContainerJSON

	ContainerCommitID       string
	ContainerCommitOptions  dockertypes.ContainerCommitOptions
	ContainerCommitResponse dockertypes.ContainerCommitResponse
	ContainerCommitErr      error

	BuildImageOpts dockertypes.ImageBuildOptions
	BuildImageErr  error
	Images         map[string]dockertypes.ImageInspect

	Containers map[string]dockercontainer.Config

	PullFail error

	Calls []string
}

// NewFakeDockerClient returns a new FakeDockerClient
func NewFakeDockerClient() *FakeDockerClient {
	return &FakeDockerClient{
		Images:     make(map[string]dockertypes.ImageInspect),
		Containers: make(map[string]dockercontainer.Config),
		Calls:      make([]string, 0),
	}
}

// ImageInspectWithRaw returns the image information and its raw representation.
func (d *FakeDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string, getSize bool) (dockertypes.ImageInspect, []byte, error) {
	d.Calls = append(d.Calls, "inspect_image")

	if _, exists := d.Images[imageID]; exists {
		return d.Images[imageID], nil, nil
	}
	return dockertypes.ImageInspect{}, nil, fmt.Errorf("No such image: %q", imageID)
}

// CopyToContainer copies content into the container filesystem.
func (d *FakeDockerClient) CopyToContainer(ctx context.Context, container, path string, content io.Reader, opts dockertypes.CopyToContainerOptions) error {
	d.CopyToContainerID = container
	d.CopyToContainerPath = path
	d.CopyToContainerContent = content
	return nil
}

// CopyFromContainer gets the content from the container and returns it as a Reader
// to manipulate it in the host. It's up to the caller to close the reader.
func (d *FakeDockerClient) CopyFromContainer(ctx context.Context, container, srcPath string) (io.ReadCloser, dockertypes.ContainerPathStat, error) {
	d.CopyFromContainerID = container
	d.CopyFromContainerPath = srcPath
	return ioutil.NopCloser(bytes.NewReader([]byte(""))), dockertypes.ContainerPathStat{}, d.CopyFromContainerErr
}

// ContainerWait pauses execution until a container exits.
func (d *FakeDockerClient) ContainerWait(ctx context.Context, containerID string) (int, error) {
	d.WaitContainerID = containerID
	return d.WaitContainerResult, d.WaitContainerErr
}

// ContainerCommit applies changes into a container and creates a new tagged image.
func (d *FakeDockerClient) ContainerCommit(ctx context.Context, container string, options dockertypes.ContainerCommitOptions) (dockertypes.ContainerCommitResponse, error) {
	d.ContainerCommitID = container
	d.ContainerCommitOptions = options
	return d.ContainerCommitResponse, d.ContainerCommitErr
}

// ContainerAttach attaches a connection to a container in the server.
func (d *FakeDockerClient) ContainerAttach(ctx context.Context, container string, options dockertypes.ContainerAttachOptions) (dockertypes.HijackedResponse, error) {
	d.Calls = append(d.Calls, "attach")
	return dockertypes.HijackedResponse{Conn: FakeConn{}, Reader: bufio.NewReader(&bytes.Buffer{})}, nil
}

// ImageBuild sends request to the daemon to build images.
func (d *FakeDockerClient) ImageBuild(ctx context.Context, buildContext io.Reader, options dockertypes.ImageBuildOptions) (dockertypes.ImageBuildResponse, error) {
	d.BuildImageOpts = options
	return dockertypes.ImageBuildResponse{
		Body: ioutil.NopCloser(bytes.NewReader([]byte(""))),
	}, d.BuildImageErr
}

// ContainerCreate creates a new container based in the given configuration.
func (d *FakeDockerClient) ContainerCreate(ctx context.Context, config *dockercontainer.Config, hostConfig *dockercontainer.HostConfig, networkingConfig *dockernetwork.NetworkingConfig, containerName string) (dockertypes.ContainerCreateResponse, error) {
	d.Calls = append(d.Calls, "create")

	d.Containers[containerName] = *config
	return dockertypes.ContainerCreateResponse{}, nil
}

// ContainerInspect returns the container information.
func (d *FakeDockerClient) ContainerInspect(ctx context.Context, containerID string) (dockertypes.ContainerJSON, error) {
	d.Calls = append(d.Calls, "inspect_container")
	return d.WaitContainerErrInspectJSON, nil
}

// ContainerRemove kills and removes a container from the docker host.
func (d *FakeDockerClient) ContainerRemove(ctx context.Context, containerID string, options dockertypes.ContainerRemoveOptions) error {
	d.Calls = append(d.Calls, "remove")

	if _, exists := d.Containers[containerID]; exists {
		delete(d.Containers, containerID)
		return nil
	}
	return errors.New("container does not exist")
}

// ContainerKill terminates the container process but does not remove the container from the docker host.
func (d *FakeDockerClient) ContainerKill(ctx context.Context, containerID, signal string) error {
	return nil
}

// ContainerStart sends a request to the docker daemon to start a container.
func (d *FakeDockerClient) ContainerStart(ctx context.Context, containerID string) error {
	d.Calls = append(d.Calls, "start")
	return nil
}

// ImagePull requests the docker host to pull an image from a remote registry.
func (d *FakeDockerClient) ImagePull(ctx context.Context, ref string, options dockertypes.ImagePullOptions) (io.ReadCloser, error) {
	d.Calls = append(d.Calls, "pull")

	if d.PullFail != nil {
		return nil, d.PullFail
	}

	return ioutil.NopCloser(bytes.NewReader([]byte{})), nil
}

// ImageRemove removes an image from the docker host.
func (d *FakeDockerClient) ImageRemove(ctx context.Context, imageID string, options dockertypes.ImageRemoveOptions) ([]dockertypes.ImageDelete, error) {
	d.Calls = append(d.Calls, "remove_image")

	if _, exists := d.Images[imageID]; exists {
		delete(d.Images, imageID)
		return []dockertypes.ImageDelete{}, nil
	}
	return []dockertypes.ImageDelete{}, errors.New("image does not exist")
}

// ServerVersion returns information of the docker client and server host.
func (d *FakeDockerClient) ServerVersion(ctx context.Context) (dockertypes.Version, error) {
	return dockertypes.Version{}, nil
}
