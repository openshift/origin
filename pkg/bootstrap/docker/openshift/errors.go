package openshift

import (
	"fmt"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
)

// ErrOpenShiftFailedToStart is thrown when the OpenShift server failed to start
func ErrOpenShiftFailedToStart(container string) errors.Error {
	return errors.NewError("could not start OpenShift container %q", container)
}

// ErrTimedOutWaitingForStart is thrown when the OpenShift server can't be pinged after reasonable
// amount of time.
func ErrTimedOutWaitingForStart(container string) errors.Error {
	return errors.NewError("timed out waiting for OpenShift container %q", container)
}

func ErrSocatNotFound() errors.Error {
	return errors.NewError("socat not found locally").
		WithDetails("socat is required to enable port forwarding\n").
		WithSolution("Install socat using your package manager first\n")
}

type errPortsNotAvailable struct {
	ports []int
}

func (e *errPortsNotAvailable) Error() string {
	return fmt.Sprintf("ports in use: %v", e.ports)
}

func ErrPortsNotAvailable(ports []int) error {
	return &errPortsNotAvailable{
		ports: ports,
	}
}

func IsPortsNotAvailableErr(err error) bool {
	_, ok := err.(*errPortsNotAvailable)
	return ok
}

func UnavailablePorts(err error) []int {
	e, ok := err.(*errPortsNotAvailable)
	if !ok {
		return []int{}
	}
	return e.ports
}
