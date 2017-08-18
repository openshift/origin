package exec

import (
	"bytes"
	"fmt"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

type execError struct {
	error
	stdOut    string
	stdErr    string
	container string
	cmd       []string
	rc        int
}

func newExecError(cause error, rc int, stdOut, errOut string, container string, cmd []string) error {
	return &execError{
		error:     errors.NewError("Docker exec error").WithCause(cause),
		stdOut:    stdOut,
		stdErr:    errOut,
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
	if len(e.stdOut) > 0 {
		errors.PrintLog(out, "Output", e.stdOut)
	}
	if len(e.stdErr) > 0 {
		errors.PrintLog(out, "Error Output", e.stdErr)
	}
	return out.String()
}

// IsExecError returns true if the given error is an execError
func IsExecError(err error) bool {
	_, isExec := err.(*execError)
	return isExec
}
