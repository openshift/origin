package run

import (
	"bytes"
	"fmt"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
)

type runError struct {
	error
	out, err []byte
	config   *docker.Config
}

func newRunError(rc int, cause error, stdOut, errOut []byte, config *docker.Config) error {
	return &runError{
		error:  errors.NewError("Docker run error rc=%d", rc).WithCause(cause),
		out:    stdOut,
		err:    errOut,
		config: config,
	}
}

func (e *runError) Details() string {
	out := &bytes.Buffer{}
	fmt.Fprintf(out, "Image: %s\n", e.config.Image)
	fmt.Fprintf(out, "Entrypoint: %v\n", e.config.Entrypoint)
	fmt.Fprintf(out, "Command: %v\n", e.config.Cmd)
	if len(e.out) > 0 {
		errors.PrintLog(out, "Output", e.out)
	}
	if len(e.err) > 0 {
		errors.PrintLog(out, "Error Output", e.err)
	}
	return out.String()
}
