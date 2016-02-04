package builder

import (
	"testing"

	"github.com/fsouza/go-dockerclient"
)

type FakeDocker struct {
	pushImageFunc   func(opts docker.PushImageOptions, auth docker.AuthConfiguration) error
	buildImageFunc  func(opts docker.BuildImageOptions) error
	removeImageFunc func(name string) error

	buildImageCalled  bool
	pushImageCalled   bool
	removeImageCalled bool
	errPushImage      error
}

func (d *FakeDocker) BuildImage(opts docker.BuildImageOptions) error {
	if d.buildImageFunc != nil {
		return d.buildImageFunc(opts)
	}
	return nil
}
func (d *FakeDocker) PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
	d.pushImageCalled = true
	if d.pushImageFunc != nil {
		return d.pushImageFunc(opts, auth)
	}
	return d.errPushImage
}
func (d *FakeDocker) RemoveImage(name string) error {
	if d.removeImageFunc != nil {
		return d.removeImageFunc(name)
	}
	return nil
}
func (d *FakeDocker) CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error) {
	return &docker.Container{}, nil
}
func (d *FakeDocker) DownloadFromContainer(id string, opts docker.DownloadFromContainerOptions) error {
	return nil
}
func (d *FakeDocker) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
	return nil
}
func (d *FakeDocker) RemoveContainer(opts docker.RemoveContainerOptions) error {
	return nil
}
func (d *FakeDocker) InspectImage(name string) (*docker.Image, error) {
	return &docker.Image{}, nil
}
func TestDockerPush(t *testing.T) {
	verifyFunc := func(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
		if opts.Name != "test/image" {
			t.Errorf("Unexpected image name: %s", opts.Name)
		}
		return nil
	}
	fd := &FakeDocker{pushImageFunc: verifyFunc}
	pushImage(fd, "test/image", docker.AuthConfiguration{})
}
