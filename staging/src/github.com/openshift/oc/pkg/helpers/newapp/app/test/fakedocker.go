package test

import (
	docker "github.com/fsouza/go-dockerclient"
)

type FakeDockerClient struct {
	// list result
	Images []docker.APIImages
	// inspect result
	Image      *docker.Image
	ListErr    error
	InspectErr error
}

func (f FakeDockerClient) ListImages(opts docker.ListImagesOptions) ([]docker.APIImages, error) {
	return f.Images, f.ListErr
}
func (f FakeDockerClient) InspectImage(name string) (*docker.Image, error) {
	return f.Image, f.InspectErr
}
