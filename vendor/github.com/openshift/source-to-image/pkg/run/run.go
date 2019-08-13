// Package run supports running images produced by S2I. It is used by the
// --run=true command line option.
package run

import (
	"io"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/docker"
	s2ierr "github.com/openshift/source-to-image/pkg/errors"
	utillog "github.com/openshift/source-to-image/pkg/util/log"
)

var (
	log = utillog.StderrLog
)

// A DockerRunner allows running a Docker image as a new container, streaming
// stdout and stderr with glog.
type DockerRunner struct {
	ContainerClient docker.Docker
}

// New creates a DockerRunner for executing the methods associated with running
// the produced image in a docker container for verification purposes.
func New(client docker.Client, config *api.Config) *DockerRunner {
	d := docker.New(client, config.PullAuthentication)
	return &DockerRunner{d}
}

// Run invokes the Docker API to run the image defined in config as a new
// container. The container's stdout and stderr will be logged with glog.
func (b *DockerRunner) Run(config *api.Config) error {
	log.V(4).Infof("Attempting to run image %s \n", config.Tag)

	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()

	opts := docker.RunContainerOptions{
		Image:        config.Tag,
		Stdout:       outWriter,
		Stderr:       errWriter,
		TargetImage:  true,
		CGroupLimits: config.CGroupLimits,
		CapDrop:      config.DropCapabilities,
	}

	docker.StreamContainerIO(errReader, nil, func(s string) { log.Error(s) })
	docker.StreamContainerIO(outReader, nil, func(s string) { log.Info(s) })

	err := b.ContainerClient.RunContainer(opts)
	// If we get a ContainerError, the original message reports the
	// container name. The container is temporary and its name is
	// meaningless, therefore we make the error message more helpful by
	// replacing the container name with the image tag.
	if e, ok := err.(s2ierr.ContainerError); ok {
		return s2ierr.NewContainerError(config.Tag, e.ErrorCode, e.Output)
	}
	return err
}
