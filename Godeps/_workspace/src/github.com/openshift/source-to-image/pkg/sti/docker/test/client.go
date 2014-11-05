package test

import (
	"sync"
	"time"

	"github.com/fsouza/go-dockerclient"
)

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

	RemoveImageName          string
	InspectImageName         []string
	PullImageOpts            docker.PullImageOptions
	PullImageAuth            docker.AuthConfiguration
	CreateContainerOpts      docker.CreateContainerOptions
	AttachToContainerOpts    []docker.AttachToContainerOptions
	AttachToContainerSleep   time.Duration
	StartContainerId         string
	StartContainerHostConfig *docker.HostConfig
	WaitContainerId          string
	RemoveContainerOpts      docker.RemoveContainerOptions
	CommitContainerOpts      docker.CommitContainerOptions
	CopyFromContainerOpts    docker.CopyFromContainerOptions

	mutex sync.Mutex
}

func (d *FakeDockerClient) RemoveImage(name string) error {
	d.RemoveImageName = name
	return d.RemoveImageErr
}

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

func (d *FakeDockerClient) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
	d.PullImageOpts = opts
	d.PullImageAuth = auth
	return d.PullImageErr
}

func (d *FakeDockerClient) CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error) {
	d.CreateContainerOpts = opts
	return d.Container, d.CreateContainerErr
}

func (d *FakeDockerClient) AttachToContainer(opts docker.AttachToContainerOptions) error {
	d.mutex.Lock()
	d.AttachToContainerOpts = append(d.AttachToContainerOpts, opts)
	if opts.Success != nil {
		opts.Success <- struct{}{}
		<-opts.Success
	}
	d.mutex.Unlock()
	if d.AttachToContainerSleep != 0 {
		time.Sleep(d.AttachToContainerSleep)
	}
	return d.AttachToContainerErr
}

func (d *FakeDockerClient) StartContainer(id string, hostConfig *docker.HostConfig) error {
	d.StartContainerId = id
	d.StartContainerHostConfig = hostConfig
	return d.StartContainerErr
}

func (d *FakeDockerClient) WaitContainer(id string) (int, error) {
	d.WaitContainerId = id
	return d.WaitContainerResult, d.WaitContainerErr
}

func (d *FakeDockerClient) RemoveContainer(opts docker.RemoveContainerOptions) error {
	d.RemoveContainerOpts = opts
	return d.RemoveContainerErr
}

func (d *FakeDockerClient) CommitContainer(opts docker.CommitContainerOptions) (*docker.Image, error) {
	d.CommitContainerOpts = opts
	return d.Image, d.CommitContainerErr
}

func (d *FakeDockerClient) CopyFromContainer(opts docker.CopyFromContainerOptions) error {
	d.CopyFromContainerOpts = opts
	return d.CopyFromContainerErr
}
