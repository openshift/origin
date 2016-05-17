package exec

import (
	"bytes"
	"fmt"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
)

type execError struct {
	error
	out, err  []byte
	container string
	cmd       []string
	rc        int
}

func newExecError(cause error, rc int, stdOut, errOut []byte, container string, cmd []string) error {
	return &execError{
		error:     errors.NewError("Docker exec error").WithCause(cause),
		out:       stdOut,
		err:       errOut,
		container: container,
		cmd:       cmd,
		rc:        rc,
	}
}

func (e *execError) Details() string {
	out := &bytes.Buffer{}
	fmt.Fprintf(out, "Container: %s\n", e.container)
	fmt.Fprintf(out, "Command: %v\n", e.cmd)
	fmt.Fprintf(out, "Result Code: %d\n", e.rc)
	if len(e.out) > 0 {
		errors.PrintLog(out, "Output", e.out)
	}
	if len(e.err) > 0 {
		errors.PrintLog(out, "Error Output", e.err)
	}
	return out.String()
}

// IsExecError returns true if the given error is an execError
func IsExecError(err error) bool {
	_, isExec := err.(*execError)
	return isExec
}
