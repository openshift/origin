package openshift

import (
	"fmt"
)

// ErrOpenShiftFailedToStart is thrown when the OpenShift server failed to start
func ErrOpenShiftFailedToStart(container string) error {
	return fmt.Errorf("Could not start OpenShift container %q", container)
}

// ErrTimedOutWaitingForStart is thrown when the OpenShift server can't be pinged after reasonable
// amount of time.
func ErrTimedOutWaitingForStart(container string) error {
	return fmt.Errorf("Could not start OpenShift container %q", container)
}
