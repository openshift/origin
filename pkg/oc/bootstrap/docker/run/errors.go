package run

import (
	"bytes"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

type runError struct {
	error
	stdOut string
	stdErr string
	config *container.Config
}

func newRunError(rc int, cause error, stdOut, stdErr string, config *container.Config) error {
	return &runError{
		error:  errors.NewError("Docker run error rc=%d", rc).WithCause(cause),
		stdOut: stdOut,
		stdErr: stdErr,
		config: config,
	}
}

func (e *runError) Details() string {
	out := &bytes.Buffer{}
	fmt.Fprintf(out, "Image: %s\n", e.config.Image)
	fmt.Fprintf(out, "Entrypoint: %v\n", e.config.Entrypoint)
	fmt.Fprintf(out, "Command: %v\n", e.config.Cmd)
	// TODO maybe we will re-introduce this, but we're starting to record the container logs in a tempdir, so I doubt it
	//if len(e.stdOut) > 0 {
	//	errors.PrintLog(out, "Output", e.stdOut)
	//}
	//if len(e.stdErr) > 0 {
	//	errors.PrintLog(out, "Error Output", e.stdErr)
	//}
	return out.String()
}
