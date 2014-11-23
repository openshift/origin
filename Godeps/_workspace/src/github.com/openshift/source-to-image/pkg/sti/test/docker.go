package test

import (
	"sync"

	"github.com/openshift/source-to-image/pkg/sti/docker"
)

type FakeDocker struct {
	LocalRegistryImage           string
	LocalRegistryResult          bool
	LocalRegistryError           error
	RemoveContainerID            string
	RemoveContainerError         error
	DefaultUrlImage              string
	DefaultUrlResult             string
	DefaultUrlError              error
	RunContainerOpts             docker.RunContainerOptions
	RunContainerError            error
	RunContainerErrorBeforeStart bool
	RunContainerContainerID      string
	RunContainerCmd              []string
	GetImageIdImage              string
	GetImageIdResult             string
	GetImageIdError              error
	CommitContainerOpts          docker.CommitContainerOptions
	CommitContainerResult        string
	CommitContainerError         error
	RemoveImageName              string
	RemoveImageError             error

	mutex sync.Mutex
}

func (f *FakeDocker) IsImageInLocalRegistry(imageName string) (bool, error) {
	f.LocalRegistryImage = imageName
	return f.LocalRegistryResult, f.LocalRegistryError
}

func (f *FakeDocker) RemoveContainer(id string) error {
	f.RemoveContainerID = id
	return f.RemoveContainerError
}

func (f *FakeDocker) GetDefaultScriptsUrl(image string) (string, error) {
	f.DefaultUrlImage = image
	return f.DefaultUrlResult, f.DefaultUrlError
}

func (f *FakeDocker) RunContainer(opts docker.RunContainerOptions) error {
	f.RunContainerOpts = opts
	if f.RunContainerErrorBeforeStart {
		return f.RunContainerError
	}
	if opts.OnStart != nil {
		if err := opts.OnStart(); err != nil {
			return err
		}
	}
	if opts.PostExec != nil {
		opts.PostExec.PostExecute(f.RunContainerContainerID, append(f.RunContainerCmd, opts.Command))
	}
	return f.RunContainerError
}

func (f *FakeDocker) GetImageId(image string) (string, error) {
	f.GetImageIdImage = image
	return f.GetImageIdResult, f.GetImageIdError
}

func (f *FakeDocker) CommitContainer(opts docker.CommitContainerOptions) (string, error) {
	f.CommitContainerOpts = opts
	return f.CommitContainerResult, f.CommitContainerError
}

func (f *FakeDocker) RemoveImage(name string) error {
	f.RemoveImageName = name
	return f.RemoveImageError
}

func (f *FakeDocker) PullImage(imageName string) error {
	return nil
}
