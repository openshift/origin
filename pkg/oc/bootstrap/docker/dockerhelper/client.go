package dockerhelper

import (
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const (
	// defaultDockerOpTimeout is the default timeout of short running docker operations.
	defaultDockerOpTimeout = 10 * time.Minute
)

// NewClient creates an instance of the client Interface, given a docker engine
// client
func NewClient(endpoint string, client *client.Client) Interface {
	return &dockerClient{
		endpoint: endpoint,
		client:   client,
	}
}

type dockerClient struct {
	endpoint string
	client   *client.Client
}

type operationTimeout struct {
	err error
}

func (e operationTimeout) Error() string {
	return fmt.Sprintf("operation timeout: %v", e.err)
}

func (c *dockerClient) Endpoint() string {
	return c.endpoint
}

func (c *dockerClient) ServerVersion() (*types.Version, error) {
	ctx, cancel := defaultContext()
	defer cancel()
	version, err := c.client.ServerVersion(ctx)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	return &version, err
}

func (c *dockerClient) Info() (*types.Info, error) {
	ctx, cancel := defaultContext()
	defer cancel()
	info, err := c.client.Info(ctx)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	return &info, err
}

func (c *dockerClient) ContainerList(options types.ContainerListOptions) ([]types.Container, error) {
	ctx, cancel := defaultContext()
	defer cancel()
	containers, err := c.client.ContainerList(ctx, options)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	return containers, err
}

func (c *dockerClient) ContainerCreate(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, name string) (*container.ContainerCreateCreatedBody, error) {
	ctx, cancel := defaultContext()
	defer cancel()
	response, err := c.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, name)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	return &response, err
}

func (c *dockerClient) ContainerInspect(container string) (*types.ContainerJSON, error) {
	ctx, cancel := defaultContext()
	defer cancel()
	response, err := c.client.ContainerInspect(ctx, container)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	return &response, err
}

func (c *dockerClient) ContainerRemove(container string, options types.ContainerRemoveOptions) error {
	ctx, cancel := defaultContext()
	defer cancel()
	err := c.client.ContainerRemove(ctx, container, options)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	return err
}

func (c *dockerClient) ContainerLogs(container string, options types.ContainerLogsOptions, stdOut, stdErr io.Writer) error {
	ctx, cancel := defaultContext()
	defer cancel()
	response, err := c.client.ContainerLogs(ctx, container, options)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return err
	}
	defer response.Close()
	return redirectResponseToOutputStream(stdOut, stdErr, response)
}

func (c *dockerClient) ContainerStart(container string) error {
	ctx, cancel := defaultContext()
	defer cancel()
	err := c.client.ContainerStart(ctx, container, types.ContainerStartOptions{})
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	return err
}

func (c *dockerClient) ContainerStop(container string, timeout int) error {
	ctx, cancel := defaultContext()
	defer cancel()
	var t *time.Duration
	if timeout > 0 {
		duration := time.Duration(timeout) * time.Second
		t = &duration
	}
	err := c.client.ContainerStop(ctx, container, t)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	return err
}

func (c *dockerClient) ContainerWait(containerID string) (int, error) {
	ctx, cancel := defaultContext()
	defer cancel()
	rcCh, errCh := c.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return 0, ctxErr
	}
	select {
	case err := <-errCh:
		return 0, err
	case rc := <-rcCh:
		return int(rc.StatusCode), nil
	}
}

func (c *dockerClient) CopyToContainer(container string, dest string, src io.Reader, options types.CopyToContainerOptions) error {
	return c.client.CopyToContainer(context.Background(), container, dest, src, options)
}

func (c *dockerClient) CopyFromContainer(container string, src string) (io.ReadCloser, error) {
	response, _, err := c.client.CopyFromContainer(context.Background(), container, src)
	return response, err
}

func (c *dockerClient) ContainerExecCreate(container string, config types.ExecConfig) (*types.IDResponse, error) {
	ctx, cancel := defaultContext()
	defer cancel()
	response, err := c.client.ContainerExecCreate(ctx, container, config)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	return &response, err
}

func (c *dockerClient) ContainerExecAttach(execID string, stdIn io.Reader, stdOut, stdErr io.Writer) error {
	ctx, cancel := defaultContext()
	defer cancel()
	response, err := c.client.ContainerExecAttach(ctx, execID, types.ExecConfig{
		AttachStdin:  stdIn != nil,
		AttachStdout: true,
		AttachStderr: true,
	})
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return err
	}
	defer response.Close()
	return holdHijackedConnection(stdIn, stdOut, stdErr, response)
}

func (c *dockerClient) ContainerExecInspect(execID string) (*types.ContainerExecInspect, error) {
	ctx, cancel := defaultContext()
	defer cancel()
	response, err := c.client.ContainerExecInspect(ctx, execID)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	return &response, err
}

func (c *dockerClient) ImageInspectWithRaw(imageID string, _ bool) (*types.ImageInspect, []byte, error) {
	ctx, cancel := defaultContext()
	defer cancel()
	image, raw, err := c.client.ImageInspectWithRaw(ctx, imageID)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, nil, ctxErr
	}
	return &image, raw, err
}

func (c *dockerClient) ImagePull(ref string, options types.ImagePullOptions, writer io.Writer) error {
	ctx := context.Background()
	response, err := c.client.ImagePull(ctx, ref, options)
	if err != nil {
		return err
	}
	defer response.Close()
	_, err = io.Copy(writer, response)
	return err
}

func defaultContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultDockerOpTimeout)
}

func contextError(ctx context.Context) error {
	if ctx.Err() == context.DeadlineExceeded {
		return operationTimeout{err: ctx.Err()}
	}
	return ctx.Err()
}

// redirectResponseToOutputStream redirect the response stream to stdout and stderr.
func redirectResponseToOutputStream(outputStream, errorStream io.Writer, resp io.Reader) error {
	if outputStream == nil {
		outputStream = ioutil.Discard
	}
	if errorStream == nil {
		errorStream = ioutil.Discard
	}
	_, err := stdcopy.StdCopy(outputStream, errorStream, resp)
	return err
}

// holdHijackedConnection holds the HijackedResponse, redirects the inputStream to the connection, and redirects the response
// stream to stdout and stderr.
func holdHijackedConnection(inputStream io.Reader, outputStream, errorStream io.Writer, resp types.HijackedResponse) error {
	receiveStdout := make(chan error)
	if outputStream != nil || errorStream != nil {
		go func() {
			receiveStdout <- redirectResponseToOutputStream(outputStream, errorStream, resp.Reader)
		}()
	}

	sendStdin := make(chan error, 1)
	go func() {
		defer resp.CloseWrite()
		if inputStream != nil {
			_, err := io.Copy(resp.Conn, inputStream)
			sendStdin <- err
			return
		}
		sendStdin <- nil
	}()

	select {
	case err := <-receiveStdout:
		return err
	case sendErr := <-sendStdin:
		if sendErr != nil {
			return sendErr
		}
		if outputStream != nil || errorStream != nil {
			return <-receiveStdout
		}
	}
	return nil
}
