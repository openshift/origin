/*
Code that supports the running of the image produced by s2i.  Triggered by the --run=true option specified on the command line.
*/
package run

import (
	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/errors"
	"io"
)

// the struct capturing the list of items needed to support the --run=true options;
// currently, only running docker containers and only need the docker client
type DockerRunner struct {
	ContainerClient docker.Docker
}

// Create the DockerRunning struct for executing the methods assoicated with running
// the produced image in a docker container for verification purposes.
func New(config *api.Config) (*DockerRunner, error) {
	cc, ccerr := docker.New(config.DockerConfig, config.PullAuthentication)
	if ccerr != nil {
		glog.Errorf("Create of Docker Container client failed with %v \n", ccerr)
		return nil, ccerr
	}
	dr := &DockerRunner{
		ContainerClient: cc,
	}
	return dr, nil
}

// Actually invoke the docker API to run the resulting s2i image in a container,
// where the redirecting of the container's stdout and stderr will go to glog.
func (b *DockerRunner) Run(config *api.Config) error {

	glog.V(4).Infof("Attempting to run image %s \n", config.Tag)

	errOutput := ""
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	defer errReader.Close()
	defer errWriter.Close()
	defer outReader.Close()
	defer outWriter.Close()

	opts := docker.RunContainerOptions{
		Image:        config.Tag,
		Stdout:       outWriter,
		Stderr:       errWriter,
		TargetImage:  true,
		CGroupLimits: config.CGroupLimits,
	}

	//NOTE, we've seen some Golang level deadlock issues with the streaming of cmd output to
	// glog, but part of the deadlock seems to have occurred when stdout was "silent"
	// and produced no data, such as when we would do a git clone with the --quiet option.
	// We have not seen the hang when the Cmd produces output to stdout.

	go docker.StreamContainerIO(errReader, nil, glog.Error)
	go docker.StreamContainerIO(outReader, nil, glog.Info)
	rerr := b.ContainerClient.RunContainer(opts)
	if e, ok := rerr.(errors.ContainerError); ok {
		return errors.NewContainerError(config.Tag, e.ErrorCode, errOutput)
	}

	return nil
}
