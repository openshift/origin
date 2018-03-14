package dockermachine

import (
	starterrors "github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

// ErrDockerMachineExec is an error that occurred while executing the docker-machine command
func ErrDockerMachineExec(cmd string, cause error) error {
	return starterrors.NewError("failed to execute docker-machine %s", cmd).WithCause(cause)
}
