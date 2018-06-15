package dockergc

import (
	"context"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockerapi "github.com/docker/docker/client"
)

type dockerClient struct {
	// timeout is the timeout of short running docker operations.
	timeout time.Duration
	// docker API client
	client *dockerapi.Client
}

func newDockerClient(timeout time.Duration) (*dockerClient, error) {
	client, err := dockerapi.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return &dockerClient{
		client:  client,
		timeout: timeout,
	}, nil
}

func clientErr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func (c *dockerClient) getTimeoutContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.timeout)
}

func (c *dockerClient) Info() (*dockertypes.Info, error) {
	ctx, cancel := c.getTimeoutContext()
	defer cancel()
	info, err := c.client.Info(ctx)
	if err := clientErr(ctx, err); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *dockerClient) ContainerList(options dockertypes.ContainerListOptions) ([]dockertypes.Container, error) {
	ctx, cancel := c.getTimeoutContext()
	defer cancel()
	containers, err := c.client.ContainerList(ctx, options)
	if err := clientErr(ctx, err); err != nil {
		return nil, err
	}
	return containers, nil
}

func (c *dockerClient) ContainerRemove(id string, opts dockertypes.ContainerRemoveOptions) error {
	ctx, cancel := c.getTimeoutContext()
	defer cancel()
	err := c.client.ContainerRemove(ctx, id, opts)
	return clientErr(ctx, err)
}

func (c *dockerClient) ImageList(opts dockertypes.ImageListOptions) ([]dockertypes.ImageSummary, error) {
	ctx, cancel := c.getTimeoutContext()
	defer cancel()
	images, err := c.client.ImageList(ctx, opts)
	if err := clientErr(ctx, err); err != nil {
		return nil, err
	}
	return images, nil
}

func (c *dockerClient) ImageRemove(image string, opts dockertypes.ImageRemoveOptions) error {
	ctx, cancel := c.getTimeoutContext()
	defer cancel()
	_, err := c.client.ImageRemove(ctx, image, opts)
	return clientErr(ctx, err)
}
