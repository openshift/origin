package dockermachine

import (
	"errors"

	starterrors "github.com/openshift/origin/pkg/bootstrap/docker/errors"
)

var (
	// ErrDockerMachineExists is returned if a Docker machine you are trying to create already exists
	ErrDockerMachineExists = errors.New("Docker machine exists")

	// ErrDockerMachineNotAvailable is returned if the docker-machine command is not available in the PATH
	ErrDockerMachineNotAvailable = errors.New("docker-machine not available")
)

// ErrDockerMachineExec is an error that occurred while executing the docker-machine command
func ErrDockerMachineExec(cmd string, cause error) error {
	return starterrors.NewError("failed to execute docker-machine %s", cmd).WithCause(cause)
}
