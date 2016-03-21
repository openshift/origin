package test

import "github.com/fsouza/go-dockerclient"

// FakeDockerClient provides a Fake client for Docker testing
type FakeDockerClient struct {
	Image                *docker.Image
	InspectImageResult   []*docker.Image
	Container            *docker.Container
	RemoveImageErr       error
	InspectImageErr      []error
	PullImageErr         error
	CreateContainerErr   error
	AttachToContainerErr error
	StartContainerErr    error
	WaitContainerResult  int
	WaitContainerErr     error
	RemoveContainerErr   error
	CommitContainerErr   error
	CopyFromContainerErr error
	BuildImageErr        error

	RemoveImageName          string
	InspectImageName         []string
	PullImageOpts            docker.PullImageOptions
	PullImageAuth            docker.AuthConfiguration
	CreateContainerOpts      docker.CreateContainerOptions
	AttachToContainerOpts    []docker.AttachToContainerOptions
	StartContainerID         string
	StartContainerHostConfig *docker.HostConfig
	WaitContainerID          string
	RemoveContainerOpts      docker.RemoveContainerOptions
	CommitContainerOpts      docker.CommitContainerOptions
	CopyFromContainerOpts    docker.CopyFromContainerOptions
	BuildImageOpts           docker.BuildImageOptions
}

// RemoveImage removes an image from the fake client
func (d *FakeDockerClient) RemoveImage(name string) error {
	d.RemoveImageName = name
	return d.RemoveImageErr
}

func (d *FakeDockerClient) Ping() error {
	return nil
}

// InspectImage inspects the fake image
func (d *FakeDockerClient) InspectImage(name string) (*docker.Image, error) {
	d.InspectImageName = append(d.InspectImageName, name)
	i := len(d.InspectImageName) - 1
	var img *docker.Image
	if i >= len(d.InspectImageResult) {
		img = d.Image
	} else {
		img = d.InspectImageResult[i]
	}
	var err error
	if i >= len(d.InspectImageErr) {
		err = nil
	} else {
		err = d.InspectImageErr[i]
	}
	return img, err
}

// PullImage pulls the fake image
func (d *FakeDockerClient) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
	d.PullImageOpts = opts
	d.PullImageAuth = auth
	return d.PullImageErr
}

// CreateContainer creates a fake container
func (d *FakeDockerClient) CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error) {
	d.CreateContainerOpts = opts
	return d.Container, d.CreateContainerErr
}

// StartContainer starts the fake container
func (d *FakeDockerClient) StartContainer(id string, hostConfig *docker.HostConfig) error {
	d.StartContainerID = id
	d.StartContainerHostConfig = hostConfig
	return d.StartContainerErr
}

func (d *FakeDockerClient) UploadToContainer(id string, opts docker.UploadToContainerOptions) error {
	return nil
}

// WaitContainer waits for a fake container to finish
func (d *FakeDockerClient) WaitContainer(id string) (int, error) {
	d.WaitContainerID = id
	return d.WaitContainerResult, d.WaitContainerErr
}

// RemoveContainer removes the fake container
func (d *FakeDockerClient) RemoveContainer(opts docker.RemoveContainerOptions) error {
	d.RemoveContainerOpts = opts
	return d.RemoveContainerErr
}

// CommitContainer commits the fake container
func (d *FakeDockerClient) CommitContainer(opts docker.CommitContainerOptions) (*docker.Image, error) {
	d.CommitContainerOpts = opts
	return d.Image, d.CommitContainerErr
}

// CopyFromContainer copies from the fake container
func (d *FakeDockerClient) CopyFromContainer(opts docker.CopyFromContainerOptions) error {
	d.CopyFromContainerOpts = opts
	return d.CopyFromContainerErr
}

// BuildImage builds image
func (d *FakeDockerClient) BuildImage(opts docker.BuildImageOptions) error {
	d.BuildImageOpts = opts
	return d.BuildImageErr
}

func (d *FakeDockerClient) InspectContainer(id string) (*docker.Container, error) {
	return nil, d.BuildImageErr
}

func (d *FakeDockerClient) AttachToContainerNonBlocking(opts docker.AttachToContainerOptions) (docker.CloseWaiter, error) {
	return fakeCloseWait{}, nil
}

type fakeCloseWait struct{}

func (cw fakeCloseWait) Close() error {
	return nil
}

func (cw fakeCloseWait) Wait() error {
	return nil
}
