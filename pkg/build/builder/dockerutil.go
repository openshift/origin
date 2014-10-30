package builder

import (
	"os"

	"github.com/fsouza/go-dockerclient"
	"github.com/openshift/source-to-image/pkg/sti/tar"
)

// DockerClient is an interface to the Docker client that contains
// the methods used by the common builder
type DockerClient interface {
	BuildImage(opts docker.BuildImageOptions) error
	PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) error
}

// pushImage pushes a docker image to the registry specified in its tag
func pushImage(client DockerClient, name string, authConfig docker.AuthConfiguration) error {
	repository, tag := docker.ParseRepositoryTag(name)
	opts := docker.PushImageOptions{
		Name:         repository,
		Tag:          tag,
		OutputStream: os.Stdout,
	}
	return client.PushImage(opts, authConfig)
}

// buildImage invokes a docker build on a particular directory
func buildImage(client DockerClient, dir string, tag string, tar tar.Tar) error {
	tarFile, err := tar.CreateTarFile("", dir)
	if err != nil {
		return err
	}
	tarStream, err := os.Open(tarFile)
	if err != nil {
		return err
	}
	defer tarStream.Close()
	opts := docker.BuildImageOptions{
		Name:           tag,
		RmTmpContainer: true,
		OutputStream:   os.Stdout,
		InputStream:    tarStream,
	}
	return client.BuildImage(opts)
}
